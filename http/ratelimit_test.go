package fbhttp

import (
	"net/http"
	"testing"
	"time"
)

func TestClientIP_DirectPublic(t *testing.T) {
	// A public client connecting directly must NOT be able to spoof its IP
	// via proxy headers.
	r, _ := http.NewRequest("POST", "/api/login", nil)
	r.RemoteAddr = "203.0.113.42:54321" // public IP, direct connection
	r.Header.Set("X-Real-IP", "1.2.3.4")
	r.Header.Set("X-Forwarded-For", "5.6.7.8")
	r.Header.Set("CF-Connecting-IP", "9.10.11.12")

	got := clientIP(r)
	if got != "203.0.113.42" {
		t.Errorf("expected RemoteAddr IP 203.0.113.42, got %q", got)
	}
}

func TestClientIP_BehindLocalProxy_XRealIP(t *testing.T) {
	// Connection from localhost (reverse proxy) — X-Real-IP should be trusted.
	r, _ := http.NewRequest("POST", "/api/login", nil)
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("X-Real-IP", "203.0.113.55")

	got := clientIP(r)
	if got != "203.0.113.55" {
		t.Errorf("expected X-Real-IP 203.0.113.55, got %q", got)
	}
}

func TestClientIP_BehindPrivateProxy_XForwardedFor(t *testing.T) {
	// Connection from a private network proxy — XFF should be trusted.
	r, _ := http.NewRequest("POST", "/api/login", nil)
	r.RemoteAddr = "10.0.0.1:9000"
	r.Header.Set("X-Forwarded-For", "198.51.100.7, 10.0.0.1")

	got := clientIP(r)
	if got != "198.51.100.7" {
		t.Errorf("expected first XFF IP 198.51.100.7, got %q", got)
	}
}

func TestClientIP_BehindCloudflare(t *testing.T) {
	// Cloudflare tunnel — CF-Connecting-IP takes priority.
	r, _ := http.NewRequest("POST", "/api/login", nil)
	r.RemoteAddr = "172.16.0.2:443"
	r.Header.Set("CF-Connecting-IP", "203.0.113.99")
	r.Header.Set("X-Real-IP", "10.0.0.5")

	got := clientIP(r)
	if got != "203.0.113.99" {
		t.Errorf("expected CF-Connecting-IP 203.0.113.99, got %q", got)
	}
}

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)
	for i := range 3 {
		if !rl.allow("ip") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)
	for range 3 {
		rl.allow("ip")
	}
	if rl.allow("ip") {
		t.Error("4th attempt should be blocked")
	}
}

func TestRateLimiter_ResetsAfterWindow(t *testing.T) {
	rl := newRateLimiter(2, 50*time.Millisecond)
	rl.allow("ip")
	rl.allow("ip")
	if rl.allow("ip") {
		t.Error("3rd attempt within window should be blocked")
	}
	time.Sleep(60 * time.Millisecond)
	if !rl.allow("ip") {
		t.Error("first attempt after window reset should be allowed")
	}
}

func TestRateLimiter_IndependentKeys(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)
	rl.allow("ip-a")
	if !rl.allow("ip-b") {
		t.Error("ip-b should have its own independent counter")
	}
}
