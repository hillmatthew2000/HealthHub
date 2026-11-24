package handlers

import (
	"errors"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/hillmatthew2000/HealthHub/internal/problem"
	"github.com/hillmatthew2000/HealthHub/internal/store"
	"github.com/hillmatthew2000/HealthHub/pkg/logger"
)

// internalError logs err with the request ID and returns a generic problem.
// Internal error text (SQL, driver messages) is never sent to clients.
func internalError(c *gin.Context, op string, err error) {
	logger.Error("request failed",
		zap.String("op", op),
		zap.String("request_id", c.GetString(problem.RequestIDKey)),
		zap.Error(err),
	)
	problem.Write(c, problem.Internal())
}

// storeError maps repository errors to problems. resource names the entity
// for the not found message, for example "Patient".
func storeError(c *gin.Context, op, resource string, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		problem.Write(c, problem.NotFound(resource+" not found."))
	case errors.Is(err, store.ErrConflict):
		problem.Write(c, problem.Conflict(resource+" conflicts with an existing record."))
	default:
		internalError(c, op, err)
	}
}

func unauthenticated(c *gin.Context) {
	problem.Write(c, problem.New(http.StatusUnauthorized, problem.CodeUnauthorized, "Authentication is required."))
}

// fieldErrors converts validator errors into field level problem entries
// without echoing the rejected values, which may contain PHI.
func fieldErrors(err error) []problem.FieldError {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return []problem.FieldError{{Field: "body", Message: "is invalid"}}
	}
	out := make([]problem.FieldError, 0, len(verrs))
	for _, fe := range verrs {
		out = append(out, problem.FieldError{Field: fieldPath(fe), Message: ruleMessage(fe)})
	}
	return out
}

// fieldPath turns "Patient.Name[0].Family" into "name[0].family".
func fieldPath(fe validator.FieldError) string {
	ns := fe.Namespace()
	if i := strings.Index(ns, "."); i >= 0 {
		ns = ns[i+1:]
	}
	parts := strings.Split(ns, ".")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToLower(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, ".")
}

func ruleMessage(fe validator.FieldError) string {
	unit := " characters"
	switch fe.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		unit = " items"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		unit = ""
	}
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		if unit == "" {
			return "must be at least " + fe.Param()
		}
		return "must have at least " + fe.Param() + unit
	case "max":
		if unit == "" {
			return "must be at most " + fe.Param()
		}
		return "must have at most " + fe.Param() + unit
	case "oneof":
		return "must be one of: " + fe.Param()
	default:
		return "failed the " + fe.Tag() + " rule"
	}
}

// jsonFieldName makes validation errors name fields as they appear in JSON.
func jsonFieldName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	switch name {
	case "-":
		return ""
	case "":
		return f.Name
	}
	return name
}
