package http

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func hlsEncodeHTTPStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return 0
	}
	msg := err.Error()
	classifier := encode.FailureClassifier{}
	kind := classifier.Classify(msg).Kind
	switch kind {
	case encode.FailureInputNotFound:
		return http.StatusNotFound
	case encode.FailureInputInvalid, encode.FailureSeek, encode.FailureHardware,
		encode.FailureEncoder, encode.FailureDecoder, encode.FailureOutputEmpty:
		return http.StatusUnprocessableEntity
	case encode.FailureTimeout:
		return http.StatusGatewayTimeout
	case encode.FailurePermission:
		return http.StatusForbidden
	default:
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "context deadline exceeded") || strings.Contains(lower, "timeout") {
			return http.StatusGatewayTimeout
		}
		return http.StatusInternalServerError
	}
}
