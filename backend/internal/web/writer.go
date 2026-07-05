package web

import (
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// ResponseWriterWrapper wraps http.ResponseWriter to capture status code and username.
type ResponseWriterWrapper struct {
	http.ResponseWriter
	StatusCode  int
	WroteHeader bool
	PayloadSize int
	User        string
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
