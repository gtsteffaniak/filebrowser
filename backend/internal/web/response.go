package web

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	libErrors "github.com/gtsteffaniak/filebrowser/backend/internal/errors"
)

// HttpResponse is the standard JSON error/success envelope.
type HttpResponse struct {
	Status  int    `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
	Token   string `json:"token,omitempty"`
}

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
