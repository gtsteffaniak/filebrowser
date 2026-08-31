package web

import (
	libError "errors"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
)

// loginMethodHTTPStatus maps a login error to an HTTP status and client-safe message.
func loginMethodHTTPStatus(err error) (int, error) {
	if libError.Is(err, errors.ErrWrongLoginMethod) {
		return http.StatusUnauthorized, errors.ErrInvalidLoginMethod
	}
	return 0, err
}
