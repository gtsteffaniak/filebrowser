package users

import (
	"encoding/json"
	"strings"
)

// GlobalPermissionsFieldMask records which global permission JSON keys were present
// on a create-user request (admin, api, share, realtime). Omitted keys should receive
// userDefaults; present keys keep the caller's value, including explicit false.
type GlobalPermissionsFieldMask struct {
	Admin    bool
	Api      bool
	Share    bool
	Realtime bool
}

// Any reports whether at least one global permission key was explicitly present.
func (m GlobalPermissionsFieldMask) Any() bool {
	return m.Admin || m.Api || m.Share || m.Realtime
}

// GlobalPermissionsFieldMaskFromJSON builds a mask from a permissions object JSON value.
// A nil or JSON null value yields an empty mask (no keys explicitly set).
func GlobalPermissionsFieldMaskFromJSON(raw json.RawMessage) GlobalPermissionsFieldMask {
	if len(raw) == 0 || string(raw) == "null" {
		return GlobalPermissionsFieldMask{}
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return GlobalPermissionsFieldMask{}
	}
	var mask GlobalPermissionsFieldMask
	for key := range obj {
		switch strings.ToLower(key) {
		case "admin":
			mask.Admin = true
		case "api":
			mask.Api = true
		case "share":
			mask.Share = true
		case "realtime":
			mask.Realtime = true
		}
	}
	return mask
}

// ParseGlobalPermissionsFieldMaskFromCreateRequest extracts permission key presence from
// a POST /api/users body shaped as {"data": {..., "permissions": {...}}}.
func ParseGlobalPermissionsFieldMaskFromCreateRequest(body []byte) GlobalPermissionsFieldMask {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return GlobalPermissionsFieldMask{}
	}
	dataRaw, ok := top["data"]
	if !ok {
		return GlobalPermissionsFieldMask{}
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(dataRaw, &data); err != nil {
		return GlobalPermissionsFieldMask{}
	}
	permsRaw, ok := data["permissions"]
	if !ok {
		return GlobalPermissionsFieldMask{}
	}
	return GlobalPermissionsFieldMaskFromJSON(permsRaw)
}

// MergeGlobalPermissionsOnCreate combines defaults with caller-supplied global permissions.
// When mask is empty, any true global flag on requested preserves the full requested struct
// (programmatic create paths). Otherwise only masked fields override defaults.
func MergeGlobalPermissionsOnCreate(defaults, requested Permissions, mask GlobalPermissionsFieldMask) Permissions {
	if !mask.Any() {
		if requested.Admin || requested.Api || requested.Share || requested.Realtime {
			return requested
		}
		return defaults
	}
	out := defaults
	if mask.Admin {
		out.Admin = requested.Admin
	}
	if mask.Api {
		out.Api = requested.Api
	}
	if mask.Share {
		out.Share = requested.Share
	}
	if mask.Realtime {
		out.Realtime = requested.Realtime
	}
	return out
}
