// Package problem renders API errors as RFC 9457 problem details
// (application/problem+json) so clients get one predictable error shape.
package problem

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ContentType is the media type of problem detail responses.
const ContentType = "application/problem+json"

// RequestIDKey is the gin context key holding the request ID. It is set by
// the request ID middleware and echoed in every problem response.
const RequestIDKey = "request_id"

// Machine readable error codes. Clients should branch on these rather than
// on the human readable title or detail.
const (
	CodeInvalidRequest       = "invalid_request"
	CodeValidationFailed     = "validation_failed"
	CodeUnauthorized         = "unauthorized"
	CodeInvalidToken         = "invalid_token"
	CodeInvalidCredentials   = "invalid_credentials"
	CodeForbidden            = "forbidden"
	CodeNotFound             = "not_found"
	CodeMethodNotAllowed     = "method_not_allowed"
	CodeConflict             = "conflict"
	CodePreconditionFailed   = "precondition_failed"
	CodePayloadTooLarge      = "payload_too_large"
	CodeUnsupportedMedia     = "unsupported_media_type"
	CodeRateLimited          = "rate_limited"
	CodeRegistrationDisabled = "registration_disabled"
	CodeInternal             = "internal_error"
	CodeUnavailable          = "service_unavailable"
	CodeTimeout              = "timeout"
)

// FieldError describes why a single input field was rejected.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Problem is an RFC 9457 problem details object with two extension
// members: code (stable, machine readable) and errors (per field).
type Problem struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Detail    string       `json:"detail,omitempty"`
	Instance  string       `json:"instance,omitempty"`
	Code      string       `json:"code"`
	RequestID string       `json:"requestId,omitempty"`
	Errors    []FieldError `json:"errors,omitempty"`
}

// New returns a problem for the given HTTP status. The title is the
// standard status text, as RFC 9457 requires for type "about:blank".
func New(status int, code, detail string) *Problem {
	return &Problem{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Code:   code,
		Detail: detail,
	}
}

// WithErrors attaches field errors and returns p.
func (p *Problem) WithErrors(errs ...FieldError) *Problem {
	p.Errors = append(p.Errors, errs...)
	return p
}

// Error implements the error interface so problems can be returned from
// helper functions.
func (p *Problem) Error() string {
	if p.Detail != "" {
		return p.Code + ": " + p.Detail
	}
	return p.Code
}

// Write sends p as the response and aborts the handler chain.
func Write(c *gin.Context, p *Problem) {
	if p.Instance == "" && c.Request != nil {
		p.Instance = c.Request.URL.Path
	}
	if p.RequestID == "" {
		p.RequestID = c.GetString(RequestIDKey)
	}
	c.Header("Content-Type", ContentType)
	c.AbortWithStatusJSON(p.Status, p)
}

// Convenience constructors for common cases.

// BadRequest reports malformed input.
func BadRequest(detail string) *Problem {
	return New(http.StatusBadRequest, CodeInvalidRequest, detail)
}

// Validation reports input that is well formed but semantically invalid.
func Validation(errs ...FieldError) *Problem {
	return New(http.StatusUnprocessableEntity, CodeValidationFailed,
		"One or more fields are invalid.").WithErrors(errs...)
}

// NotFound reports a missing resource.
func NotFound(detail string) *Problem {
	return New(http.StatusNotFound, CodeNotFound, detail)
}

// Conflict reports a uniqueness or state conflict.
func Conflict(detail string) *Problem {
	return New(http.StatusConflict, CodeConflict, detail)
}

// Forbidden reports an authenticated caller lacking permission.
func Forbidden() *Problem {
	return New(http.StatusForbidden, CodeForbidden, "You do not have permission to perform this action.")
}

// Internal reports an unexpected server error without leaking details.
func Internal() *Problem {
	return New(http.StatusInternalServerError, CodeInternal,
		"An unexpected error occurred. Quote the request ID when reporting this problem.")
}
