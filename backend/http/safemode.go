package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"unicode"

	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

func hashSafePIN(pin string) string {
	h := sha256.Sum256([]byte(pin))
	return hex.EncodeToString(h[:])
}

func validateSafePIN(pin string) error {
	if len(pin) != 4 {
		return fmt.Errorf("PIN must be exactly 4 digits")
	}
	for _, c := range pin {
		if !unicode.IsDigit(c) {
			return fmt.Errorf("PIN must contain only digits")
		}
	}
	return nil
}

// safeModeGetHandler returns the current user's SAFEMode items and whether a PIN is set.
// GET /api/safemode
func safeModeGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	items := d.user.SafeModeItems
	if items == nil {
		items = []users.SafeModeItem{}
	}
	return renderJSON(w, r, map[string]interface{}{
		"items":  items,
		"hasPIN": d.user.SafeModePINHash != "",
	})
}

// safeModeAddHandler adds items to the user's SAFEMode.
// If the user has no PIN yet, the provided PIN becomes their PIN.
// If the user already has a PIN, the provided PIN must match.
// POST /api/safemode
// Body: { "items": [{"source":"...","path":"..."}], "pin": "1234" }
func safeModeAddHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	var req struct {
		Items []users.SafeModeItem `json:"items"`
		PIN   string               `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err)
	}
	if len(req.Items) == 0 {
		return http.StatusBadRequest, fmt.Errorf("no items provided")
	}
	if err := validateSafePIN(req.PIN); err != nil {
		return http.StatusBadRequest, err
	}

	if d.user.SafeModePINHash != "" {
		// PIN already set — verify it
		if hashSafePIN(req.PIN) != d.user.SafeModePINHash {
			return http.StatusForbidden, fmt.Errorf("incorrect PIN")
		}
	} else {
		// First SAFEMode item — this PIN becomes the user's PIN
		d.user.SafeModePINHash = hashSafePIN(req.PIN)
	}

	// Append items, skipping duplicates
	for _, newItem := range req.Items {
		duplicate := false
		for _, existing := range d.user.SafeModeItems {
			if existing.Source == newItem.Source && existing.Path == newItem.Path {
				duplicate = true
				break
			}
		}
		if !duplicate {
			d.user.SafeModeItems = append(d.user.SafeModeItems, newItem)
		}
	}

	if err := store.Users.Update(d.user, true, "SafeModeItems", "SafeModePINHash"); err != nil {
		logger.Errorf("safemode: failed to update user %s: %v", d.user.Username, err)
		return http.StatusInternalServerError, fmt.Errorf("failed to save SAFEMode: %w", err)
	}
	AcornStateSaveSafeMode(d.user.Username, d.user.SafeModePINHash, d.user.SafeModeItems)
	items := d.user.SafeModeItems
	if items == nil {
		items = []users.SafeModeItem{}
	}
	return renderJSON(w, r, map[string]interface{}{"items": items})
}

// safeModeRemoveHandler removes items from the user's SAFEMode. Requires PIN.
// DELETE /api/safemode
// Body: { "items": [{"source":"...","path":"..."}], "pin": "1234" }
func safeModeRemoveHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	var req struct {
		Items []users.SafeModeItem `json:"items"`
		PIN   string               `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err)
	}
	if len(req.Items) == 0 {
		return http.StatusBadRequest, fmt.Errorf("no items provided")
	}
	if d.user.SafeModePINHash == "" {
		return http.StatusBadRequest, fmt.Errorf("no SAFEMode PIN set")
	}
	if hashSafePIN(req.PIN) != d.user.SafeModePINHash {
		return http.StatusForbidden, fmt.Errorf("incorrect PIN")
	}

	var remaining []users.SafeModeItem
	for _, existing := range d.user.SafeModeItems {
		keep := true
		for _, toRemove := range req.Items {
			if existing.Source == toRemove.Source && existing.Path == toRemove.Path {
				keep = false
				break
			}
		}
		if keep {
			remaining = append(remaining, existing)
		}
	}
	d.user.SafeModeItems = remaining

	if err := store.Users.Update(d.user, true, "SafeModeItems"); err != nil {
		logger.Errorf("safemode: failed to update user %s: %v", d.user.Username, err)
		return http.StatusInternalServerError, fmt.Errorf("failed to update SAFEMode: %w", err)
	}
	AcornStateSaveSafeMode(d.user.Username, d.user.SafeModePINHash, d.user.SafeModeItems)
	items := d.user.SafeModeItems
	if items == nil {
		items = []users.SafeModeItem{}
	}
	return renderJSON(w, r, map[string]interface{}{"items": items})
}

// safeModeVerifyHandler checks the PIN without changing any data.
// Returns { "valid": true/false }. Used for session unlock on the frontend.
// POST /api/safemode/verify
// Body: { "pin": "1234" }
func safeModeVerifyHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	var req struct {
		PIN string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err)
	}
	if d.user.SafeModePINHash == "" {
		return renderJSON(w, r, map[string]bool{"valid": false})
	}
	valid := hashSafePIN(req.PIN) == d.user.SafeModePINHash
	return renderJSON(w, r, map[string]bool{"valid": valid})
}
