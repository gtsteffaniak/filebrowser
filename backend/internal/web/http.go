package web

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	libErrors "github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-cache/cache"
	"golang.org/x/time/rate"
)

// Context carries per-request state for HTTP handlers.
type Context struct {
	User         *users.User
	ShareUser    *users.User
	FileInfo     iteminfo.ExtendedFileInfo
	Token        string
	Share        share.Share
	ShareValid   bool
	Ctx          context.Context
	MaxBandwidth int
	Data         interface{}
	IndexPath    string
}

// HandleFunc is the signature used by middleware-wrapped handlers.
type HandleFunc func(w http.ResponseWriter, r *http.Request, d *Context) (int, error)

func effectiveFilePerms(d *Context, sourceName string) (users.SourceFilePermissions, error) {
	if d == nil {
		return users.DenyAllSourceFilePermissions(), fmt.Errorf("user context not set")
	}
	var link *share.Share
	if d.Share.Hash != "" {
		link = &d.Share
	}
	return share.EffectiveFilePermissions(d.User, link, sourceName)
}

// HttpResponse is the standard JSON error/success envelope.
type HttpResponse struct {
	Status  int    `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
	Token   string `json:"token,omitempty"`
}

// ResponseWriterWrapper wraps http.ResponseWriter to capture status code and username.
type ResponseWriterWrapper struct {
	http.ResponseWriter
	StatusCode  int
	WroteHeader bool
	PayloadSize int
	User        string
}

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

const authRateKeySep = "\x1e"

// In-memory failed-attempt counters and lockout flags (per server process).
var (
	authFailCounts = cache.NewCache[int](time.Minute, 10*time.Minute)
	authLockouts   = cache.NewCache[bool](time.Minute, 10*time.Minute)
)

// Per-route-class token buckets (keys: IP or username). Expired entries are dropped by go-cache.
var (
	authRateLimitCredentialByIP       = cache.NewCache[*rate.Limiter](authLimiterEntryTTL)
	authRateLimitCredentialByUsername   = cache.NewCache[*rate.Limiter](authLimiterEntryTTL)
	authRateLimitModerateByIP         = cache.NewCache[*rate.Limiter](authLimiterEntryTTL)
	authRateLimitOIDCByIP             = cache.NewCache[*rate.Limiter](authLimiterEntryTTL)
	authRateLimitAuthenticatedByUser  = cache.NewCache[*rate.Limiter](authLimiterEntryTTL)
)

// ErrToStatus maps domain errors to HTTP status codes.
func ErrToStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case os.IsPermission(err):
		return http.StatusForbidden
	case errors.Is(err, libErrors.ErrAccessDenied):
		return http.StatusForbidden
	case os.IsNotExist(err), err == libErrors.ErrNotExist:
		return http.StatusNotFound
	case os.IsExist(err), err == libErrors.ErrExist:
		return http.StatusConflict
	case errors.Is(err, libErrors.ErrPermissionDenied):
		return http.StatusForbidden
	case errors.Is(err, libErrors.ErrInvalidRequestParams):
		return http.StatusBadRequest
	case errors.Is(err, libErrors.ErrIsDirectory):
		return http.StatusMethodNotAllowed
	default:
		return http.StatusInternalServerError
	}
}

// RenderJSON writes a JSON response, optionally gzip-compressed.
func RenderJSON(w http.ResponseWriter, r *http.Request, data interface{}, statusCode ...int) (int, error) {
	code := http.StatusOK
	if len(statusCode) > 0 && statusCode[0] != 0 {
		code = statusCode[0]
	}

	marsh, err := json.Marshal(data)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	payloadSizeKB := len(marsh) / 1024
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if acceptsGzip(r) && payloadSizeKB > 10 {
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(code)
		gz := gzip.NewWriter(w)
		defer gz.Close()
		if _, err := gz.Write(marsh); err != nil {
			return http.StatusInternalServerError, err
		}
	} else {
		w.WriteHeader(code)
		if _, err := w.Write(marsh); err != nil {
			return http.StatusInternalServerError, err
		}
	}
	return code, nil
}

func acceptsGzip(r *http.Request) bool {
	ae := r.Header.Get("Accept-Encoding")
	return ae != "" && strings.Contains(ae, "gzip")
}

// WriteHeader captures the status code and ensures it is only written once.
func (w *ResponseWriterWrapper) WriteHeader(statusCode int) {
	if !w.WroteHeader {
		if statusCode == 0 {
			statusCode = http.StatusInternalServerError
		}
		w.StatusCode = statusCode
		w.ResponseWriter.WriteHeader(statusCode)
		w.WroteHeader = true
	}
}

// Write ensures WriteHeader is called before writing the body.
func (w *ResponseWriterWrapper) Write(b []byte) (int, error) {
	if !w.WroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Flush implements http.Flusher when the underlying writer supports it.
func (w *ResponseWriterWrapper) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// SetUserInResponseWriter records the authenticated username on the wrapper when present.
func SetUserInResponseWriter(w http.ResponseWriter, user *users.User) {
	if wrappedWriter, ok := w.(*ResponseWriterWrapper); ok && user != nil {
		wrappedWriter.User = user.Username
	}
}

// GetRemoteIP resolves the client IP, honoring trusted proxy headers when configured.
func GetRemoteIP(r *http.Request) string {
	cfg := &settings.Config

	xff := r.Header.Get("X-Forwarded-For")
	if cfg.Http.TrustedHeaders["x-forwarded-for"] && xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	xri := r.Header.Get("X-Real-IP")
	if cfg.Http.TrustedHeaders["x-real-ip"] && xri != "" {
		return xri
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

// GetScheme returns the request scheme (http or https).
func GetScheme(r *http.Request) string {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func toASCIIFilename(fileName string) string {
	var result strings.Builder
	for _, r := range fileName {
		if r > 127 {
			result.WriteRune('_')
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func SetContentDisposition(w http.ResponseWriter, r *http.Request, fileName string, forceInline bool) {
	dispositionType := "attachment"
	if forceInline || r.URL.Query().Get("inline") == "true" {
		dispositionType = "inline"
		w.Header().Set("Content-Security-Policy", "script-src 'none'")
	}
	asciiFileName := toASCIIFilename(fileName)
	encodedFileName := url.PathEscape(fileName)
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%q; filename*=utf-8''%s", dispositionType, asciiFileName, encodedFileName))
}

func IsOnlyOfficeCompatibleFile(fileName string) bool {
	return iteminfo.IsOnlyOffice(fileName)
}

// WithRateLimitChain applies a rate limit tier then fn for nesting inside withUser / withOrWithoutUser / withoutUser.
func WithRateLimitChain(kind AuthRateLimitKind, fn HandleFunc) HandleFunc {
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

func authRateLimitActive() bool {
	if settings.Config.Http.DisableRateLimit {
		return false
	}
	if settings.Config.Auth.Methods.NoAuth {
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
	ip := GetRemoteIP(r)
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
	ip := GetRemoteIP(r)
	lim := tokenBucketLimiter(authRateLimitModerateByIP, ip, authModerateRPM, authModerateBurst)
	if !lim.Allow() {
		return retryAfterSeconds(lim), false
	}
	return 0, true
}

func allowOIDC(r *http.Request) (retryAfter int, ok bool) {
	ip := GetRemoteIP(r)
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

// withRateLimit registers a rate-limited route (same shape as withTimeout: option first, handler second).
func withRateLimit(kind AuthRateLimitKind, fn HandleFunc) http.HandlerFunc {
	return wrapHandler(WithRateLimitChain(kind, fn))
}

// withRateLimitChain applies a rate limit tier then fn for nesting inside withUser / withOrWithoutUser / withoutUser.
func withRateLimitChain(kind AuthRateLimitKind, fn HandleFunc) HandleFunc {
	return WithRateLimitChain(kind, fn)
}

func withRateLimitInternal(fn HandleFunc, allow func(*http.Request, *Context) (retryAfter int, ok bool)) HandleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
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
func withAuthRateLimitCredential(trackFailures bool, fn HandleFunc) HandleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
		if !authRateLimitActive() {
			return fn(w, r, d)
		}
		username := r.URL.Query().Get("username")
		if isAuthLockout(GetRemoteIP(r), username) {
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
		ip := GetRemoteIP(r)
		if status == http.StatusUnauthorized && username != "" {
			recordAuthFailure(ip, username)
		}
		if err == nil && (status == 0 || status == http.StatusOK) {
			clearAuthLockout(ip, username)
		}
		return status, err
	}
}

func withAuthRateLimitModerate(fn HandleFunc) HandleFunc {
	return withRateLimitInternal(fn, func(r *http.Request, _ *Context) (int, bool) {
		return allowModerate(r)
	})
}

func withAuthRateLimitOIDC(fn HandleFunc) HandleFunc {
	return withRateLimitInternal(fn, func(r *http.Request, _ *Context) (int, bool) {
		return allowOIDC(r)
	})
}

func withAuthRateLimitAuthenticated(fn HandleFunc) HandleFunc {
	return withRateLimitInternal(fn, func(_ *http.Request, d *Context) (int, bool) {
		if d.User == nil || d.User.Username == "" || d.User.Username == "anonymous" {
			return 0, true
		}
		return allowAuthenticated(d.User.Username)
	})
}

// healthHandler returns a simple JSON health check response.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := HttpResponse{Message: "ok"}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
