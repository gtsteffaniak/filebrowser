package sharedefaults

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// DefaultsToEditable converts defaults template to a share editable payload.
func DefaultsToEditable(d settings.ShareDefaults) share.ShareEditable {
	return share.ShareEditable{
		FrontendShareInfo: share.FrontendShareInfo{
			ShareTheme:           d.ShareTheme,
			DisableAnonymous:     d.DisableAnonymous,
			DisableThumbnails:    d.DisableThumbnails,
			KeepAfterExpiration:  d.KeepAfterExpiration,
			ThemeColor:           d.ThemeColor,
			Title:                d.Title,
			Description:          d.Description,
			Favicon:              d.Favicon,
			QuickDownload:        d.QuickDownload,
			HideNavButtons:       d.HideNavButtons,
			DisableSidebar:       d.DisableSidebar,
			DisableShareCard:     d.DisableShareCard,
			EnforceDarkLightMode: d.EnforceDarkLightMode,
			ViewMode:             d.ViewMode,
			EnableOnlyOffice:     d.EnableOnlyOffice,
			ShareType:            d.ShareType,
			AllowDelete:          d.AllowDelete,
			AllowCreate:          d.AllowCreate,
			AllowModify:          d.AllowModify,
			DisableFileViewer:    d.DisableFileViewer,
			DisableDownload:      d.DisableDownload,
			AllowReplacements:    d.AllowReplacements,
			SidebarLinks:         append([]users.SidebarLink(nil), d.SidebarLinks...),
			ShowHidden:           d.ShowHidden,
			DisableLoginOption:   d.DisableLoginOption,
		},
		ShareLimits: share.ShareLimits{
			MaxBandwidth:             d.MaxBandwidth,
			AllowedUsernames:         append([]string(nil), d.AllowedUsernames...),
			PerUserDownloadLimit:     d.PerUserDownloadLimit,
			ExtractEmbeddedSubtitles: d.ExtractEmbeddedSubtitles,
			DownloadsLimit:           d.DownloadsLimit,
			QuotaLimitBytes:          d.QuotaLimitBytes,
			HideFileExt:              d.HideFileExt,
			Banner:                   d.Banner,
		},
	}
}

// EditableToDefaults extracts defaults-relevant fields from a share editable payload.
func EditableToDefaults(e share.ShareEditable) settings.ShareDefaults {
	return settings.ShareDefaults{
		ShareTheme:               e.ShareTheme,
		DisableAnonymous:         e.DisableAnonymous,
		DisableThumbnails:        e.DisableThumbnails,
		KeepAfterExpiration:      e.KeepAfterExpiration,
		ThemeColor:               e.ThemeColor,
		Title:                    e.Title,
		Description:              e.Description,
		Favicon:                  e.Favicon,
		QuickDownload:            e.QuickDownload,
		HideNavButtons:           e.HideNavButtons,
		DisableSidebar:           e.DisableSidebar,
		DisableShareCard:         e.DisableShareCard,
		EnforceDarkLightMode:     e.EnforceDarkLightMode,
		ViewMode:                 e.ViewMode,
		EnableOnlyOffice:         e.EnableOnlyOffice,
		ShareType:                e.ShareType,
		AllowDelete:              e.AllowDelete,
		AllowCreate:              e.AllowCreate,
		AllowModify:              e.AllowModify,
		DisableFileViewer:        e.DisableFileViewer,
		DisableDownload:          e.DisableDownload,
		AllowReplacements:        e.AllowReplacements,
		SidebarLinks:             append([]users.SidebarLink(nil), e.SidebarLinks...),
		ShowHidden:               e.ShowHidden,
		DisableLoginOption:       e.DisableLoginOption,
		MaxBandwidth:             e.MaxBandwidth,
		AllowedUsernames:         append([]string(nil), e.AllowedUsernames...),
		PerUserDownloadLimit:     e.PerUserDownloadLimit,
		ExtractEmbeddedSubtitles: e.ExtractEmbeddedSubtitles,
		DownloadsLimit:           e.DownloadsLimit,
		QuotaLimitBytes:          e.QuotaLimitBytes,
		HideFileExt:              e.HideFileExt,
		Banner:                   e.Banner,
	}
}
