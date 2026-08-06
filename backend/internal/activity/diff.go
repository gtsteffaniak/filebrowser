package activity

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/usersidebar"
)

var userActivitySkipJSONTags = normalizedSkipTags(
	"id",
	"password",
	"totpSecret",
	"totpNonce",
	"tokens",
	"apiKeys",
	"backendScopes",
	"perm",
	"passkeyCredentials",
	"version",
	"pinnedItems",
)

var activitySkipStructJSONTags = normalizedSkipTags(
	"configured",
)

var shareActivitySkipJSONTags = normalizedSkipTags(
	"hash",
	"password_hash",
	"token",
	"userID",
	"userDownloads",
	"version",
	"sourcePath",
	"pinnedItems",
	"pathExists",
	"downloads",
	"username",
	"downloadURL",
	"shareURL",
	"faviconUrl",
	"bannerUrl",
	"sourceURL",
	"canEditShare",
	"hasPassword",
)

func normalizedSkipTags(tags ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		out[strings.ToLower(strings.TrimSpace(t))] = struct{}{}
	}
	return out
}

func UserUpdateChanges(before, after *users.User, which []string, passwordChanged bool) []activitydb.FieldChange {
	if before == nil || after == nil {
		return nil
	}
	fieldNames := normalizeUserWhich(which)
	changes := make([]activitydb.FieldChange, 0, len(fieldNames))
	for _, jsonTag := range fieldNames {
		tag := strings.TrimSpace(jsonTag)
		if tag == "" {
			continue
		}
		if strings.EqualFold(tag, "password") {
			if passwordChanged {
				changes = append(changes, activitydb.FieldChange{
					Field: "password",
					From:  "[redacted]",
					To:    "[changed]",
				})
			}
			continue
		}
		if _, skip := userActivitySkipJSONTags[strings.ToLower(tag)]; skip {
			continue
		}
		if strings.EqualFold(tag, "scopes") {
			changes = append(changes, scopeFieldChanges(before, after)...)
			continue
		}
		if strings.EqualFold(tag, "backendSourcePermissions") {
			changes = append(changes, backendSourcePermissionsFieldChanges(before, after)...)
			continue
		}
		if strings.EqualFold(tag, "sidebarLinks") {
			if change, ok := sidebarLinksFieldChange(before, after); ok {
				changes = append(changes, change)
			}
			continue
		}
		changes = append(changes, structFieldChanges(reflect.ValueOf(before).Elem(), reflect.ValueOf(after).Elem(), reflect.TypeOf(users.User{}), tag)...)
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Field < changes[j].Field })
	return changes
}

func ShareUpdateChanges(before, after *share.Share) []activitydb.FieldChange {
	if before == nil || after == nil {
		return nil
	}
	tags := collectJSONTags(reflect.TypeOf(share.Share{}), shareActivitySkipJSONTags)
	changes := make([]activitydb.FieldChange, 0, len(tags))
	for _, tag := range tags {
		changes = append(changes, structFieldChanges(reflect.ValueOf(before).Elem(), reflect.ValueOf(after).Elem(), reflect.TypeOf(share.Share{}), tag)...)
	}
	if before.HasPassword() != after.HasPassword() {
		changes = append(changes, activitydb.FieldChange{
			Field: "hasPassword",
			From:  formatActivityValue(reflect.ValueOf(before.HasPassword())),
			To:    formatActivityValue(reflect.ValueOf(after.HasPassword())),
		})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Field < changes[j].Field })
	return changes
}

func normalizeUserWhich(which []string) []string {
	return which
}

func scopeFieldChanges(before, after *users.User) []activitydb.FieldChange {
	return frontendScopeListChanges(before.GetFrontendScopes(), after.GetFrontendScopes(), "scopes")
}

func frontendScopeListChanges(before, after []users.FrontendScope, prefix string) []activitydb.FieldChange {
	beforeByName := make(map[string]users.FrontendScope, len(before))
	for _, scope := range before {
		beforeByName[scope.Name] = scope
	}
	afterByName := make(map[string]users.FrontendScope, len(after))
	for _, scope := range after {
		afterByName[scope.Name] = scope
	}
	names := sortedMapKeys(beforeByName, afterByName)

	changes := make([]activitydb.FieldChange, 0, len(names))
	for _, name := range names {
		bScope, bOk := beforeByName[name]
		aScope, aOk := afterByName[name]
		subPrefix := prefix + "." + name
		if !bOk {
			changes = append(changes, valueFieldChanges(reflect.Value{}, reflect.ValueOf(aScope), subPrefix)...)
			continue
		}
		if !aOk {
			changes = append(changes, valueFieldChanges(reflect.ValueOf(bScope), reflect.Value{}, subPrefix)...)
			continue
		}
		if bScope.Scope != aScope.Scope {
			changes = append(changes, activitydb.FieldChange{
				Field: subPrefix + ".scope",
				From:  bScope.Scope,
				To:    aScope.Scope,
			})
		}
		changes = append(changes, valueFieldChanges(
			reflect.ValueOf(bScope.Permissions),
			reflect.ValueOf(aScope.Permissions),
			subPrefix+".permissions",
		)...)
	}
	return changes
}

func backendSourcePermissionsFieldChanges(before, after *users.User) []activitydb.FieldChange {
	if users.SourceConfigLoaded() {
		return sourcePermissionsMapChanges(
			before.GetFrontendSourcePermissions(),
			after.GetFrontendSourcePermissions(),
			"backendSourcePermissions",
		)
	}
	return sourcePermissionsMapChanges(
		before.BackendSourcePermissions,
		after.BackendSourcePermissions,
		"backendSourcePermissions",
	)
}

func sourcePermissionsMapChanges(before, after map[string]users.SourceFilePermissions, prefix string) []activitydb.FieldChange {
	if len(before) == 0 && len(after) == 0 {
		return nil
	}
	names := sortedMapKeys(before, after)

	changes := make([]activitydb.FieldChange, 0, len(names))
	for _, name := range names {
		bPerms, bOk := before[name]
		aPerms, aOk := after[name]
		subPrefix := prefix + "." + name
		if !bOk {
			changes = append(changes, valueFieldChanges(reflect.Value{}, reflect.ValueOf(aPerms), subPrefix)...)
			continue
		}
		if !aOk {
			changes = append(changes, valueFieldChanges(reflect.ValueOf(bPerms), reflect.Value{}, subPrefix)...)
			continue
		}
		changes = append(changes, valueFieldChanges(reflect.ValueOf(bPerms), reflect.ValueOf(aPerms), subPrefix)...)
	}
	return changes
}

func sortedMapKeys[K comparable, V any](before, after map[K]V) []K {
	seen := make(map[K]struct{}, len(before)+len(after))
	for key := range before {
		seen[key] = struct{}{}
	}
	for key := range after {
		seen[key] = struct{}{}
	}
	keys := make([]K, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprintf("%v", keys[i]) < fmt.Sprintf("%v", keys[j])
	})
	return keys
}

func sidebarLinksFieldChange(before, after *users.User) (activitydb.FieldChange, bool) {
	from := formatActivityValue(reflect.ValueOf(usersidebar.FrontendLinks(before.SidebarLinks, before.ShowToolsInSidebar)))
	to := formatActivityValue(reflect.ValueOf(usersidebar.FrontendLinks(after.SidebarLinks, after.ShowToolsInSidebar)))
	if from == to {
		return activitydb.FieldChange{}, false
	}
	return activitydb.FieldChange{Field: "sidebarLinks", From: from, To: to}, true
}

func structFieldChanges(beforeVal, afterVal reflect.Value, rootType reflect.Type, jsonTag string) []activitydb.FieldChange {
	fieldIndex, ok := fieldIndexByJSONTag(rootType, jsonTag)
	if !ok {
		return nil
	}
	beforeField := beforeVal.FieldByIndex(fieldIndex)
	afterField := afterVal.FieldByIndex(fieldIndex)
	return valueFieldChanges(beforeField, afterField, jsonTag)
}

func valueFieldChanges(before, after reflect.Value, fieldPrefix string) []activitydb.FieldChange {
	fromVal, toVal := before, after
	before = reflect.Indirect(before)
	after = reflect.Indirect(after)
	if !before.IsValid() && !after.IsValid() {
		return nil
	}
	if !before.IsValid() || !after.IsValid() {
		return []activitydb.FieldChange{{
			Field: fieldPrefix,
			From:  formatActivityValue(fromVal),
			To:    formatActivityValue(toVal),
		}}
	}
	if reflect.DeepEqual(before.Interface(), after.Interface()) {
		return nil
	}
	if before.Kind() == reflect.Map && after.Kind() == reflect.Map && before.Type().Key().Kind() == reflect.String {
		return mapFieldChanges(before, after, fieldPrefix)
	}
	if before.Kind() == reflect.Struct && after.Kind() == reflect.Struct {
		changes := make([]activitydb.FieldChange, 0, before.Type().NumField())
		for i := 0; i < before.Type().NumField(); i++ {
			sf := before.Type().Field(i)
			if sf.PkgPath != "" {
				continue
			}
			tagName := strings.Split(sf.Tag.Get("json"), ",")[0]
			if tagName == "" || tagName == "-" {
				continue
			}
			if _, skip := activitySkipStructJSONTags[strings.ToLower(tagName)]; skip {
				continue
			}
			subPrefix := tagName
			if fieldPrefix != "" {
				subPrefix = fieldPrefix + "." + tagName
			}
			changes = append(changes, valueFieldChanges(before.Field(i), after.Field(i), subPrefix)...)
		}
		if len(changes) > 0 {
			return changes
		}
	}
	return []activitydb.FieldChange{{
		Field: fieldPrefix,
		From:  formatActivityValue(before),
		To:    formatActivityValue(after),
	}}
}

func mapFieldChanges(before, after reflect.Value, fieldPrefix string) []activitydb.FieldChange {
	if before.IsNil() && after.IsNil() {
		return nil
	}
	if before.IsNil() {
		before = reflect.MakeMap(after.Type())
	}
	if after.IsNil() {
		after = reflect.MakeMap(before.Type())
	}

	keys := make(map[string]reflect.Value)
	for _, key := range before.MapKeys() {
		keys[key.String()] = key
	}
	for _, key := range after.MapKeys() {
		keys[key.String()] = key
	}
	keyNames := make([]string, 0, len(keys))
	for keyName := range keys {
		keyNames = append(keyNames, keyName)
	}
	sort.Strings(keyNames)

	changes := make([]activitydb.FieldChange, 0, len(keyNames))
	for _, keyName := range keyNames {
		key := keys[keyName]
		bVal := before.MapIndex(key)
		aVal := after.MapIndex(key)
		subPrefix := fieldPrefix + "." + keyName
		if !bVal.IsValid() {
			changes = append(changes, valueFieldChanges(reflect.Value{}, aVal, subPrefix)...)
			continue
		}
		if !aVal.IsValid() {
			changes = append(changes, valueFieldChanges(bVal, reflect.Value{}, subPrefix)...)
			continue
		}
		changes = append(changes, valueFieldChanges(bVal, aVal, subPrefix)...)
	}
	return changes
}

func fieldIndexByJSONTag(t reflect.Type, jsonTag string) ([]int, bool) {
	target := strings.ToLower(strings.TrimSpace(jsonTag))
	if target == "" {
		return nil, false
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			if idx, ok := fieldIndexByJSONTag(field.Type, jsonTag); ok {
				return append([]int{i}, idx...), true
			}
			continue
		}
		tagName := strings.Split(field.Tag.Get("json"), ",")[0]
		if tagName == "" || tagName == "-" {
			continue
		}
		if strings.EqualFold(tagName, target) {
			return []int{i}, true
		}
	}
	return nil, false
}

func collectJSONTags(t reflect.Type, skip map[string]struct{}) []string {
	tags := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			tags = append(tags, collectJSONTags(field.Type, skip)...)
			continue
		}
		tagName := strings.Split(field.Tag.Get("json"), ",")[0]
		if tagName == "" || tagName == "-" {
			continue
		}
		if _, ok := skip[strings.ToLower(tagName)]; ok {
			continue
		}
		tags = append(tags, tagName)
	}
	sort.Strings(tags)
	return tags
}

func formatActivityValue(v reflect.Value) string {
	if !v.IsValid() {
		return ""
	}
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return "null"
	}
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Bool:
		return fmt.Sprintf("%t", v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", v.Uint())
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%g", v.Float())
	default:
		b, err := json.Marshal(v.Interface())
		if err != nil {
			return fmt.Sprintf("%v", v.Interface())
		}
		return string(b)
	}
}
