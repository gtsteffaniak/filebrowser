package sharedefaults

import (
	"encoding/json"
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

var sharePostBodyMetaKeys = map[string]struct{}{
	"hash":     {},
	"password": {},
	"path":     {},
}

// MergeEditableUpdate overlays a partial POST body onto existing share editable fields.
// Only keys present in patchJSON replace existing values.
func MergeEditableUpdate(existing share.ShareEditable, patchJSON []byte) (share.ShareEditable, error) {
	var patchMap map[string]json.RawMessage
	if err := json.Unmarshal(patchJSON, &patchMap); err != nil {
		return share.ShareEditable{}, fmt.Errorf("unmarshal share update patch: %w", err)
	}
	for key := range sharePostBodyMetaKeys {
		delete(patchMap, key)
	}
	if len(patchMap) == 0 {
		return existing, nil
	}
	trimmedPatch, err := json.Marshal(patchMap)
	if err != nil {
		return share.ShareEditable{}, fmt.Errorf("marshal share update patch: %w", err)
	}
	baseBytes, err := json.Marshal(existing)
	if err != nil {
		return share.ShareEditable{}, fmt.Errorf("marshal existing share editable: %w", err)
	}
	mergedBytes, err := settings.MergeUserDefaultsPatchJSONBytes(baseBytes, trimmedPatch)
	if err != nil {
		return share.ShareEditable{}, err
	}
	var merged share.ShareEditable
	if err := json.Unmarshal(mergedBytes, &merged); err != nil {
		return share.ShareEditable{}, fmt.Errorf("unmarshal merged share editable: %w", err)
	}
	return merged, nil
}
