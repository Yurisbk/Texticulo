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

// ShortenRateLimit allows up to limit requests per window per IP for POST /api/shorten.
func ShortenRateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}
			ip := r.RemoteAddr
			if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
				ip = strings.TrimSpace(strings.Split(xf, ",")[0])
			}
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
