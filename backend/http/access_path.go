package http

import (
	"fmt"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
)

// parseAccessQueryPath validates and parses an index path from an API query parameter.
func parseAccessQueryPath(path string) (utils.IndexPath, error) {
	if path == "" {
		return utils.IndexPath{}, fmt.Errorf("path is required")
	}
	return utils.ParseSanitizedIndexPath(path, true)
}

// parseAccessQueryPathOrBadRequest wraps parseAccessQueryPath with a standard HTTP 400 response.
func parseAccessQueryPathOrBadRequest(path string) (utils.IndexPath, int, error) {
	parsed, err := parseAccessQueryPath(path)
	if err != nil {
		return utils.IndexPath{}, http.StatusBadRequest, err
	}
	return parsed, 0, nil
}
