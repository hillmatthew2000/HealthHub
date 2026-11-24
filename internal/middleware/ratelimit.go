package middleware

import (
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hillmatthew2000/HealthHub/internal/problem"
)

// Limiter is an in-memory token bucket rate limiter with one bucket per
// key. Each bucket holds up to perMinute tokens and refills continuously
// at perMinute per minute, so a client may burst up to its limit and then
// sustain the limit. State is per process: with several replicas each one
// enforces the limit separately.
type Limiter struct {
	mu        sync.Mutex
	perSecond float64
	burst     float64
	buckets   map[string]*bucket
	now       func() time.Time
	lastSweep time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

// sweepInterval is how often idle buckets are dropped.
const sweepInterval = time.Minute

// NewLimiter returns a limiter allowing perMinute requests per minute per
// key. perMinute must be positive.
func NewLimiter(perMinute int) *Limiter {
	return newLimiter(perMinute, time.Now)
}

func newLimiter(perMinute int, now func() time.Time) *Limiter {
	if perMinute < 1 {
		panic("middleware: rate limit must be positive")
	}
	return &Limiter{
		perSecond: float64(perMinute) / 60,
		burst:     float64(perMinute),
		buckets:   map[string]*bucket{},
		now:       now,
		lastSweep: now(),
	}
}

// Allow takes a token from key's bucket. If none is left it reports how
// long until one will be.
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.Sub(l.lastSweep) >= sweepInterval {
		l.sweep(now)
	}

	b, found := l.buckets[key]
	if !found {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	} else {
		b.tokens = math.Min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.perSecond)
		b.last = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := time.Duration(math.Ceil((1 - b.tokens) / l.perSecond * float64(time.Second)))
	return false, wait
}

// sweep drops buckets that have refilled completely; recreating them later
// gives the same result, so this only bounds memory.
func (l *Limiter) sweep(now time.Time) {
	full := time.Duration(l.burst / l.perSecond * float64(time.Second))
	for k, b := range l.buckets {
		if now.Sub(b.last) >= full {
			delete(l.buckets, k)
		}
	}
	l.lastSweep = now
}

// size returns the number of tracked keys.
func (l *Limiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

// ClientKey identifies the client of a request for rate limiting: its IP
// address as resolved through the trusted proxies, with IPv6 addresses
// reduced to their /64 network, since a single host usually controls a
// whole /64.
func ClientKey(c *gin.Context) string {
	ip := c.ClientIP()
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	addr = addr.Unmap()
	if addr.Is6() {
		if p, err := addr.Prefix(64); err == nil {
			return p.String()
		}
	}
	return addr.String()
}

// RateLimit rejects requests over l's limit with 429 and a Retry-After
// header. scope names the limit in the problem detail.
func RateLimit(l *Limiter, scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ok, wait := l.Allow(ClientKey(c))
		if ok {
			c.Next()
			return
		}
		seconds := int(math.Ceil(wait.Seconds()))
		c.Header("Retry-After", strconv.Itoa(max(seconds, 1)))
		problem.Write(c, problem.New(http.StatusTooManyRequests, problem.CodeRateLimited,
			"Too many "+scope+" requests. Retry after the time given in the Retry-After header."))
	}
}
