package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/hillmatthew2000/HealthHub/internal/problem"
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

// fieldPath turns "RegisterRequest.FirstName" into "firstName".
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
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return "must be at least " + fe.Param() + " characters"
	case "max":
		return "must be at most " + fe.Param() + " characters"
	case "oneof":
		return "must be one of: " + fe.Param()
	default:
		return "failed the " + fe.Tag() + " rule"
	}
}
