package settings

import (
	"fmt"
	"strings"
)

var userDefaultsNestedSections = map[string]struct{}{
	"sidebar":     {},
	"listing":     {},
	"fileViewer":  {},
	"search":      {},
	"ui":          {},
	"account":     {},
	"fileLoading": {},
}

// legacyFlatUserDefaultKeys maps deprecated flat userDefaults keys to nested dot paths.
var legacyFlatUserDefaultKeys = map[string]string{
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
	"loginMethod":                "account.loginMethod",
}

var legacyPreviewSubfieldPaths = map[string]string{
	"disableHideSidebar": "sidebar.disableHideOnPreview",
	"image":              "preview.image",
	"video":              "preview.video",
	"audio":              "preview.audio",
	"motionVideoPreview": "preview.motionVideoPreview",
	"office":             "preview.office",
	"popup":              "preview.popup",
	"folder":             "preview.folder",
	"models":             "preview.models",
	"highQuality":        "preview.highQuality",
	"autoplayMedia":      "fileViewer.autoplayMedia",
	"defaultMediaPlayer": "fileViewer.defaultMediaPlayer",
}

var legacyGlobalPermissionKeys = map[string]string{
	"admin":    "account.permissions.admin",
	"api":      "account.permissions.api",
	"share":    "account.permissions.share",
	"realtime": "account.permissions.realtime",
}

var legacySourcePermissionKeys = map[string]struct{}{
	"modify":   {},
	"delete":   {},
	"create":   {},
	"download": {},
}

// MigrateUserDefaultsConfigResult holds migrated userDefaults and migration notes.
type MigrateUserDefaultsConfigResult struct {
	UserDefaults map[string]interface{}
	Warnings     []string
	Changed      bool
}

// NeedsUserDefaultsConfigMigration reports whether userDefaults uses deprecated flat keys.
func NeedsUserDefaultsConfigMigration(raw map[string]interface{}) bool {
	if raw == nil {
		return false
	}
	for key := range raw {
		if _, nested := userDefaultsNestedSections[key]; nested {
			continue
		}
		if key == "permissions" || key == "preview" || key == "fileLoading" {
			continue
		}
		if _, legacy := legacyFlatUserDefaultKeys[key]; legacy {
			return true
		}
	}
	if perms, ok := raw["permissions"].(map[string]interface{}); ok {
		for key := range perms {
			if _, legacy := legacySourcePermissionKeys[key]; legacy {
				return true
			}
		}
	}
	if preview, ok := raw["preview"].(map[string]interface{}); ok {
		for key := range preview {
			if path, ok := legacyPreviewSubfieldPaths[key]; ok {
				parts := strings.Split(path, ".")
				if len(parts) == 2 && parts[0] != "preview" {
					return true
				}
				_ = path
			}
			if key == "disableHideSidebar" || key == "autoplayMedia" || key == "defaultMediaPlayer" {
				return true
			}
		}
	}
	return false
}

// MigrateUserDefaultsConfig converts legacy flat userDefaults to the nested v2 structure.
func MigrateUserDefaultsConfig(raw map[string]interface{}) (MigrateUserDefaultsConfigResult, error) {
	result := MigrateUserDefaultsConfigResult{
		UserDefaults: make(map[string]interface{}),
	}
	if raw == nil {
		return result, nil
	}

	for key, val := range raw {
		if _, nested := userDefaultsNestedSections[key]; nested {
			existing, _ := result.UserDefaults[key].(map[string]interface{})
			incoming, ok := val.(map[string]interface{})
			if !ok {
				return result, fmt.Errorf("userDefaults.%s must be an object", key)
			}
			if existing == nil {
				existing = make(map[string]interface{})
			}
			for subKey, subVal := range incoming {
				existing[subKey] = subVal
			}
			result.UserDefaults[key] = existing
			result.Changed = true
			continue
		}

		switch key {
		case "preview":
			previewMap, ok := val.(map[string]interface{})
			if !ok {
				return result, fmt.Errorf("userDefaults.preview must be an object")
			}
			for subKey, subVal := range previewMap {
				path, ok := legacyPreviewSubfieldPaths[subKey]
				if !ok {
					setNestedValue(result.UserDefaults, "preview."+subKey, subVal)
					continue
				}
				setNestedValue(result.UserDefaults, path, subVal)
				if subKey == "disableHideSidebar" || subKey == "autoplayMedia" || subKey == "defaultMediaPlayer" {
					result.Changed = true
				}
			}
		case "fileLoading":
			fileLoadingMap, ok := val.(map[string]interface{})
			if !ok {
				return result, fmt.Errorf("userDefaults.fileLoading must be an object")
			}
			existing, _ := result.UserDefaults["fileLoading"].(map[string]interface{})
			if existing == nil {
				existing = make(map[string]interface{})
			}
			for subKey, subVal := range fileLoadingMap {
				existing[subKey] = subVal
			}
			result.UserDefaults["fileLoading"] = existing
		case "permissions":
			permsMap, ok := val.(map[string]interface{})
			if !ok {
				return result, fmt.Errorf("userDefaults.permissions must be an object")
			}
			for permKey, permVal := range permsMap {
				if path, ok := legacyGlobalPermissionKeys[permKey]; ok {
					setNestedValue(result.UserDefaults, path, permVal)
					continue
				}
				if _, legacy := legacySourcePermissionKeys[permKey]; legacy {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("userDefaults.permissions.%s is deprecated; set server.sources[].config.defaultPermissions.%s instead", permKey, permKey))
					result.Changed = true
					continue
				}
				setNestedValue(result.UserDefaults, "account.permissions."+permKey, permVal)
			}
		default:
			path, ok := legacyFlatUserDefaultKeys[key]
			if !ok {
				result.Warnings = append(result.Warnings, fmt.Sprintf("unknown userDefaults key %q copied to nested structure unchanged", key))
				result.UserDefaults[key] = val
				continue
			}
			setNestedValue(result.UserDefaults, path, val)
			result.Changed = true
		}
	}

	if !result.Changed && len(result.UserDefaults) > 0 && NeedsUserDefaultsConfigMigration(raw) {
		result.Changed = true
	}
	return result, nil
}

func setNestedValue(root map[string]interface{}, path string, value interface{}) {
	parts := strings.Split(path, ".")
	current := root
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}
		next, ok := current[part].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			current[part] = next
		}
		current = next
	}
}
