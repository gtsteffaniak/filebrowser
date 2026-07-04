package web

import (
	"context"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

// Context carries per-request state for HTTP handlers.
type Context struct {
	User         *users.User
	ShareUser    *users.User
	FileInfo     iteminfo.ExtendedFileInfo
	Token        string
	Share        share.Share
	ShareValid   bool
	Ctx          context.Context
	MaxBandwidth int
	Data         interface{}
	IndexPath    string
}

// HandleFunc is the signature used by middleware-wrapped handlers.
type HandleFunc func(w http.ResponseWriter, r *http.Request, d *Context) (int, error)
