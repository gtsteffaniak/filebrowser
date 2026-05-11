package settings

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
)

// UserDefaultsDeprecationNotice records migrated YAML preference usage.
type UserDefaultsDeprecationNotice struct {
	DeprecatedPath string
	UseInstead     string
	Detail         string
}

func (n UserDefaultsDeprecationNotice) noticeKey() string {
	return n.DeprecatedPath + "\x00" + n.UseInstead + "\x00" + n.Detail
}

// deprecatedUserDefaultsLogPrefix is logged for grep/alert rules.
const deprecatedUserDefaultsLogPrefix = "deprecated userDefaults:"

// LogUserDefaultsDeprecations writes one warning line per deprecation (deduped by caller slice).
func LogUserDefaultsDeprecations(notices []UserDefaultsDeprecationNotice) {
	for _, n := range notices {
		switch {
		case n.Detail != "" && n.UseInstead != "":
			logger.Warningf("%s %s use %s — %s", deprecatedUserDefaultsLogPrefix, n.DeprecatedPath, n.UseInstead, n.Detail)
		case n.UseInstead != "":
			logger.Warningf("%s %s use %s", deprecatedUserDefaultsLogPrefix, n.DeprecatedPath, n.UseInstead)
		default:
			logger.Warningf("%s %s", deprecatedUserDefaultsLogPrefix, n.DeprecatedPath)
		}
	}
}

func appendDedupeNotices(slice []UserDefaultsDeprecationNotice, seen map[string]struct{}, add ...UserDefaultsDeprecationNotice) []UserDefaultsDeprecationNotice {
	for _, n := range add {
		if n.DeprecatedPath == "" {
			continue
		}
		k := n.noticeKey()
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		slice = append(slice, n)
	}
	return slice
}

func normalizeExtensionsListValue(v interface{}) interface{} {
	s, ok := v.(string)
	if !ok {
		return v
	}
	s = strings.ReplaceAll(s, ",", " ")
	s = strings.TrimSpace(s)
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}

func asStringMap(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}
	raw, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	return raw
}

func pullSubsection(ud map[string]interface{}, key string) map[string]interface{} {
	raw, ok := ud[key]
	if !ok {
		return nil
	}
	m := asStringMap(raw)
	if m == nil {
		return nil
	}
	delete(ud, key)
	return m
}

func overlayShallow(dst, src map[string]interface{}) {
	for k, v := range src {
		dst[k] = v
	}
}

func mustPreview(ud map[string]interface{}) map[string]interface{} {
	pm := asStringMap(ud["preview"])
	if pm == nil {
		pm = make(map[string]interface{})
		ud["preview"] = pm
	}
	return pm
}

func mergePreviewInto(ud map[string]interface{}, kv map[string]interface{}) {
	pm := mustPreview(ud)
	overlayShallow(pm, kv)
}

// NormalizeUserDefaultsMap expands Profile-aligned YAML groups (and sketch aliases)
// into the shape expected by [UserDefaults]. Subsection wins over legacy flat scalars when both exist.
func NormalizeUserDefaultsMap(ud map[string]interface{}) []UserDefaultsDeprecationNotice {
	if len(ud) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	notices := make([]UserDefaultsDeprecationNotice, 0, 32)

	rootMigratable := []string{
		"editorQuickSave", "hideSidebarFileActions", "disableQuickToggles", "disableSearchOptions",
		"stickySidebar", "hideFilesInTree", "darkMode", "locale", "viewMode", "singleClick",
		"showHidden", "hideFileExt", "dateFormat", "gallerySize", "themeColor", "quickDownload",
		"disablePreviewExt", "disableViewingExt", "lockPassword", "disableSettings",
		"loginMethod", "disableUpdateNotifications", "deleteWithoutConfirming", "deleteAfterArchive",
		"disableOfficePreviewExt", "disableOnlyOfficeExt", "customTheme", "showSelectMultiple",
		"showToolsInSidebar", "debugOffice", "preferEditorForMarkdown", "showCopyPath",
		"permissions",
	}
	hadMigratableFlat := map[string]struct{}{}
	for _, k := range rootMigratable {
		if _, ok := ud[k]; ok {
			hadMigratableFlat[k] = struct{}{}
		}
	}
	previewHadBefore := func(key string) bool {
		pm := asStringMap(ud["preview"])
		if pm == nil {
			return false
		}
		_, ok := pm[key]
		return ok
	}
	prevDisableHide := previewHadBefore("disableHideSidebar")
	prevHQ := previewHadBefore("highQuality")

	groupedRootKeys := map[string]struct{}{
		"listing": {}, "listingOptions": {}, "thumbnail": {}, "thumbnailOptions": {},
		"sidebar": {}, "sidebarOptions": {}, "search": {}, "searchOptions": {},
		"fileViewer": {}, "fileViewerOptions": {}, "ui": {}, "themeLanguage": {},
		"account": {}, "admin": {},
	}
	hadGroupedSubsectionInitially := false
	for k := range ud {
		if _, ok := groupedRootKeys[k]; ok {
			hadGroupedSubsectionInitially = true
			break
		}
	}

	// Pull combined maps: aliases merged first then canonical key wins overlays.
	combineSections := func(legacyFirst, canonical string, preferPath string) map[string]interface{} {
		combined := make(map[string]interface{})
		if legacyFirst != canonical {
			if sub := pullSubsection(ud, legacyFirst); len(sub) > 0 {
				notices = appendDedupeNotices(notices, seen, UserDefaultsDeprecationNotice{
					DeprecatedPath: fmt.Sprintf("userDefaults subsection %q", legacyFirst),
					UseInstead:     preferPath,
				})
				overlayShallow(combined, sub)
			}
		}
		if sub := pullSubsection(ud, canonical); len(sub) > 0 {
			overlayShallow(combined, sub)
		}
		return combined
	}

	listingOpts := combineSections("listing", "listingOptions", "userDefaults.listingOptions")
	thumbnailOpts := combineSections("thumbnail", "thumbnailOptions", "userDefaults.thumbnailOptions")
	sidebarOpts := combineSections("sidebar", "sidebarOptions", "userDefaults.sidebarOptions")
	searchOpts := combineSections("search", "searchOptions", "userDefaults.searchOptions")
	fileViewerOpts := combineSections("fileViewer", "fileViewerOptions", "userDefaults.fileViewerOptions")
	themeLangOpts := combineSections("ui", "themeLanguage", "userDefaults.themeLanguage")
	accountOpts := pullSubsection(ud, "account")
	adminOpts := pullSubsection(ud, "admin")

	if _, ok := searchOpts["disableOptions"]; ok && searchOpts != nil {
		searchOpts["disableSearchOptions"] = searchOpts["disableOptions"]
		delete(searchOpts, "disableOptions")
		notices = appendDedupeNotices(notices, seen, UserDefaultsDeprecationNotice{
			DeprecatedPath: "userDefaults.searchOptions.disableOptions",
			UseInstead:     "userDefaults.searchOptions.disableSearchOptions",
		})
	}

	if _, ok := sidebarOpts["disableHideSidebarOnPreview"]; ok {
		sidebarOpts["disableHideSidebar"] = sidebarOpts["disableHideSidebarOnPreview"]
		delete(sidebarOpts, "disableHideSidebarOnPreview")
		notices = appendDedupeNotices(notices, seen, UserDefaultsDeprecationNotice{
			DeprecatedPath: "userDefaults.sidebarOptions.disableHideSidebarOnPreview",
			UseInstead:     "userDefaults.sidebarOptions.disableHideSidebar",
			Detail:         "persisted internally as preview.disableHideSidebar",
		})
	}

	setFlat := func(flatKey string, subsectionPathWithField string, val interface{}) {
		_ = subsectionPathWithField
		ud[flatKey] = val
	}

	listingRoots := []string{
		"deleteWithoutConfirming", "dateFormat", "showHidden", "quickDownload",
		"showSelectMultiple", "deleteAfterArchive", "showCopyPath", "hideFileExt",
		"viewMode", "singleClick", "gallerySize",
	}
	for _, k := range listingRoots {
		if v, ok := listingOpts[k]; ok {
			setFlat(k, "userDefaults.listingOptions."+k, v)
		}
	}

	thumbnailPreviewFrag := func() map[string]interface{} {
		out := map[string]interface{}{}
		thKeys := []string{
			"image", "video", "audio", "motionVideoPreview", "office", "popup", "folder", "models", "highQuality",
		}
		if nested := asStringMap(thumbnailOpts["preview"]); nested != nil {
			for _, k := range thKeys {
				if v, ok := nested[k]; ok {
					out[k] = v
				}
			}
		}
		for _, k := range thKeys {
			if v, ok := thumbnailOpts[k]; ok {
				out[k] = v
			}
		}
		return out
	}
	if v, ok := thumbnailOpts["disablePreviewExt"]; ok {
		nv := normalizeExtensionsListValue(v)
		setFlat("disablePreviewExt", "userDefaults.thumbnailOptions.disablePreviewExt", nv)
	}
	if frag := thumbnailPreviewFrag(); len(frag) > 0 {
		mergePreviewInto(ud, frag)
	}

	sidebarRoots := []string{
		"disableQuickToggles", "hideSidebarFileActions", "hideFilesInTree", "showToolsInSidebar", "stickySidebar",
	}
	for _, k := range sidebarRoots {
		if v, ok := sidebarOpts[k]; ok {
			setFlat(k, "userDefaults.sidebarOptions."+k, v)
		}
	}
	if v, ok := sidebarOpts["disableHideSidebar"]; ok {
		mergePreviewInto(ud, map[string]interface{}{"disableHideSidebar": v})
	}

	if v, ok := searchOpts["disableSearchOptions"]; ok {
		k := "disableSearchOptions"
		setFlat(k, "userDefaults.searchOptions."+k, v)
	}

	fvPreview := asStringMap(fileViewerOpts["preview"])
	if fvPreview != nil {
		if v, ok := fvPreview["autoplayMedia"]; ok {
			mergePreviewInto(ud, map[string]interface{}{"autoplayMedia": v})
		}
		if v, ok := fvPreview["defaultMediaPlayer"]; ok {
			mergePreviewInto(ud, map[string]interface{}{"defaultMediaPlayer": v})
		}
	}
	fvRoots := []string{
		"editorQuickSave", "preferEditorForMarkdown", "disableViewingExt", "disableOnlyOfficeExt",
		"debugOffice", "disableOfficePreviewExt",
	}
	for _, k := range fvRoots {
		if v, ok := fileViewerOpts[k]; ok {
			nv := v
			switch k {
			case "disableViewingExt", "disableOnlyOfficeExt", "disableOfficePreviewExt":
				nv = normalizeExtensionsListValue(v)
			default:
			}
			setFlat(k, "userDefaults.fileViewerOptions."+k, nv)
			if k == "disableOfficePreviewExt" {
				str, ok2 := nv.(string)
				if ok2 && strings.TrimSpace(str) != "" {
					notices = appendDedupeNotices(notices, seen, UserDefaultsDeprecationNotice{
						DeprecatedPath: "userDefaults.disableOfficePreviewExt",
						UseInstead:     "userDefaults.thumbnailOptions.disablePreviewExt",
						Detail:         "disableOfficePreviewExt is superseded",
					})
				}
			}
		}
	}
	if v, ok := fileViewerOpts["autoplayMedia"]; ok {
		mergePreviewInto(ud, map[string]interface{}{"autoplayMedia": v})
	}
	if v, ok := fileViewerOpts["defaultMediaPlayer"]; ok {
		mergePreviewInto(ud, map[string]interface{}{"defaultMediaPlayer": v})
	}

	for _, k := range []string{"themeColor", "customTheme", "locale", "darkMode"} {
		if v, ok := themeLangOpts[k]; ok {
			setFlat(k, "userDefaults.themeLanguage."+k, v)
		}
	}

	if len(accountOpts) > 0 {
		if pm := asStringMap(accountOpts["permissions"]); pm != nil {
			if ud["permissions"] != nil {
				notices = appendDedupeNotices(notices, seen, UserDefaultsDeprecationNotice{
					DeprecatedPath: "userDefaults.permissions",
					UseInstead:     "userDefaults.account.permissions",
				})
			}
			rootPerms := asStringMap(ud["permissions"])
			ud["permissions"] = mergePermissions(rootPerms, pm)
			delete(accountOpts, "permissions")
		}
		for _, k := range []string{"lockPassword", "disableSettings", "loginMethod"} {
			if v, ok := accountOpts[k]; ok {
				setFlat(k, "userDefaults.account."+k, v)
			}
		}
	}

	if len(adminOpts) > 0 {
		if v, ok := adminOpts["disableUpdateNotifications"]; ok {
			setFlat("disableUpdateNotifications", "userDefaults.admin.disableUpdateNotifications", v)
		}
	}

	ud["disablePreviewExt"] = normalizeExtensionsListValue(ud["disablePreviewExt"])
	ud["disableViewingExt"] = normalizeExtensionsListValue(ud["disableViewingExt"])
	ud["disableOnlyOfficeExt"] = normalizeExtensionsListValue(ud["disableOnlyOfficeExt"])
	ud["disableOfficePreviewExt"] = normalizeExtensionsListValue(ud["disableOfficePreviewExt"])
	sanitizeYAMLPreviewMap(ud)

	// Warn on legacy flat keys when mixing grouped subsections or for specific deprecations always.
	perFlatDeprecation := func(k string) {
		if strings.TrimSpace(k) == "" {
			return
		}
		if k == "disableOfficePreviewExt" {
			str, ok2 := ud[k].(string)
			if ok2 && strings.TrimSpace(str) != "" {
				notices = appendDedupeNotices(notices, seen, UserDefaultsDeprecationNotice{
					DeprecatedPath: "userDefaults." + k + " (flat YAML)",
					UseInstead:     "userDefaults.thumbnailOptions.disablePreviewExt",
				})
			}
			return
		}
		pref := flatMigrationTarget(k)
		if pref == "" {
			return
		}
		notices = appendDedupeNotices(notices, seen, UserDefaultsDeprecationNotice{
			DeprecatedPath: "userDefaults." + k + " (flat YAML)",
			UseInstead:     pref,
		})
	}

	if len(hadMigratableFlat) > 0 && !hadGroupedSubsectionInitially {
		notices = appendDedupeNotices(notices, seen, UserDefaultsDeprecationNotice{
			DeprecatedPath: "flat userDefaults keys at YAML root",
			UseInstead:     "Profile-aligned groups listingOptions / thumbnailOptions / sidebarOptions / searchOptions / fileViewerOptions / themeLanguage / account / admin",
			Detail:         "See generated config.generated.yaml reference",
		})
		perFlatDeprecation("disableOfficePreviewExt")
	} else if hadGroupedSubsectionInitially {
		for k := range hadMigratableFlat {
			perFlatDeprecation(k)
		}
	} else {
		perFlatDeprecation("disableOfficePreviewExt")
	}

	if prevDisableHide {
		notices = appendDedupeNotices(notices, seen, UserDefaultsDeprecationNotice{
			DeprecatedPath: "userDefaults.preview.disableHideSidebar",
			UseInstead:     "userDefaults.sidebarOptions.disableHideSidebar",
		})
	}
	if prevHQ {
		notices = appendDedupeNotices(notices, seen, UserDefaultsDeprecationNotice{
			DeprecatedPath: "userDefaults.preview.highQuality",
			UseInstead:     "omit (ignored; thumbnails are always high quality as of v1.3.0+)",
		})
	}

	return notices
}

func mergePermissions(root, overlay map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	if root != nil {
		for k, v := range root {
			out[k] = v
		}
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}

func sanitizeYAMLPreviewMap(ud map[string]interface{}) {
	pm := asStringMap(ud["preview"])
	if pm == nil {
		return
	}
	t := reflect.TypeOf(UserDefaultsPreview{})
	allowed := make(map[string]struct{})
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := strings.TrimSpace(strings.Split(f.Tag.Get("json"), ",")[0])
		if tag == "" || tag == "-" {
			continue
		}
		allowed[tag] = struct{}{}
	}
	for k := range pm {
		if _, ok := allowed[k]; !ok {
			delete(pm, k)
		}
	}
}

func flatMigrationTarget(field string) string {
	switch field {
	case "deleteWithoutConfirming", "dateFormat", "showHidden", "quickDownload", "showSelectMultiple",
		"deleteAfterArchive", "showCopyPath", "hideFileExt", "viewMode", "singleClick", "gallerySize":
		return "userDefaults.listingOptions." + field
	case "disablePreviewExt":
		return "userDefaults.thumbnailOptions.disablePreviewExt"
	case "stickySidebar", "disableQuickToggles", "hideSidebarFileActions", "hideFilesInTree", "showToolsInSidebar":
		return "userDefaults.sidebarOptions." + field
	case "disableSearchOptions":
		return "userDefaults.searchOptions.disableSearchOptions"
	case "editorQuickSave", "preferEditorForMarkdown", "disableViewingExt", "disableOnlyOfficeExt", "debugOffice":
		return "userDefaults.fileViewerOptions." + field
	case "themeColor", "customTheme", "locale", "darkMode":
		return "userDefaults.themeLanguage." + field
	case "permissions", "lockPassword", "disableSettings", "loginMethod":
		return "userDefaults.account." + field
	case "disableUpdateNotifications":
		return "userDefaults.admin.disableUpdateNotifications"
	default:
		return ""
	}
}
