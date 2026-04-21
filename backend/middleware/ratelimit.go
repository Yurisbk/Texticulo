package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

type ipWindow struct {
	count int
	until time.Time
}

var shortenLimiter = struct {
	mu sync.Mutex
	m  map[string]*ipWindow
}{
	m: make(map[string]*ipWindow),
}

// ShortenRateLimit allows up to limit POST requests per window per IP.
func ShortenRateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}
			ip := clientIP(r)
			now := time.Now()
			shortenLimiter.mu.Lock()
			e, ok := shortenLimiter.m[ip]
			if !ok || now.After(e.until) {
				shortenLimiter.m[ip] = &ipWindow{count: 1, until: now.Add(window)}
				shortenLimiter.mu.Unlock()
				next.ServeHTTP(w, r)
				return
			}
			e.count++
			if e.count > limit {
				shortenLimiter.mu.Unlock()
				http.Error(w, `{"error":"rate_limit"}`, http.StatusTooManyRequests)
				return
			}
			shortenLimiter.mu.Unlock()
			next.ServeHTTP(w, r)
		})
	}
}

// NewIPRateLimiter creates an isolated per-IP rate limiter that applies to all
// HTTP methods. Use for auth endpoints (register, login) to prevent brute-force.
func NewIPRateLimiter(limit int, window time.Duration) func(http.Handler) http.Handler {
	type entry struct {
		count int
		until time.Time
	}
	var mu sync.Mutex
	m := make(map[string]*entry)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			now := time.Now()
			mu.Lock()
			e, ok := m[ip]
			if !ok || now.After(e.until) {
				m[ip] = &entry{count: 1, until: now.Add(window)}
				mu.Unlock()
				next.ServeHTTP(w, r)
				return
			}
			e.count++
			if e.count > limit {
				mu.Unlock()
				http.Error(w, `{"error":"rate_limit"}`, http.StatusTooManyRequests)
				return
			}
			mu.Unlock()
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		return strings.TrimSpace(strings.Split(xf, ",")[0])
	}
	return r.RemoteAddr
}
