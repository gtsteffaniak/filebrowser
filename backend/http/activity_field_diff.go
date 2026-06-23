package http

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	activitydb "github.com/gtsteffaniak/filebrowser/backend/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

var userActivitySkipJSONTags = map[string]struct{}{
	"id":                 {},
	"password":           {},
	"totpSecret":         {},
	"totpNonce":          {},
	"tokens":             {},
	"apiKeys":            {},
	"backendScopes":      {},
	"perm":               {},
	"passkeyCredentials": {},
	"version":            {},
	"pinnedItems":        {},
}

var shareActivitySkipJSONTags = map[string]struct{}{
	"password_hash": {},
	"token":         {},
	"userID":        {},
	"userDownloads": {},
	"version":       {},
	"sourcePath":    {},
	"pinnedItems":   {},
	"pathExists":    {},
	"downloads":     {},
	"username":      {},
	"downloadURL":   {},
	"shareURL":      {},
	"faviconUrl":    {},
	"bannerUrl":     {},
	"sourceURL":     {},
	"canEditShare":  {},
	"hasPassword":   {},
}

func userUpdateChanges(before, after *users.User, which []string, passwordChanged bool) []activitydb.FieldChange {
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
			if change, ok := scopeFieldChange(before, after); ok {
				changes = append(changes, change)
			}
			continue
		}
		if strings.EqualFold(tag, "sidebarLinks") {
			if change, ok := sidebarLinksFieldChange(before, after); ok {
				changes = append(changes, change)
			}
			continue
		}
		if change, ok := structFieldChange(reflect.ValueOf(before).Elem(), reflect.ValueOf(after).Elem(), reflect.TypeOf(users.User{}), tag); ok {
			changes = append(changes, change)
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Field < changes[j].Field })
	return changes
}

func shareUpdateChanges(before, after *share.Share) []activitydb.FieldChange {
	if before == nil || after == nil {
		return nil
	}
	tags := collectJSONTags(reflect.TypeOf(share.Share{}), shareActivitySkipJSONTags)
	changes := make([]activitydb.FieldChange, 0, len(tags))
	for _, tag := range tags {
		if change, ok := structFieldChange(reflect.ValueOf(before).Elem(), reflect.ValueOf(after).Elem(), reflect.TypeOf(share.Share{}), tag); ok {
			changes = append(changes, change)
		}
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
	if len(which) == 0 {
		return collectJSONTags(reflect.TypeOf(users.User{}), userActivitySkipJSONTags)
	}
	if len(which) == 1 && strings.EqualFold(strings.TrimSpace(which[0]), "all") {
		return collectJSONTags(reflect.TypeOf(users.User{}), userActivitySkipJSONTags)
	}
	return which
}

func scopeFieldChange(before, after *users.User) (activitydb.FieldChange, bool) {
	from := formatActivityValue(reflect.ValueOf(before.GetFrontendScopes()))
	to := formatActivityValue(reflect.ValueOf(after.GetFrontendScopes()))
	if from == to {
		return activitydb.FieldChange{}, false
	}
	return activitydb.FieldChange{Field: "scopes", From: from, To: to}, true
}

func sidebarLinksFieldChange(before, after *users.User) (activitydb.FieldChange, bool) {
	from := formatActivityValue(reflect.ValueOf(GetFrontendSidebarLinks(before.SidebarLinks, before.ShowToolsInSidebar)))
	to := formatActivityValue(reflect.ValueOf(GetFrontendSidebarLinks(after.SidebarLinks, after.ShowToolsInSidebar)))
	if from == to {
		return activitydb.FieldChange{}, false
	}
	return activitydb.FieldChange{Field: "sidebarLinks", From: from, To: to}, true
}

func structFieldChange(beforeVal, afterVal reflect.Value, rootType reflect.Type, jsonTag string) (activitydb.FieldChange, bool) {
	fieldIndex, ok := fieldIndexByJSONTag(rootType, jsonTag)
	if !ok {
		return activitydb.FieldChange{}, false
	}
	beforeField := beforeVal.FieldByIndex(fieldIndex)
	afterField := afterVal.FieldByIndex(fieldIndex)
	if !beforeField.IsValid() || !afterField.IsValid() {
		return activitydb.FieldChange{}, false
	}
	if reflect.DeepEqual(beforeField.Interface(), afterField.Interface()) {
		return activitydb.FieldChange{}, false
	}
	return activitydb.FieldChange{
		Field: jsonTag,
		From:  formatActivityValue(beforeField),
		To:    formatActivityValue(afterField),
	}, true
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
