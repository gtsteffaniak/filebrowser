package web

import (
	"fmt"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
)

func denyIfSourceReadOnly(sourceName string) (int, error) {
	idx := indexing.GetIndex(sourceName)
	if idx != nil && idx.Config.ReadOnly {
		return http.StatusForbidden, fmt.Errorf("source is read-only")
	}
	return 0, nil
}
