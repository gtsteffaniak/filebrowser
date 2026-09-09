package web

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

type hijackableResponseWriter struct {
	http.ResponseWriter
	hijacked bool
}

func (h *hijackableResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	return nil, nil, nil
}

func TestResponseWriterWrapperHijack(t *testing.T) {
	t.Parallel()

	inner := &hijackableResponseWriter{ResponseWriter: httptest.NewRecorder()}
	wrapper := &ResponseWriterWrapper{ResponseWriter: inner}

	hijacker, ok := interface{}(wrapper).(http.Hijacker)
	if !ok {
		t.Fatal("ResponseWriterWrapper must implement http.Hijacker")
	}

	_, _, err := hijacker.Hijack()
	if err != nil {
		t.Fatalf("Hijack() error: %v", err)
	}
	if !inner.hijacked {
		t.Fatal("expected underlying Hijack to be called")
	}
}
