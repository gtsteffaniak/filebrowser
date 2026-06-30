package http

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ipLimiter holds a rate limiter per IP address.
type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type rateLimitStore struct {
	mu       sync.Mutex
	limiters map[string]*ipLimiter
}

var loginRateLimiter = &rateLimitStore{
	limiters: make(map[string]*ipLimiter),
}

// shareRateLimiter limits public share access attempts to 30 per minute per IP.
// This prevents brute-forcing share passwords while allowing legitimate bulk access.
var shareRateLimiter = &rateLimitStore{
	limiters: make(map[string]*ipLimiter),
}

func init() {
	// Periodically clean up stale entries every 5 minutes
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			loginRateLimiter.cleanup()
			shareRateLimiter.cleanup()
		}
	}()
}

func (s *rateLimitStore) get(ip string) *rate.Limiter {
	return s.getWithRate(ip, rate.Every(time.Minute/5), 5)
}

func (s *rateLimitStore) getWithRate(ip string, r rate.Limit, burst int) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.limiters[ip]
	if !ok {
		entry = &ipLimiter{limiter: rate.NewLimiter(r, burst)}
		s.limiters[ip] = entry
	}
	entry.lastSeen = time.Now()
	return entry.limiter
}

func (s *rateLimitStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-10 * time.Minute)
	for ip, entry := range s.limiters {
		if entry.lastSeen.Before(cutoff) {
			delete(s.limiters, ip)
		}
	}
}

// isPrivateIP returns true for loopback and RFC-1918/RFC-4193 addresses used
// by Azure load balancers and reverse proxies. Only requests from these addresses
// are permitted to set X-Forwarded-For / X-Real-Ip headers.
func isPrivateIP(ip string) bool {
	privateRanges := []string{
		"127.", "::1",        // loopback
		"10.",                // RFC-1918 class A
		"192.168.",           // RFC-1918 class C
		"172.16.", "172.17.", "172.18.", "172.19.",
		"172.20.", "172.21.", "172.22.", "172.23.",
		"172.24.", "172.25.", "172.26.", "172.27.",
		"172.28.", "172.29.", "172.30.", "172.31.", // RFC-1918 class B
	}
	for _, prefix := range privateRanges {
		if strings.HasPrefix(ip, prefix) {
			return true
		}
	}
	return false
}

// realIP extracts the real client IP, respecting X-Forwarded-For only when
// the immediate peer (RemoteAddr) is a trusted private/proxy address.
func realIP(r *http.Request) string {
	// Strip port from RemoteAddr to get the peer IP
	remoteIP := r.RemoteAddr
	if idx := strings.LastIndex(remoteIP, ":"); idx != -1 {
		remoteIP = remoteIP[:idx]
	}

	// Only trust forwarded headers when the request comes from a private address
	if isPrivateIP(remoteIP) {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			// Take the rightmost entry — the one appended by the trusted proxy. The leftmost
			// entries are client-supplied and spoofable, which would let an attacker rotate
			// the rate-limit key by sending a unique X-Forwarded-For per request.
			parts := strings.Split(fwd, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
		if fwd := r.Header.Get("X-Real-Ip"); fwd != "" {
			return strings.TrimSpace(fwd)
		}
	}

	return remoteIP
}

// withLoginRateLimit wraps a handleFunc with per-IP rate limiting.
// Returns 429 Too Many Requests when the limit is exceeded.
func withLoginRateLimit(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		ip := realIP(r)
		limiter := loginRateLimiter.get(ip)
		if !limiter.Allow() {
			w.Header().Set("Retry-After", "60")
			return http.StatusTooManyRequests, errTooManyRequests
		}
		return fn(w, r, data)
	}
}

// withShareRateLimit wraps a handleFunc with per-IP rate limiting for public share endpoints.
// Allows 30 requests per minute with a burst of 10 to cover normal browsing of a share,
// while preventing automated password brute-forcing.
func withShareRateLimit(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		ip := realIP(r)
		limiter := shareRateLimiter.getWithRate(ip, rate.Every(time.Minute/30), 10)
		if !limiter.Allow() {
			w.Header().Set("Retry-After", "60")
			return http.StatusTooManyRequests, errTooManyRequests
		}
		return fn(w, r, data)
	}
}

var errTooManyRequests = &rateLimitError{}

type rateLimitError struct{}

func (e *rateLimitError) Error() string {
	return "too many login attempts, please try again later"
}
