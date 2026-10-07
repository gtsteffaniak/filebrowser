package access

import (
	"fmt"
	"strings"
	"unicode"
)

// MaxGroupNameLength bounds names for manually created groups. Existing names
// (e.g. LDAP DNs synced from an IdP) may exceed it and are never rejected on read.
const MaxGroupNameLength = 128

// NormalizeGroupName trims and validates a group name supplied by an admin API.
// It is only used when manually creating or referencing a group; names that
// already exist in the database are always accepted on read, list, edit and
// delete paths so legacy records stay usable.
func NormalizeGroupName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("group name is required")
	}
	if len(name) > MaxGroupNameLength {
		return "", fmt.Errorf("group name exceeds %d characters", MaxGroupNameLength)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("group name must not contain control characters")
		}
	}
	return name, nil
}

// NormalizeMembers trims, dedupes and drops empty usernames while preserving order.
func NormalizeMembers(usernames []string) []string {
	seen := make(map[string]struct{}, len(usernames))
	out := make([]string, 0, len(usernames))
	for _, name := range usernames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

// IsUsableSyncedGroupName reports whether a group name coming from an identity
// provider can be stored. Only the minimum needed to keep rows usable is
// rejected; everything else is kept so IdP naming conventions keep working.
func IsUsableSyncedGroupName(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
