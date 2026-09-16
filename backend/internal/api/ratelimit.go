// Package api — rate limit helpers for auth endpoints.
package api

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// ipRateLimiter is a per-IP sliding window rate limiter.
type ipRateLimiter struct {
	mu       sync.Mutex
	windows  map[string]*window
	limit    int
	windowSz time.Duration
}

type window struct {
	hits    []time.Time
}

func newIPRateLimiter(limit int, windowSz time.Duration) *ipRateLimiter {
	return &ipRateLimiter{
		windows:  make(map[string]*window),
		limit:    limit,
		windowSz: windowSz,
	}
}

// Allow returns true if the IP is within budget.
func (rl *ipRateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	w, ok := rl.windows[ip]
	if !ok {
		w = &window{}
		rl.windows[ip] = w
	}
	// Trim entries outside the window.
	cutoff := now.Add(-rl.windowSz)
	n := 0
	for _, t := range w.hits {
		if t.After(cutoff) {
			w.hits[n] = t
			n++
		}
	}
	w.hits = w.hits[:n]
	if len(w.hits) >= rl.limit {
		return false
	}
	w.hits = append(w.hits, now)
	return true
}

// extractIP extracts the real client IP from X-Forwarded-For (Caddy sets this)
// or falls back to RemoteAddr.
func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First IP in the chain is the original client.
		if ip := net.ParseIP(xff[:len(xff)]); ip != nil {
			return xff
		}
		// Handle comma-separated: take first.
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// rateLimitAuth is middleware that limits auth-related endpoints per IP:
// max requests per sliding window.
func (s *Server) rateLimitAuth(rl *ipRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		if !rl.Allow(ip) {
			writeErr(w, http.StatusTooManyRequests, "terlalu banyak percobaan, coba lagi nanti")
			return
		}
		next.ServeHTTP(w, r)
	})
}
