package share

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// PrepForFrontend builds API-safe ShareFrontend copies (never exposes backend Share fields).
// When r is non-nil, download and share URLs are derived from the request (or ExternalUrl).
func PrepForFrontend(viewer *users.User, usersStore *users.Storage, r *http.Request, links ...*Share) []*ShareFrontend {
	out := make([]*ShareFrontend, 0, len(links))
	for _, link := range links {
		if link == nil {
			continue
		}
		out = append(out, prepForFrontendOne(link, viewer, usersStore, r))
	}
	return utils.NonNilSlice(out)
}

func copyShareFrontendFromShare(link *Share) ShareFrontend {
	snap := *link
	out := ShareFrontend{
		ShareEditable: ShareEditable{
			FrontendShareInfo: snap.FrontendShareInfo,
			ShareLimits:       snap.ShareLimits,
		},
		ShareColumns: snap.ShareColumns,
	}
	if snap.AllowedUsernames != nil {
		out.AllowedUsernames = append([]string(nil), snap.AllowedUsernames...)
	}
	if snap.SidebarLinks != nil {
		out.SidebarLinks = make([]users.SidebarLink, len(snap.SidebarLinks))
		copy(out.SidebarLinks, snap.SidebarLinks)
	}
	return out
}

func prepForFrontendOne(link *Share, viewer *users.User, usersStore *users.Storage, r *http.Request) *ShareFrontend {
	snap := *link
	out := copyShareFrontendFromShare(link)
	out.HasPassword = snap.HasPassword()
	if snap.UserID != 0 && usersStore != nil {
		if owner, err := usersStore.Get(snap.UserID); err == nil && owner != nil {
			out.Username = owner.Username
		}
	}
	if sourceInfo, ok := resolveSource(snap.SourcePath); ok {
		out.PathExists = utils.CheckPathExists(filepath.Join(sourceInfo.Path, out.Path))
	}
	if viewer != nil {
		out.CanEditShare = snap.UserCanEdit(viewer)
		if out.CanEditShare {
			out.SourceURL = snap.SourceURL(viewer)
		}
	}
	if r != nil {
		out.DownloadURL = URLFromRequest(r, out.Hash, true, snap.Token)
		out.ShareURL = URLFromRequest(r, out.Hash, false, snap.Token)
	}
	return &out
}

// URLFromRequest builds a public share or direct-download URL from the request and server config.
func URLFromRequest(r *http.Request, hash string, isDirectDownload bool, token string) string {
	tokenParam := ""
	if token != "" && isDirectDownload {
		tokenParam = fmt.Sprintf("&token=%s", url.QueryEscape(token))
	}

	if settings.Config.Server.ExternalUrl != "" {
		if isDirectDownload {
			return fmt.Sprintf("%s%spublic/api/resources/download?hash=%s%s",
				settings.Config.Server.ExternalUrl, settings.Config.Server.BaseURL, hash, tokenParam)
		}
		return fmt.Sprintf("%s%spublic/share/%s",
			settings.Config.Server.ExternalUrl, settings.Config.Server.BaseURL, hash)
	}

	host := r.Host
	scheme := requestScheme(r)
	if forwardedHost := r.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
		host = forwardedHost
		if forwardedProto := r.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
			scheme = forwardedProto
		} else {
			scheme = "https"
		}
	}
	if isDirectDownload {
		return fmt.Sprintf("%s://%s%spublic/api/resources/download?hash=%s%s",
			scheme, host, settings.Config.Server.BaseURL, hash, tokenParam)
	}
	return fmt.Sprintf("%s://%s%spublic/share/%s", scheme, host, settings.Config.Server.BaseURL, hash)
}

func requestScheme(r *http.Request) string {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
