package settings

import "strings"

// userJSONFieldEnforcementPaths maps flat user JSON field names (PATCH which / data keys)
// to user-defaults enforcement dot-paths (see UserDefaultsEnforcement).
var userJSONFieldEnforcementPaths = map[string]string{
	"disableQuickToggles":        "sidebar.disableQuickToggles",
	"hideSidebarFileActions":     "sidebar.hideFileActions",
	"stickySidebar":              "sidebar.sticky",
	"hideFilesInTree":            "sidebar.hideFiles",
	"showToolsInSidebar":         "sidebar.showTools",
	"deleteWithoutConfirming":    "listing.deleteWithoutConfirming",
	"dateFormat":                 "listing.dateFormat",
	"showHidden":                 "listing.showHidden",
	"quickDownload":              "listing.quickDownload",
	"showSelectMultiple":         "listing.showSelectMultiple",
	"singleClick":                "listing.singleClick",
	"hideFileExt":                "listing.hideFileExt",
	"showCopyPath":               "listing.showCopyPath",
	"deleteAfterArchive":         "listing.deleteAfterArchive",
	"viewMode":                   "listing.viewMode",
	"gallerySize":                "listing.gallerySize",
	"disablePreviewExt":          "preview.disablePreviewExt",
	"disableSearchOptions":       "search.disableOptions",
	"editorQuickSave":            "fileViewer.editorQuickSave",
	"preferEditorForMarkdown":    "fileViewer.preferEditorForMarkdown",
	"debugOffice":                "fileViewer.debugOffice",
	"disableViewingExt":          "fileViewer.disableViewingExt",
	"disableOnlyOfficeExt":       "fileViewer.disableOnlyOfficeExt",
	"disableOfficePreviewExt":    "fileViewer.disableOnlyOfficeExt",
	"darkMode":                   "ui.darkMode",
	"themeColor":                 "ui.themeColor",
	"customTheme":                "ui.customTheme",
	"locale":                     "ui.locale",
	"lockPassword":               "account.lockPassword",
	"disableSettings":            "account.disableSettings",
	"disableUpdateNotifications": "account.disableUpdateNotifications",
}

var previewJSONSubfields = map[string]string{
	"image":              "preview.image",
	"video":              "preview.video",
	"audio":              "preview.audio",
	"motionVideoPreview": "preview.motionVideoPreview",
	"office":             "preview.office",
	"popup":              "preview.popup",
	"folder":             "preview.folder",
	"models":             "preview.models",
	"autoplayMedia":      "fileViewer.autoplayMedia",
	"defaultMediaPlayer": "fileViewer.defaultMediaPlayer",
	"disableHideSidebar": "sidebar.disableHideOnPreview",
}

var fileLoadingJSONSubfields = map[string]string{
	"maxConcurrentUpload": "fileLoading.maxConcurrentUpload",
	"uploadChunkSizeMb":   "fileLoading.uploadChunkSizeMb",
	"downloadChunkSizeMb": "fileLoading.downloadChunkSizeMb",
}

// EnforcedPathsTouchedByUserJSONField returns enforcement paths a user JSON field name may affect.
func EnforcedPathsTouchedByUserJSONField(jsonField string) []string {
	jsonField = strings.TrimSpace(jsonField)
	if jsonField == "" {
		return nil
	}
	if path, ok := userJSONFieldEnforcementPaths[jsonField]; ok {
		return []string{path}
	}
	switch jsonField {
	case "preview":
		out := make([]string, 0, len(previewJSONSubfields))
		for _, p := range previewJSONSubfields {
			out = append(out, p)
		}
		return out
	case "fileLoading":
		out := make([]string, 0, len(fileLoadingJSONSubfields))
		for _, p := range fileLoadingJSONSubfields {
			out = append(out, p)
		}
		return out
	case "permissions", "Permissions":
		return []string{
			"account.permissions.admin",
			"account.permissions.api",
			"account.permissions.share",
			"account.permissions.realtime",
		}
	default:
		if path, ok := previewJSONSubfields[jsonField]; ok {
			return []string{path}
		}
		if path, ok := fileLoadingJSONSubfields[jsonField]; ok {
			return []string{path}
		}
	}
	return nil
}

// ErrEnforcedUserField is returned when a self-service user update touches an enforced default.
type ErrEnforcedUserField struct {
	Field string
	Path  string
}

func (e ErrEnforcedUserField) Error() string {
	if e.Field != "" {
		return "setting is enforced by administrator: " + e.Field
	}
	return "setting is enforced by administrator: " + e.Path
}

// ValidateSelfUserUpdateNotEnforced rejects PATCH fields that are locked by user-defaults enforcement.
func ValidateSelfUserUpdateNotEnforced(which []string, enforced UserDefaultsEnforcement) error {
	paths := EnforcedPathSet(enforced)
	if len(paths) == 0 {
		return nil
	}
	updateAll := len(which) == 0
	if !updateAll && len(which) == 1 && strings.EqualFold(strings.TrimSpace(which[0]), "all") {
		updateAll = true
	}
	fields := which
	if updateAll {
		fields = allEnforceableUserJSONFields()
	}
	for _, jsonField := range fields {
		jsonField = strings.TrimSpace(jsonField)
		if jsonField == "" || strings.EqualFold(jsonField, "password") {
			continue
		}
		for _, path := range EnforcedPathsTouchedByUserJSONField(jsonField) {
			if _, ok := paths[path]; ok {
				return ErrEnforcedUserField{Field: jsonField, Path: path}
			}
		}
	}
	return nil
}

func allEnforceableUserJSONFields() []string {
	out := make([]string, 0, len(userJSONFieldEnforcementPaths)+2)
	for k := range userJSONFieldEnforcementPaths {
		out = append(out, k)
	}
	out = append(out, "preview", "fileLoading", "permissions")
	return out
}
