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
	limit   int
	window  time.Duration
}

type rlEntry struct {
	count     int
	expiresAt time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		entries: make(map[string]*rlEntry),
		limit:   limit,
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
	if e.count >= rl.limit {
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

// privateCIDRs lists all RFC-1918, RFC-4193 and loopback ranges.
// Connections arriving from these addresses are considered to originate
// from a trusted local reverse proxy (Caddy, Nginx, Pangolin, …), so
// proxy-injected headers (X-Real-IP, X-Forwarded-For, CF-Connecting-IP)
// are honoured.  Connections from public IPs are never allowed to supply
// their own "real IP" header — that would let an attacker bypass rate limits.
var privateCIDRs = func() []*net.IPNet {
	blocks := []string{
		"127.0.0.0/8",    // IPv4 loopback
		"::1/128",        // IPv6 loopback
		"10.0.0.0/8",     // RFC-1918
		"172.16.0.0/12",  // RFC-1918
		"192.168.0.0/16", // RFC-1918
		"fc00::/7",       // RFC-4193 unique local
	}
	nets := make([]*net.IPNet, 0, len(blocks))
	for _, b := range blocks {
		_, ipnet, _ := net.ParseCIDR(b)
		nets = append(nets, ipnet)
	}
	return nets
}()

// isTrustedProxy returns true when the direct TCP peer is a local or
// private-network address — meaning a reverse proxy that we trust to
// have set accurate forwarding headers.
func isTrustedProxy(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, cidr := range privateCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// clientIP returns the real originating IP address of the request.
//
// Security model:
//   - If the TCP connection comes directly from a public IP, that IP is used
//     unconditionally.  Proxy headers are ignored because a public client can
//     forge them trivially to bypass rate limiting.
//   - If the TCP connection comes from a private/loopback address (a local
//     reverse proxy such as Caddy, Nginx, Pangolin, or a Cloudflare tunnel),
//     the real client IP is read from proxy headers in this priority order:
//     1. CF-Connecting-IP  (Cloudflare)
//     2. X-Real-IP         (Nginx, Caddy)
//     3. X-Forwarded-For   (standard, leftmost IP)
//     4. RemoteAddr        (fallback)
func clientIP(r *http.Request) string {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}

	if !isTrustedProxy(r.RemoteAddr) {
		// Public connection — use the TCP peer address directly.
		return remoteHost
	}

	// Connection from a local proxy — trust its forwarding headers.
	if ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); ip != "" {
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	return remoteHost
}
