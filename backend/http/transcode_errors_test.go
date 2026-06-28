package http

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestHLSEncodeHTTPStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: http.StatusOK},
		{name: "canceled", err: context.Canceled, want: 0},
		{name: "deadline", err: context.DeadlineExceeded, want: http.StatusGatewayTimeout},
		{name: "deadline message", err: errors.New("context deadline exceeded"), want: http.StatusGatewayTimeout},
		{name: "input not found", err: errors.New("Error opening input: No such file or directory"), want: http.StatusNotFound},
		{name: "permission", err: errors.New("permission denied"), want: http.StatusForbidden},
		{name: "encoder failure", err: errors.New("Unknown encoder 'libfoo'"), want: http.StatusUnprocessableEntity},
		{name: "invalid input", err: errors.New("Invalid data found when processing input"), want: http.StatusUnprocessableEntity},
		{name: "empty output", err: errors.New("Output file is empty"), want: http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := hlsEncodeHTTPStatus(tt.err); got != tt.want {
				t.Fatalf("hlsEncodeHTTPStatus() = %d, want %d", got, tt.want)
			}
		})
	}
}
