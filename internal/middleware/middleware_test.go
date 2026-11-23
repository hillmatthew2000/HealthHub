package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func router(origins []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders(true), CORS(origins))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func do(r http.Handler, method, origin string, preflight bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/x", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if preflight {
		req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCORSAllowedOrigin(t *testing.T) {
	w := do(router([]string{"https://app.example"}), http.MethodGet, "https://app.example", false)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Fatalf("allow origin = %q", got)
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatal("credentials must not be allowed")
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	w := do(router([]string{"https://app.example"}), http.MethodGet, "https://evil.example", false)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected allow origin %q", got)
	}
}

func TestCORSDefaultDeniesAll(t *testing.T) {
	w := do(router(nil), http.MethodGet, "https://app.example", false)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected allow origin %q", got)
	}
}

func TestCORSWildcardDoesNotReflect(t *testing.T) {
	w := do(router([]string{"*"}), http.MethodGet, "https://any.example", false)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("allow origin = %q, want *", got)
	}
}

func TestCORSPreflight(t *testing.T) {
	w := do(router([]string{"https://app.example"}), http.MethodOptions, "https://app.example", true)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatal("missing allow methods on preflight")
	}

	w = do(router([]string{"https://app.example"}), http.MethodOptions, "https://evil.example", true)
	if w.Header().Get("Access-Control-Allow-Methods") != "" {
		t.Fatal("preflight from unknown origin must not be granted")
	}
}

func TestSecurityHeaders(t *testing.T) {
	w := do(router(nil), http.MethodGet, "", false)
	for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy", "Strict-Transport-Security", "Cache-Control"} {
		if w.Header().Get(h) == "" {
			t.Errorf("missing %s", h)
		}
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", w.Header().Get("Cache-Control"))
	}
}
