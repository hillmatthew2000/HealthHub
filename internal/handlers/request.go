// Package handlers implements the HTTP API. Handlers depend on small
// repository interfaces so they can be tested with in-memory fakes.
package handlers

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/hillmatthew2000/HealthHub/internal/models"
	"github.com/hillmatthew2000/HealthHub/internal/problem"
	"github.com/hillmatthew2000/HealthHub/internal/store"
)

// Paging holds the page size limits for list endpoints.
type Paging struct {
	DefaultLimit int
	MaxLimit     int
}

// DefaultPaging is used when a handler is built with a zero Paging.
var DefaultPaging = Paging{DefaultLimit: 20, MaxLimit: 100}

func (p Paging) orDefault() Paging {
	if p.DefaultLimit < 1 || p.MaxLimit < p.DefaultLimit {
		return DefaultPaging
	}
	return p
}

// PageResponse wraps one page of a list result.
type PageResponse[T any] struct {
	Data       []T `json:"data"`
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

func newPage[T any](data []T, page, limit, total int) PageResponse[T] {
	if data == nil {
		data = []T{}
	}
	return PageResponse[T]{
		Data:       data,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: (total + limit - 1) / limit,
	}
}

// queryErrors collects problems with query parameters so a client sees all
// of them at once.
type queryErrors []problem.FieldError

func (q *queryErrors) add(field, msg string) {
	*q = append(*q, problem.FieldError{Field: field, Message: msg})
}

// write sends a 400 listing the errors and reports whether there were any.
func (q queryErrors) write(c *gin.Context) bool {
	if len(q) == 0 {
		return false
	}
	problem.Write(c, problem.BadRequest("One or more query parameters are invalid.").WithErrors(q...))
	return true
}

// page parses the page and limit query parameters.
func (p Paging) page(c *gin.Context, errs *queryErrors) (store.Page, int, int) {
	p = p.orDefault()
	page, limit := 1, p.DefaultLimit
	if v, ok := c.GetQuery("page"); ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			errs.add("page", "must be a positive integer")
		} else {
			page = n
		}
	}
	if v, ok := c.GetQuery("limit"); ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > p.MaxLimit {
			errs.add("limit", "must be an integer between 1 and "+strconv.Itoa(p.MaxLimit))
		} else {
			limit = n
		}
	}
	// Guard the offset against overflow from absurd page numbers.
	const maxOffset = 1_000_000
	if (page - 1) > maxOffset/limit {
		errs.add("page", "is too large")
		page = 1
	}
	return store.Page{Limit: limit, Offset: (page - 1) * limit}, page, limit
}

// pathID parses a UUID path parameter, writing a 400 if it is malformed.
func pathID(c *gin.Context, name, resource string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		problem.Write(c, problem.BadRequest(resource+" IDs are UUIDs; "+strconv.Quote(c.Param(name))+" is not one."))
		return uuid.Nil, false
	}
	return id, true
}

// queryUUID parses an optional UUID query parameter.
func queryUUID(c *gin.Context, name string, errs *queryErrors) *uuid.UUID {
	v, ok := c.GetQuery(name)
	if !ok {
		return nil
	}
	id, err := uuid.Parse(v)
	if err != nil {
		errs.add(name, "must be a UUID")
		return nil
	}
	return &id
}

// queryBool parses an optional boolean query parameter.
func queryBool(c *gin.Context, name string, errs *queryErrors) *bool {
	v, ok := c.GetQuery(name)
	if !ok {
		return nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		errs.add(name, "must be true or false")
		return nil
	}
	return &b
}

// queryEnum returns an optional query parameter that must be one of allowed.
func queryEnum(c *gin.Context, name string, errs *queryErrors, allowed ...string) string {
	v, ok := c.GetQuery(name)
	if !ok {
		return ""
	}
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	errs.add(name, "must be one of: "+strings.Join(allowed, " "))
	return ""
}

// queryTime parses an optional RFC 3339 timestamp or YYYY-MM-DD date. A date
// means the start of that day in UTC, or with endOfDay the last instant of
// it, so to=2026-01-31 includes the whole of 31 January.
func queryTime(c *gin.Context, name string, endOfDay bool, errs *queryErrors) *time.Time {
	v, ok := c.GetQuery(name)
	if !ok {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return &t
	}
	d, err := models.ParseDate(v)
	if err != nil {
		errs.add(name, "must be an RFC 3339 timestamp or a YYYY-MM-DD date")
		return nil
	}
	t := d.Time
	if endOfDay {
		t = t.Add(24*time.Hour - time.Nanosecond)
	}
	return &t
}

// bindJSON decodes the request body into dst and validates it. It writes
// 400 for malformed JSON, 413 for oversized bodies and 422 for validation
// failures, and reports whether the handler should continue.
func bindJSON(c *gin.Context, v *validator.Validate, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		var tooLarge *http.MaxBytesError
		switch {
		case errors.As(err, &tooLarge):
			problem.Write(c, problem.New(http.StatusRequestEntityTooLarge, problem.CodePayloadTooLarge,
				"The request body is too large."))
		case errors.Is(err, io.EOF):
			problem.Write(c, problem.BadRequest("A JSON request body is required."))
		default:
			problem.Write(c, problem.BadRequest("The request body must be valid JSON matching the documented schema."))
		}
		return false
	}
	if err := v.Struct(dst); err != nil {
		problem.Write(c, problem.Validation(fieldErrors(err)...))
		return false
	}
	return true
}

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	// Report JSON field names rather than Go field names.
	v.RegisterTagNameFunc(jsonFieldName)
	return v
}
