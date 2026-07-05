package activity

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/ports"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

// QueryDeps holds access and share dependencies for activity query filtering.
type QueryDeps struct {
	Access ports.AccessGate
	Shares ports.ShareReader
}

var queryDeps QueryDeps

// SetQueryDeps registers access and share ports for activity queries (called from app.WireServices).
func SetQueryDeps(access ports.AccessGate, shares ports.ShareReader) {
	queryDeps = QueryDeps{Access: access, Shares: shares}
}

func accessPermitted(sourcePath string, indexPath utils.IndexPath, username string) bool {
	if queryDeps.Access == nil {
		return true
	}
	return queryDeps.Access.AccessPermitted(sourcePath, indexPath, username)
}

func getShare(hash string) (share.Share, error) {
	if queryDeps.Shares == nil {
		return share.Share{}, nil
	}
	return queryDeps.Shares.GetShare(hash)
}
