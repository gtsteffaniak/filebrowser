package http

import (
	"fmt"
	"net/http"

	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/net/webdav"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
)

func createWebDAVHandler(prefix string) handleFunc {
	var locks = webdav.NewMemLS()
	// webdavHandler serves the webdav requests.
	return func(writer http.ResponseWriter, request *http.Request, d *requestContext) (int, error) {
		fullPath := request.PathValue("path")
		source := request.PathValue("scope")
		if !userHasReadWriteAccess(d) {
			// (reddec): we're currently allowing WebDAV access only for users with R/W access for simplicity.
			// in the follow-ups, we need to add mapping of webdav operations and permissions (which is trickier than it looks like).
			return http.StatusForbidden, fmt.Errorf("user is not allowed to create or modify")
		}
		userscope, err := settings.GetScopeFromSourceName(d.user.Scopes, source)
		if err != nil {
			logger.Debugf("error getting scope from source name: %v", err)
			return http.StatusForbidden, err
		}

		virtualPath := utils.JoinPathAsUnix(userscope, fullPath)

		idx := indexing.GetIndex(source)
		if idx == nil {
			logger.Debugf("source %s not found", source)
			return http.StatusNotFound, fmt.Errorf("source %s not found", source)
		}

		realPath, _, _ := idx.GetRealPath(virtualPath)
		scopePath, _, _ := idx.GetRealPath(userscope)
		logger.Debugf("webdav: method=%s, request=%s, source=%s, path=%s, virtual_path=%s, real_path=%s, scope_path=%s", request.Method, request.URL.Path, source, fullPath, virtualPath, realPath, scopePath)

		wd := &webdav.Handler{
			Prefix:     prefix + "/" + source,
			FileSystem: webdav.Dir(scopePath),
			LockSystem: locks,
			Logger: func(request *http.Request, err error) {
				if err != nil {
					logger.Errorf("webdav handler failed on path %s: %s", request.URL.Path, err)
				}
			},
		}

		wd.ServeHTTP(writer, request)
		return 0, nil // errors and responses (XML-formatted) are handled by webdav handler
	}
}
func userHasReadWriteAccess(d *requestContext) bool {
	return d.user.Permissions.Create && d.user.Permissions.Delete && d.user.Permissions.Modify && d.user.Permissions.Download
}
