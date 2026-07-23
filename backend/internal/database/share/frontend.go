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
// publicHost and publicScheme must be set when building share/download URLs (resolved in web/proxy.go).
// ownerLookup resolves share owner usernames from stable user ids; may be nil.
func PrepForFrontend(viewer *users.User, r *http.Request, publicHost, publicScheme string, ownerLookup func(uint64) string, links ...*Share) []*ShareFrontend {
	out := make([]*ShareFrontend, 0, len(links))
	for _, link := range links {
		if link == nil {
			continue
		}
		out = append(out, prepForFrontendOne(link, viewer, r, publicHost, publicScheme, ownerLookup))
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

func prepForFrontendOne(link *Share, viewer *users.User, r *http.Request, publicHost, publicScheme string, ownerLookup func(uint64) string) *ShareFrontend {
	snap := *link
	out := copyShareFrontendFromShare(link)
	out.HasPassword = snap.HasPassword()
	if snap.UserID != 0 && ownerLookup != nil {
		out.Username = ownerLookup(snap.UserID)
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
	if r != nil && publicHost != "" {
		out.DownloadURL = PublicShareURL(publicHost, publicScheme, out.Hash, true, snap.Token)
		out.ShareURL = PublicShareURL(publicHost, publicScheme, out.Hash, false, snap.Token)
	}
	return &out
}

// PublicShareURL builds share/download URLs using pre-resolved host and scheme (see web/proxy.go).
func PublicShareURL(host, scheme, hash string, isDirectDownload bool, token string) string {
	tokenParam := ""
	if token != "" && isDirectDownload {
		tokenParam = fmt.Sprintf("&token=%s", url.QueryEscape(token))
	}

	if settings.Config.Http.ExternalUrl != "" {
		if isDirectDownload {
			return fmt.Sprintf("%s%spublic/api/resources/download?hash=%s%s",
				settings.Config.Http.ExternalUrl, settings.Config.Http.BaseURL, hash, tokenParam)
		}
		return fmt.Sprintf("%s%spublic/share/%s",
			settings.Config.Http.ExternalUrl, settings.Config.Http.BaseURL, hash)
	}

	if isDirectDownload {
		return fmt.Sprintf("%s://%s%spublic/api/resources/download?hash=%s%s",
			scheme, host, settings.Config.Http.BaseURL, hash, tokenParam)
	}
	return fmt.Sprintf("%s://%s%spublic/share/%s", scheme, host, settings.Config.Http.BaseURL, hash)
}
