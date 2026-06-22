package http

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	activitydb "github.com/gtsteffaniak/filebrowser/backend/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/state"
)

// resolveActivityPathFilters validates source/path filters and scopes paths to the actor's access.
func resolveActivityPathFilters(d *requestContext, filter *activitydb.QueryFilter) (int, error) {
	if filter.Source == "" && filter.PathPrefix == "" && filter.PathGlob == "" {
		return 0, nil
	}
	if filter.PathPrefix != "" && filter.PathGlob != "" {
		return http.StatusBadRequest, fmt.Errorf("use path or pathGlob, not both")
	}
	if filter.Source == "" && (filter.PathPrefix != "" || filter.PathGlob != "") {
		return http.StatusBadRequest, fmt.Errorf("source is required when filtering by path")
	}

	sourceInfo, ok := config.Server.NameToSource[filter.Source]
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("invalid source: %s", filter.Source)
	}
	if _, err := d.user.GetScopeForSourceName(filter.Source); err != nil {
		return http.StatusForbidden, err
	}
	if filter.PathPrefix == "" && filter.PathGlob == "" {
		return 0, nil
	}
	idx := indexing.GetIndex(sourceInfo.Name)
	if idx == nil {
		return http.StatusBadRequest, fmt.Errorf("index not found for source: %s", filter.Source)
	}

	userScope, err := d.user.GetScopeForSourceName(filter.Source)
	if err != nil {
		return http.StatusForbidden, err
	}

	if filter.PathPrefix != "" {
		indexPath, status, err := resolveActivityIndexPath(d, idx, userScope, filter.PathPrefix)
		if err != nil {
			return status, err
		}
		filter.PathPrefix = indexPath
	}
	if filter.PathGlob != "" {
		scopedGlob, status, err := resolveActivityPathGlob(d, idx, userScope, filter.PathGlob)
		if err != nil {
			return status, err
		}
		filter.PathGlob = scopedGlob
	}
	return 0, nil
}

func resolveActivityIndexPath(d *requestContext, idx *indexing.Index, userScope, clientPath string) (string, int, error) {
	clientPath = strings.TrimSpace(clientPath)
	if clientPath == "" {
		return "", http.StatusBadRequest, fmt.Errorf("path is empty")
	}
	cleanPath, err := utils.SanitizePath(clientPath)
	if err != nil {
		return "", http.StatusBadRequest, err
	}
	clientPath = cleanPath
	if !strings.HasPrefix(clientPath, "/") {
		clientPath = "/" + clientPath
	}

	userScope = strings.TrimRight(userScope, "/")
	fullIndexPath := clientPath
	if userScope != "" && userScope != "/" {
		if !strings.HasPrefix(clientPath, userScope+"/") && clientPath != userScope {
			fullIndexPath = utils.JoinPathAsUnix(userScope, strings.TrimPrefix(clientPath, "/"))
		}
	}

	isDir := strings.HasSuffix(fullIndexPath, "/")
	fullPath := idx.MakeIndexPath(fullIndexPath, isDir)
	if accessStore != nil && !accessStore.Permitted(idx.Path, fullPath, d.user.Username) {
		return "", http.StatusForbidden, fmt.Errorf("user is not allowed to access this location")
	}
	return strings.TrimSuffix(fullIndexPath, "/"), 0, nil
}

func resolveActivityPathGlob(d *requestContext, idx *indexing.Index, userScope, glob string) (string, int, error) {
	glob = strings.TrimSpace(glob)
	if glob == "" {
		return "", http.StatusBadRequest, fmt.Errorf("pathGlob is empty")
	}
	cleanGlob, err := utils.SanitizePath(glob)
	if err != nil {
		return "", http.StatusBadRequest, err
	}
	glob = cleanGlob

	userScope = strings.TrimRight(userScope, "/")
	var scoped string
	if strings.HasPrefix(glob, "/") {
		scoped = glob
	} else if userScope != "" && userScope != "/" {
		scoped = userScope + "/" + glob
	} else {
		scoped = "/" + glob
	}

	if userScope != "" && userScope != "/" && !strings.HasPrefix(scoped, userScope) {
		return "", http.StatusForbidden, fmt.Errorf("glob pattern outside user scope")
	}

	rootPath := userScope
	if rootPath == "" {
		rootPath = "/"
	}
	fullPath := idx.MakeIndexPath(rootPath, true)
	if accessStore != nil && !accessStore.Permitted(idx.Path, fullPath, d.user.Username) {
		return "", http.StatusForbidden, fmt.Errorf("user is not allowed to access this location")
	}
	return scoped, 0, nil
}

// enforceActivityPathGlobFilter restricts path glob queries to admins.
func enforceActivityPathGlobFilter(r *http.Request, d *requestContext) (int, error) {
	if d.user.Permissions.Admin {
		return 0, nil
	}
	if strings.TrimSpace(r.URL.Query().Get("pathGlob")) != "" {
		return http.StatusForbidden, fmt.Errorf("forbidden: pathGlob requires admin")
	}
	return 0, nil
}

// resolveActivityShareAccess validates share hash filters and configures share-owner scoping for non-admins.
func resolveActivityShareAccess(d *requestContext, filter *activitydb.QueryFilter) (int, error) {
	if filter.ShareHash != "" {
		share, err := state.GetShare(filter.ShareHash)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("share not found")
		}
		if !d.user.Permissions.Admin && share.UserID != d.user.ID {
			return http.StatusForbidden, fmt.Errorf("forbidden: not your share")
		}
	}

	if d.user.Permissions.Admin {
		return 0, nil
	}

	if filter.Scope == "shares" || filter.ShareHash != "" {
		filter.UserFilter = false
		filter.ShareOwnerUserID = d.user.ID
		filter.ShareOwnerFilter = true
		shares, err := state.GetSharesByUserID(d.user.ID)
		if err != nil {
			return http.StatusInternalServerError, fmt.Errorf("failed to load user shares: %w", err)
		}
		hashes := make([]string, 0, len(shares))
		for _, s := range shares {
			hashes = append(hashes, s.Hash)
		}
		filter.OwnedShareHashes = hashes
	}
	return 0, nil
}
