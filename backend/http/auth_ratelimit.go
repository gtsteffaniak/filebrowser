package http

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/go-cache/cache"
	"golang.org/x/time/rate"
)

// Built-in auth rate limits (per process). Toggle all off with auth.disableRateLimit.
const (
	authCredentialRPM          = 10
	authCredentialBurst        = 8
	authModerateRPM            = 40
	authModerateBurst          = 20
	authOIDCRPM                = 60
	authOIDCBurst              = 30
	authAuthenticatedRPM       = 180
	authAuthenticatedBurst     = 60
	authFailedLoginMaxAttempts = 10
	authFailedLoginLockoutMins = 15
)

// In-memory failed-attempt counters and lockout flags (per server process).
var (
	authFailCounts = cache.NewCache[int](time.Minute, 10*time.Minute)
	authLockouts   = cache.NewCache[bool](time.Minute, 10*time.Minute)
)

var (
	credentialIPLimiters      sync.Map // string -> *rate.Limiter
	credentialUsernameLimiter sync.Map
	moderateIPLimiters        sync.Map
	oidcIPLimiters            sync.Map
	authenticatedUserLimiters sync.Map
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
	if settings.Config.Auth.DisableRateLimit {
		return false
	}
	if settings.Config.Auth.Methods.NoAuth {
		return false
	}
	return true
}

func clientIPForAuthRateLimit(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func limiterFor(m *sync.Map, key string, requestsPerMinute, burst int) *rate.Limiter {
	if requestsPerMinute < 1 {
		requestsPerMinute = 1
	}
	if burst < 1 {
		burst = 1
	}
	if v, ok := m.Load(key); ok {
		if lim, ok := v.(*rate.Limiter); ok {
			return lim
		}
	}
	lim := rate.NewLimiter(rate.Limit(float64(requestsPerMinute))/60.0, burst)
	if actual, loaded := m.LoadOrStore(key, lim); loaded {
		if existing, ok := actual.(*rate.Limiter); ok {
			return existing
		}
	}
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
	ip := clientIPForAuthRateLimit(r)
	limIP := limiterFor(&credentialIPLimiters, "ip:"+ip, authCredentialRPM, authCredentialBurst)
	if !limIP.Allow() {
		return retryAfterSeconds(limIP), false
	}
	if loginUsername != "" {
		limUser := limiterFor(&credentialUsernameLimiter, "u:"+loginUsername, authCredentialRPM, authCredentialBurst)
		if !limUser.Allow() {
			return retryAfterSeconds(limUser), false
		}
	}
	return 0, true
}

func allowModerate(r *http.Request) (retryAfter int, ok bool) {
	ip := clientIPForAuthRateLimit(r)
	lim := limiterFor(&moderateIPLimiters, "ip:"+ip, authModerateRPM, authModerateBurst)
	if !lim.Allow() {
		return retryAfterSeconds(lim), false
	}
	return 0, true
}

func allowOIDC(r *http.Request) (retryAfter int, ok bool) {
	ip := clientIPForAuthRateLimit(r)
	lim := limiterFor(&oidcIPLimiters, "ip:"+ip, authOIDCRPM, authOIDCBurst)
	if !lim.Allow() {
		return retryAfterSeconds(lim), false
	}
	return 0, true
}

func allowAuthenticated(username string) (retryAfter int, ok bool) {
	lim := limiterFor(&authenticatedUserLimiters, "user:"+username, authAuthenticatedRPM, authAuthenticatedBurst)
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

// withAuthRateLimitCredential enforces the credential tier (and optional per-username limits).
// When trackFailures is true, 401 responses increment failed-attempt state for lockout; success clears it.
func withAuthRateLimitCredential(trackFailures bool, fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
		if !authRateLimitActive() {
			return fn(w, r, d)
		}
		username := r.URL.Query().Get("username")
		if isAuthLockout(clientIPForAuthRateLimit(r), username) {
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
		ip := clientIPForAuthRateLimit(r)
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
	return func(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
		if !authRateLimitActive() {
			return fn(w, r, d)
		}
		if after, ok := allowModerate(r); !ok {
			w.Header().Set("Retry-After", strconv.Itoa(after))
			return http.StatusTooManyRequests, fmt.Errorf("too many requests")
		}
		return fn(w, r, d)
	}
}

func withAuthRateLimitOIDC(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
		if !authRateLimitActive() {
			return fn(w, r, d)
		}
		if after, ok := allowOIDC(r); !ok {
			w.Header().Set("Retry-After", strconv.Itoa(after))
			return http.StatusTooManyRequests, fmt.Errorf("too many requests")
		}
		return fn(w, r, d)
	}
}

func withAuthRateLimitAuthenticated(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
		if !authRateLimitActive() {
			return fn(w, r, d)
		}
		if d.user == nil || d.user.Username == "" || d.user.Username == "anonymous" {
			return fn(w, r, d)
		}
		if after, ok := allowAuthenticated(d.user.Username); !ok {
			w.Header().Set("Retry-After", strconv.Itoa(after))
			return http.StatusTooManyRequests, fmt.Errorf("too many requests")
		}
		return fn(w, r, d)
	}
}
