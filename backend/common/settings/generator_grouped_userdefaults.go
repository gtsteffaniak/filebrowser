package settings

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

func fieldYAMLKeyFromStruct(ud reflect.Type, goField string) string {
	f, ok := ud.FieldByName(goField)
	if !ok {
		return strings.ToLower(goField)
	}
	if j := f.Tag.Get("json"); j != "" && j != "-" {
		return strings.Split(j, ",")[0]
	}
	return f.Name
}

func udField(ud reflect.Value, name string) reflect.Value {
	for ud.Kind() == reflect.Ptr && !ud.IsNil() {
		ud = ud.Elem()
	}
	return ud.FieldByName(name)
}

func udDefaultsField(defaults reflect.Value, name string) reflect.Value {
	for defaults.Kind() == reflect.Ptr && !defaults.IsNil() {
		defaults = defaults.Elem()
	}
	if !defaults.IsValid() || defaults.Type().Kind() != reflect.Struct {
		return reflect.Value{}
	}
	f := defaults.FieldByName(name)
	if !f.IsValid() {
		return reflect.Value{}
	}
	return f
}

// userDefaultsAppendScalarOrStruct adds one key to a YAML mapping if it differs from default (or has no default).
func userDefaultsAppendScalarOrStruct(
	mapNode *yaml.Node,
	ud, defUd reflect.Value,
	goField, yamlKey, commentType string,
	comm CommentsMap, secrets SecretFieldsMap, deprecated DeprecatedFieldsMap,
) error {
	if len(deprecated) > 0 && deprecated[commentType][goField] {
		return nil
	}
	cur := udField(ud, goField)
	defF := udDefaultsField(defUd, goField)
	if !cur.IsValid() {
		return nil
	}
	if defF.IsValid() && reflect.DeepEqual(cur.Interface(), defF.Interface()) {
		return nil
	}

	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: yamlKey}
	var parts []string
	if cm := comm[commentType][goField]; cm != "" {
		parts = append(parts, cm)
	}
	if len(comm) > 0 {
		if f, ok := ud.Type().FieldByName(goField); ok {
			if vt := f.Tag.Get("validate"); vt != "" {
				parts = append(parts, fmt.Sprintf(" validate:%s", vt))
			}
		}
	}
	if len(parts) > 0 {
		keyNode.LineComment = strings.Join(parts, " ")
	}

	var valNode *yaml.Node
	var err error
	generateConfig := os.Getenv("FILEBROWSER_GENERATE_CONFIG") == "true"
	if secrets[commentType][goField] && !generateConfig {
		fieldValue := cur.Interface()
		if str, ok := fieldValue.(string); ok && str == "" {
			valNode = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "", Style: yaml.DoubleQuotedStyle}
		} else {
			valNode = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "**hidden**", Style: yaml.DoubleQuotedStyle}
		}
	} else {
		valNode, err = buildNodeWithDefaults(cur, comm, defF, secrets, deprecated)
		if err != nil {
			return err
		}
	}
	mapNode.Content = append(mapNode.Content, keyNode, valNode)
	return nil
}

func appendSubsectionIfNonEmpty(root *yaml.Node, sectionKey string, sub *yaml.Node, lineComment string, comm CommentsMap) {
	if sub == nil || len(sub.Content) == 0 {
		return
	}
	kn := &yaml.Node{Kind: yaml.ScalarNode, Value: sectionKey}
	if lineComment != "" && len(comm) > 0 {
		kn.LineComment = lineComment
	}
	root.Content = append(root.Content, kn, sub)
}

// buildGroupedUserDefaultsYAML emits Profile.vue–aligned userDefaults groups for generated reference YAML.
func buildGroupedUserDefaultsYAML(v reflect.Value, defaults reflect.Value, comm CommentsMap, secrets SecretFieldsMap, deprecated DeprecatedFieldsMap) (*yaml.Node, error) {
	udT := reflect.TypeOf(UserDefaults{})
	for v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}
	for defaults.Kind() == reflect.Ptr && !defaults.IsNil() {
		defaults = defaults.Elem()
	}

	root := &yaml.Node{Kind: yaml.MappingNode}
	ud := v

	var defUd reflect.Value
	if defaults.IsValid() && defaults.Type() == udT {
		defUd = defaults
	}

	listing := &yaml.Node{Kind: yaml.MappingNode}
	listingFields := []string{
		"DeleteWithoutConfirming", "DateFormat", "ShowHidden", "QuickDownload", "ShowSelectMultiple",
		"DeleteAfterArchive", "ShowCopyPath", "HideFileExt", "ViewMode", "SingleClick", "GallerySize",
	}
	for _, fn := range listingFields {
		yk := fieldYAMLKeyFromStruct(udT, fn)
		if err := userDefaultsAppendScalarOrStruct(listing, ud, defUd, fn, yk, "UserDefaults", comm, secrets, deprecated); err != nil {
			return nil, err
		}
	}
	appendSubsectionIfNonEmpty(root, "listingOptions", listing, "Profile: listing options", comm)

	thumb := &yaml.Node{Kind: yaml.MappingNode}
	if err := userDefaultsAppendScalarOrStruct(thumb, ud, defUd, "DisablePreviewExt", "disablePreviewExt", "UserDefaults", comm, secrets, deprecated); err != nil {
		return nil, err
	}
	previewT := reflect.TypeOf(UserDefaultsPreview{})
	pCur := udField(ud, "Preview")
	pDef := udDefaultsField(defUd, "Preview")
	thumbPreviewFields := []string{
		"Image", "Video", "Audio", "MotionVideoPreview", "Office", "PopUp", "Folder", "Models",
	}
	for _, fn := range thumbPreviewFields {
		yk := fieldYAMLKeyFromStruct(previewT, fn)
		if len(deprecated) > 0 && deprecated["UserDefaultsPreview"][fn] {
			continue
		}
		subCur := udField(pCur, fn)
		subDef := udDefaultsField(pDef, fn)
		if !subCur.IsValid() {
			continue
		}
		if subCur.Kind() == reflect.Ptr && subCur.IsNil() {
			continue
		}
		if subDef.IsValid() && reflect.DeepEqual(subCur.Interface(), subDef.Interface()) {
			continue
		}
		kn := &yaml.Node{Kind: yaml.ScalarNode, Value: yk}
		if cm := comm["UserDefaultsPreview"][fn]; cm != "" {
			kn.LineComment = cm
		}
		vn, err := buildNodeWithDefaults(subCur, comm, subDef, secrets, deprecated)
		if err != nil {
			return nil, err
		}
		thumb.Content = append(thumb.Content, kn, vn)
	}
	appendSubsectionIfNonEmpty(root, "thumbnailOptions", thumb, "Profile: thumbnail options", comm)

	side := &yaml.Node{Kind: yaml.MappingNode}
	sidebarRoots := []string{
		"DisableQuickToggles", "HideSidebarFileActions", "HideFilesInTree", "ShowToolsInSidebar", "StickySidebar",
	}
	for _, fn := range sidebarRoots {
		yk := fieldYAMLKeyFromStruct(udT, fn)
		if err := userDefaultsAppendScalarOrStruct(side, ud, defUd, fn, yk, "UserDefaults", comm, secrets, deprecated); err != nil {
			return nil, err
		}
	}
	dsCur := udField(pCur, "DisableHideSidebar")
	dsDef := udDefaultsField(pDef, "DisableHideSidebar")
	if dsCur.IsValid() {
		equal := dsDef.IsValid() && reflect.DeepEqual(dsCur.Interface(), dsDef.Interface())
		if !equal {
			kn := &yaml.Node{Kind: yaml.ScalarNode, Value: "disableHideSidebar"}
			if cm := comm["UserDefaultsPreview"]["DisableHideSidebar"]; cm != "" {
				kn.LineComment = cm + " — Profile: sidebarOptions"
			}
			vn, err := buildNodeWithDefaults(dsCur, comm, dsDef, secrets, deprecated)
			if err != nil {
				return nil, err
			}
			side.Content = append(side.Content, kn, vn)
		}
	}
	appendSubsectionIfNonEmpty(root, "sidebarOptions", side, "Profile: sidebar options", comm)

	search := &yaml.Node{Kind: yaml.MappingNode}
	if err := userDefaultsAppendScalarOrStruct(search, ud, defUd, "DisableSearchOptions", "disableSearchOptions", "UserDefaults", comm, secrets, deprecated); err != nil {
		return nil, err
	}
	appendSubsectionIfNonEmpty(root, "searchOptions", search, "Profile: search options", comm)

	fv := &yaml.Node{Kind: yaml.MappingNode}
	fileViewerUd := []string{"EditorQuickSave", "PreferEditorForMarkdown", "DisableViewingExt", "DisableOnlyOfficeExt", "DebugOffice"}
	for _, fn := range fileViewerUd {
		yk := fieldYAMLKeyFromStruct(udT, fn)
		if err := userDefaultsAppendScalarOrStruct(fv, ud, defUd, fn, yk, "UserDefaults", comm, secrets, deprecated); err != nil {
			return nil, err
		}
	}
	if err := userDefaultsAppendScalarOrStruct(fv, ud, defUd, "DisableOfficePreviewExt", "disableOfficePreviewExt", "UserDefaults", comm, secrets, deprecated); err != nil {
		return nil, err
	}
	for _, fn := range []string{"DefaultMediaPlayer"} {
		yk := fieldYAMLKeyFromStruct(previewT, fn)
		subCur := udField(pCur, fn)
		subDef := udDefaultsField(pDef, fn)
		if !subCur.IsValid() {
			continue
		}
		if subDef.IsValid() && reflect.DeepEqual(subCur.Interface(), subDef.Interface()) {
			continue
		}
		kn := &yaml.Node{Kind: yaml.ScalarNode, Value: yk}
		if cm := comm["UserDefaultsPreview"][fn]; cm != "" {
			kn.LineComment = cm
		}
		vn, err := buildNodeWithDefaults(subCur, comm, subDef, secrets, deprecated)
		if err != nil {
			return nil, err
		}
		fv.Content = append(fv.Content, kn, vn)
	}
	subCur := udField(pCur, "AutoplayMedia")
	subDef := udDefaultsField(pDef, "AutoplayMedia")
	if subCur.IsValid() && !(subCur.Kind() == reflect.Ptr && subCur.IsNil()) {
		if !(subDef.IsValid() && reflect.DeepEqual(subCur.Interface(), subDef.Interface())) {
			yk := fieldYAMLKeyFromStruct(previewT, "AutoplayMedia")
			kn := &yaml.Node{Kind: yaml.ScalarNode, Value: yk}
			if cm := comm["UserDefaultsPreview"]["AutoplayMedia"]; cm != "" {
				kn.LineComment = cm
			}
			vn, err := buildNodeWithDefaults(subCur, comm, subDef, secrets, deprecated)
			if err != nil {
				return nil, err
			}
			fv.Content = append(fv.Content, kn, vn)
		}
	}
	appendSubsectionIfNonEmpty(root, "fileViewerOptions", fv, "Profile: file viewer options", comm)

	theme := &yaml.Node{Kind: yaml.MappingNode}
	for _, fn := range []string{"ThemeColor", "CustomTheme", "Locale", "DarkMode"} {
		yk := fieldYAMLKeyFromStruct(udT, fn)
		if err := userDefaultsAppendScalarOrStruct(theme, ud, defUd, fn, yk, "UserDefaults", comm, secrets, deprecated); err != nil {
			return nil, err
		}
	}
	appendSubsectionIfNonEmpty(root, "themeLanguage", theme, "Profile: theme and language", comm)

	acct := &yaml.Node{Kind: yaml.MappingNode}
	permsCur := udField(ud, "Permissions")
	permsDef := udDefaultsField(defUd, "Permissions")
	if permsCur.IsValid() {
		if !(permsDef.IsValid() && reflect.DeepEqual(permsCur.Interface(), permsDef.Interface())) {
			kn := &yaml.Node{Kind: yaml.ScalarNode, Value: "permissions"}
			if cm := comm["UserDefaults"]["Permissions"]; cm != "" {
				kn.LineComment = cm
			}
			vn, err := buildNodeWithDefaults(permsCur, comm, permsDef, secrets, deprecated)
			if err != nil {
				return nil, err
			}
			acct.Content = append(acct.Content, kn, vn)
		}
	}
	for _, fn := range []string{"LockPassword", "DisableSettings", "LoginMethod"} {
		yk := fieldYAMLKeyFromStruct(udT, fn)
		if err := userDefaultsAppendScalarOrStruct(acct, ud, defUd, fn, yk, "UserDefaults", comm, secrets, deprecated); err != nil {
			return nil, err
		}
	}
	appendSubsectionIfNonEmpty(root, "account", acct, "Server defaults: new user account", comm)

	adm := &yaml.Node{Kind: yaml.MappingNode}
	if err := userDefaultsAppendScalarOrStruct(adm, ud, defUd, "DisableUpdateNotifications", "disableUpdateNotifications", "UserDefaults", comm, secrets, deprecated); err != nil {
		return nil, err
	}
	appendSubsectionIfNonEmpty(root, "admin", adm, "Server defaults: admin banner", comm)

	flCur := udField(ud, "FileLoading")
	flDef := udDefaultsField(defUd, "FileLoading")
	if flCur.IsValid() {
		if !(flDef.IsValid() && reflect.DeepEqual(flCur.Interface(), flDef.Interface())) {
			kn := &yaml.Node{Kind: yaml.ScalarNode, Value: "fileLoading"}
			if cm := comm["UserDefaults"]["FileLoading"]; cm != "" && len(comm) > 0 {
				kn.LineComment = cm
			}
			vn, err := buildNodeWithDefaults(flCur, comm, flDef, secrets, deprecated)
			if err != nil {
				return nil, err
			}
			root.Content = append(root.Content, kn, vn)
		}
	}

	return root, nil
}
