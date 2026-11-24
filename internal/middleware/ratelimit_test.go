package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func TestLimiterBurstAndRefill(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := newLimiter(6, clock.now) // one token every 10s, bursts of 6

	for i := range 6 {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatalf("request %d within the burst was refused", i+1)
		}
	}
	ok, wait := l.Allow("a")
	if ok || wait != 10*time.Second {
		t.Fatalf("over the limit: ok=%v wait=%s, want refusal with a 10s wait", ok, wait)
	}
	if ok, _ := l.Allow("b"); !ok {
		t.Error("another key shares the bucket")
	}

	clock.advance(4 * time.Second)
	if ok, wait := l.Allow("a"); ok || wait != 6*time.Second {
		t.Errorf("after 4s: ok=%v wait=%s", ok, wait)
	}
	clock.advance(6 * time.Second)
	if ok, _ := l.Allow("a"); !ok {
		t.Error("no token after the refill interval")
	}
	if ok, _ := l.Allow("a"); ok {
		t.Error("refill granted more than one token")
	}

	// A long pause refills to the burst size, not beyond it.
	clock.advance(time.Hour)
	for range 6 {
		l.Allow("a")
	}
	if ok, _ := l.Allow("a"); ok {
		t.Error("bucket held more than the burst size")
	}
}

func TestLimiterDropsIdleClients(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := newLimiter(60, clock.now)
	l.Allow("old")
	clock.advance(2 * time.Minute)
	l.Allow("new")
	if n := l.size(); n != 1 {
		t.Errorf("tracking %d clients, want only the active one", n)
	}
}

func TestClientKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := map[string]string{
		"192.0.2.7:5000":              "192.0.2.7",
		"[2001:db8:1:2:aaaa::1]:5000": "2001:db8:1:2::/64",
		"[2001:db8:1:2:bbbb::9]:5000": "2001:db8:1:2::/64",
		"[::ffff:198.51.100.4]:5000":  "198.51.100.4",
	}
	for remote, want := range tests {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Request.RemoteAddr = remote
		if got := ClientKey(c); got != want {
			t.Errorf("ClientKey(%s) = %q, want %q", remote, got, want)
		}
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimit(NewLimiter(2), "test"))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	codes := make([]int, 0, 3)
	var last *httptest.ResponseRecorder
	for range 3 {
		last = httptest.NewRecorder()
		r.ServeHTTP(last, httptest.NewRequest(http.MethodGet, "/x", nil))
		codes = append(codes, last.Code)
	}
	if codes[0] != http.StatusNoContent || codes[1] != http.StatusNoContent || codes[2] != http.StatusTooManyRequests {
		t.Fatalf("status codes = %v", codes)
	}
	if ra := last.Header().Get("Retry-After"); ra != "30" {
		t.Errorf("Retry-After = %q, want 30", ra)
	}
	if ct := last.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q", ct)
	}
}
