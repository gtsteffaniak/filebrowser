package http

import (
	"errors"
	"net/http"
	"os"

	libErrors "github.com/gtsteffaniak/filebrowser/backend/common/errors"
)

func errToStatus(err error) int {
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
