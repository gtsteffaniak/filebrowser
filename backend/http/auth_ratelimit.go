package http

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gtsteffaniak/go-cache/cache"
	"golang.org/x/time/rate"
)

// Built-in auth rate limits (per process). Toggle all off with http.disableRateLimit.
//
// Credential tier (login, OTP verify): dual token buckets (per IP + per username) plus
// failed-login lockout (per IP+username). Tune burst ≤ maxAttempts so rapid floods hit
// HTTP 429 from the bucket before lockout; lockout covers paced attacks that stay under RPM.
const (
	authCredentialRPM          = 10 // sustained ~1 attempt / 6s per IP and per username
	authCredentialBurst        = 8  // rapid burst; 9th immediate attempt gets 429
	authModerateRPM            = 30 // signup, logout, OTP generate
	authModerateBurst          = 10
	authOIDCRPM                = 60 // OIDC redirects (browser-driven, higher ceiling)
	authOIDCBurst              = 20
	authAuthenticatedRPM       = 180 // session/token management for logged-in users
	authAuthenticatedBurst     = 60
	authFailedLoginMaxAttempts = 8 // lockout after N consecutive 401s (same IP + username)
	authFailedLoginLockoutMins = 15
	// Evict idle per-key token buckets so unique IPs/usernames cannot grow without bound.
	authLimiterEntryTTL = 24 * time.Hour
)

// In-memory failed-attempt counters and lockout flags (per server process).
var (
	authFailCounts = cache.NewCache[int](time.Minute, 10*time.Minute)
	authLockouts   = cache.NewCache[bool](time.Minute, 10*time.Minute)
)

// Per-route-class token buckets (keys: IP or username). Expired entries are dropped by go-cache.
var (
	authRateLimitCredentialByIP       = cache.NewCache[*rate.Limiter](authLimiterEntryTTL)
	authRateLimitCredentialByUsername = cache.NewCache[*rate.Limiter](authLimiterEntryTTL)
	authRateLimitModerateByIP         = cache.NewCache[*rate.Limiter](authLimiterEntryTTL)
	authRateLimitOIDCByIP             = cache.NewCache[*rate.Limiter](authLimiterEntryTTL)
	authRateLimitAuthenticatedByUser  = cache.NewCache[*rate.Limiter](authLimiterEntryTTL)
)

const authRateKeySep = "\x1e"

// AuthRateLimitKind selects which /api/auth rate limit tier applies (see withRateLimit and withRateLimitChain).
type AuthRateLimitKind int

const (
	// AuthRateLimitCredential: strict per-IP and per-username limits; no failed-login lockout.
	AuthRateLimitCredential AuthRateLimitKind = iota
	// AuthRateLimitCredentialLockout: same limits as Credential, plus lockout after repeated 401s.
	AuthRateLimitCredentialLockout
	AuthRateLimitModerate
	AuthRateLimitOIDC
	// AuthRateLimitAuthenticated: per logged-in username (compose: withUser(withRateLimitChain(AuthRateLimitAuthenticated, h))).
	AuthRateLimitAuthenticated
)

// withRateLimitChain applies a rate limit tier then fn for nesting inside withUser / withOrWithoutUser / withoutUser.
func withRateLimitChain(kind AuthRateLimitKind, fn handleFunc) handleFunc {
	switch kind {
	case AuthRateLimitCredential:
		return withAuthRateLimitCredential(false, fn)
	case AuthRateLimitCredentialLockout:
		return withAuthRateLimitCredential(true, fn)
	case AuthRateLimitModerate:
		return withAuthRateLimitModerate(fn)
	case AuthRateLimitOIDC:
		return withAuthRateLimitOIDC(fn)
	case AuthRateLimitAuthenticated:
		return withAuthRateLimitAuthenticated(fn)
	default:
		return fn
	}
}

// withRateLimit registers a rate-limited route (same shape as withTimeout: option first, handler second).
func withRateLimit(kind AuthRateLimitKind, fn handleFunc) http.HandlerFunc {
	return wrapHandler(withRateLimitChain(kind, fn))
}

func authRateLimitActive() bool {
	if config.Http.DisableRateLimit {
		return false
	}
	if config.Auth.Methods.NoAuth {
		return false
	}
	return true
}

func tokenBucketLimiter(c *cache.KeyCache[*rate.Limiter], key string, requestsPerMinute, burst int) *rate.Limiter {
	if requestsPerMinute < 1 {
		requestsPerMinute = 1
	}
	if burst < 1 {
		burst = 1
	}
	if lim, ok := c.Get(key); ok && lim != nil {
		c.SetWithExp(key, lim, authLimiterEntryTTL)
		return lim
	}
	lim := rate.NewLimiter(rate.Limit(float64(requestsPerMinute))/60.0, burst)
	c.SetWithExp(key, lim, authLimiterEntryTTL)
	return lim
}

func retryAfterSeconds(lim *rate.Limiter) int {
	res := lim.Reserve()
	delay := res.Delay()
	res.Cancel()
	secs := int(math.Ceil(delay.Seconds()))
	if secs < 1 {
		secs = 1
	}
	return secs
}

func allowCredential(r *http.Request, loginUsername string) (retryAfter int, ok bool) {
	ip := getRemoteIP(r)
	limIP := tokenBucketLimiter(authRateLimitCredentialByIP, ip, authCredentialRPM, authCredentialBurst)
	if !limIP.Allow() {
		return retryAfterSeconds(limIP), false
	}
	if loginUsername != "" {
		limUser := tokenBucketLimiter(authRateLimitCredentialByUsername, loginUsername, authCredentialRPM, authCredentialBurst)
		if !limUser.Allow() {
			return retryAfterSeconds(limUser), false
		}
	}
	return 0, true
}

func allowModerate(r *http.Request) (retryAfter int, ok bool) {
	ip := getRemoteIP(r)
	lim := tokenBucketLimiter(authRateLimitModerateByIP, ip, authModerateRPM, authModerateBurst)
	if !lim.Allow() {
		return retryAfterSeconds(lim), false
	}
	return 0, true
}

func allowOIDC(r *http.Request) (retryAfter int, ok bool) {
	ip := getRemoteIP(r)
	lim := tokenBucketLimiter(authRateLimitOIDCByIP, ip, authOIDCRPM, authOIDCBurst)
	if !lim.Allow() {
		return retryAfterSeconds(lim), false
	}
	return 0, true
}

func allowAuthenticated(username string) (retryAfter int, ok bool) {
	lim := tokenBucketLimiter(authRateLimitAuthenticatedByUser, username, authAuthenticatedRPM, authAuthenticatedBurst)
	if !lim.Allow() {
		return retryAfterSeconds(lim), false
	}
	return 0, true
}

func authFailKey(ip, username string) string {
	return "fail" + authRateKeySep + ip + authRateKeySep + username
}

func authLockKey(ip, username string) string {
	return "lock" + authRateKeySep + ip + authRateKeySep + username
}

func authLockoutRetryAfterSecs() int {
	return authFailedLoginLockoutMins * 60
}

func isAuthLockout(ip, username string) bool {
	locked, ok := authLockouts.Get(authLockKey(ip, username))
	return ok && locked
}

func clearAuthLockout(ip, username string) {
	authLockouts.Delete(authLockKey(ip, username))
	authFailCounts.Delete(authFailKey(ip, username))
}

func recordAuthFailure(ip, username string) {
	if username == "" {
		return
	}
	window := time.Duration(authFailedLoginLockoutMins) * time.Minute
	fk := authFailKey(ip, username)
	n := 0
	if v, ok := authFailCounts.Get(fk); ok {
		n = v
	}
	n++
	authFailCounts.SetWithExp(fk, n, window)
	if n >= authFailedLoginMaxAttempts {
		authLockouts.SetWithExp(authLockKey(ip, username), true, window)
	}
}

// withRateLimitInternal applies token-bucket allow checks: when inactive, calls fn; when denied, 429 + Retry-After.
func withRateLimitInternal(fn handleFunc, allow func(*http.Request, *requestContext) (retryAfter int, ok bool)) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
		if !authRateLimitActive() {
			return fn(w, r, d)
		}
		if after, ok := allow(r, d); !ok {
			w.Header().Set("Retry-After", strconv.Itoa(after))
			return http.StatusTooManyRequests, fmt.Errorf("too many requests")
		}
		return fn(w, r, d)
	}
}

// withAuthRateLimitCredential enforces the credential tier (and optional per-username limits).
// When trackFailures is true, 401 responses increment failed-attempt state for lockout; success clears it.
func withAuthRateLimitCredential(trackFailures bool, fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
		if !authRateLimitActive() {
			return fn(w, r, d)
		}
		username := r.URL.Query().Get("username")
		if isAuthLockout(getRemoteIP(r), username) {
			w.Header().Set("Retry-After", strconv.Itoa(authLockoutRetryAfterSecs()))
			return http.StatusTooManyRequests, fmt.Errorf("too many failed authentication attempts")
		}
		if after, ok := allowCredential(r, username); !ok {
			w.Header().Set("Retry-After", strconv.Itoa(after))
			return http.StatusTooManyRequests, fmt.Errorf("too many requests")
		}
		status, err := fn(w, r, d)
		if !trackFailures {
			return status, err
		}
		ip := getRemoteIP(r)
		if status == http.StatusUnauthorized && username != "" {
			recordAuthFailure(ip, username)
		}
		if err == nil && (status == 0 || status == http.StatusOK) {
			clearAuthLockout(ip, username)
		}
		return status, err
	}
}

func withAuthRateLimitModerate(fn handleFunc) handleFunc {
	return withRateLimitInternal(fn, func(r *http.Request, _ *requestContext) (int, bool) {
		return allowModerate(r)
	})
}

func withAuthRateLimitOIDC(fn handleFunc) handleFunc {
	return withRateLimitInternal(fn, func(r *http.Request, _ *requestContext) (int, bool) {
		return allowOIDC(r)
	})
}

func withAuthRateLimitAuthenticated(fn handleFunc) handleFunc {
	return withRateLimitInternal(fn, func(_ *http.Request, d *requestContext) (int, bool) {
		if d.user == nil || d.user.Username == "" || d.user.Username == "anonymous" {
			return 0, true
		}
		return allowAuthenticated(d.user.Username)
	})
}
