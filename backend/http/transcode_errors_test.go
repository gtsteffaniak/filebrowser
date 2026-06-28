package http

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestHLSEncodeHTTPStatus(t *testing.T) {
	t.Parallel()
	if got := hlsEncodeHTTPStatus(nil); got != http.StatusOK {
		t.Fatalf("nil = %d, want 200", got)
	}
	if got := hlsEncodeHTTPStatus(context.Canceled); got != 0 {
		t.Fatalf("canceled = %d, want 0", got)
	}
	if got := hlsEncodeHTTPStatus(context.DeadlineExceeded); got != http.StatusGatewayTimeout {
		t.Fatalf("deadline = %d, want 504", got)
	}
	if got := hlsEncodeHTTPStatus(errors.New("context deadline exceeded")); got != http.StatusGatewayTimeout {
		t.Fatalf("deadline message = %d, want 504", got)
	}
}
