package fbhttp

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter is a simple sliding-window, in-memory rate limiter.
// It counts events per key within a fixed window and blocks when the
// threshold is exceeded.  A background goroutine cleans up expired entries
// every minute so memory use stays bounded.
type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rlEntry
	max     int
	window  time.Duration
}

type rlEntry struct {
	count     int
	expiresAt time.Time
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		entries: make(map[string]*rlEntry),
		max:     max,
		window:  window,
	}
	go rl.gc()
	return rl
}

// allow increments the counter for key and returns true if the request is
// within the allowed rate, false if it should be blocked.
func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	e, ok := rl.entries[key]
	if !ok || now.After(e.expiresAt) {
		rl.entries[key] = &rlEntry{count: 1, expiresAt: now.Add(rl.window)}
		return true
	}
	if e.count >= rl.max {
		return false
	}
	e.count++
	return true
}

// gc removes expired entries once per minute.
func (rl *rateLimiter) gc() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for k, e := range rl.entries {
			if now.After(e.expiresAt) {
				delete(rl.entries, k)
			}
		}
		rl.mu.Unlock()
	}
}

// Package-level rate limiters shared across all handlers.
var (
	// loginLimiter allows at most 10 login attempts per IP per 5 minutes.
	loginLimiter = newRateLimiter(10, 5*time.Minute)

	// shareLimiter allows at most 10 password attempts per (IP+shareHash) per 5 minutes.
	shareLimiter = newRateLimiter(10, 5*time.Minute)
)

// clientIP returns the originating IP address of the request.
// It honours the X-Real-IP and X-Forwarded-For headers that a trusted reverse
// proxy may set, falling back to RemoteAddr when neither is present or valid.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		if net.ParseIP(strings.TrimSpace(ip)) != nil {
			return strings.TrimSpace(ip)
		}
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
