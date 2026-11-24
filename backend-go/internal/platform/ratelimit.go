package platform

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/biletflow/backend-go/internal/auth"
)

// ErrRateLimited is returned when a caller exceeds an endpoint's limit (NFR-15).
var ErrRateLimited = Err(http.StatusTooManyRequests, "RATE_LIMITED", "Too many attempts. Please wait a moment and try again.")

// RateLimiter is a fixed-window counter per key. It is per instance; a multi-instance
// deployment would move it to a shared store, which is documented as future work.
type RateLimiter struct {
	limit  int
	window time.Duration
	mu     sync.Mutex
	hits   map[string]*windowCount
}

type windowCount struct {
	start time.Time
	n     int
}

// NewRateLimiter allows limit requests per window for each key. A limit of 0 disables limiting.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window, hits: map[string]*windowCount{}}
}

func (l *RateLimiter) Allow(key string) bool {
	if l == nil || l.limit <= 0 {
		return true
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.hits) > 50_000 {
		for k, w := range l.hits {
			if now.Sub(w.start) > l.window {
				delete(l.hits, k)
			}
		}
	}
	w, ok := l.hits[key]
	if !ok || now.Sub(w.start) > l.window {
		l.hits[key] = &windowCount{start: now, n: 1}
		return true
	}
	w.n++
	return w.n <= l.limit
}

// ClientKey identifies a caller: the authenticated user when known, else the client IP.
// X-Forwarded-For is only trusted when TRUST_PROXY is configured (set by main).
func ClientKey(r *http.Request) string {
	if p, ok := auth.Current(r.Context()); ok && p.ID != "" {
		return "u:" + p.ID
	}
	return "ip:" + ClientIP(r)
}

// TrustProxy makes ClientIP honour the first X-Forwarded-For address (reverse proxy in front).
var TrustProxy bool

func ClientIP(r *http.Request) string {
	if TrustProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			return strings.TrimSpace(strings.Split(fwd, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
