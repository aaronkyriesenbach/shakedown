package middleware

import (
	"net/http"
	"sync"
	"time"
)

type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientState
	rate    int
	window  time.Duration
}

type clientState struct {
	count    int
	windowAt time.Time
}

func newRateLimiter(rate int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		clients: make(map[string]*clientState),
		rate:    rate,
		window:  window,
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, cs := range rl.clients {
			if now.Sub(cs.windowAt) > rl.window {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cs, ok := rl.clients[ip]
	if !ok || now.Sub(cs.windowAt) > rl.window {
		rl.clients[ip] = &clientState{count: 1, windowAt: now}
		return true
	}
	cs.count++
	return cs.count <= rl.rate
}

func RateLimit(rate int, window time.Duration) func(http.Handler) http.Handler {
	rl := newRateLimiter(rate, window)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				ip = xff
			}
			if !rl.allow(ip) {
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
