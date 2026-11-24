package problem

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/patients/:id", func(c *gin.Context) {
		c.Set(RequestIDKey, "req-123")
		Write(c, Validation(FieldError{Field: "gender", Message: "is required"}))
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/patients/42", nil))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != ContentType {
		t.Fatalf("content type = %q", ct)
	}
	var p Problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Type != "about:blank" || p.Title != "Unprocessable Entity" || p.Status != 422 ||
		p.Code != CodeValidationFailed || p.Instance != "/patients/42" || p.RequestID != "req-123" ||
		len(p.Errors) != 1 || p.Errors[0].Field != "gender" {
		t.Fatalf("problem = %+v", p)
	}
}

func TestInternalHidesDetails(t *testing.T) {
	p := Internal()
	if p.Status != http.StatusInternalServerError || p.Code != CodeInternal || p.Detail == "" {
		t.Fatalf("problem = %+v", p)
	}
}
