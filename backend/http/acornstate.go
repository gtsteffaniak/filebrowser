package http

// acornstate.go — Persistent state file for protection records and SAFEMode items.
//
// BoltDB lives at an ephemeral path on Azure Container Apps and is wiped on redeploy.
// This file writes a JSON snapshot to the persistent /srv volume so that both
// ChainFS protection records and SAFEMode items survive redeployments.
//
// The state file is placed at <firstSourcePath>/.acornstate.json.
// It lives outside any user directory, starts with a dot (hidden from listings),
// and is read-back into BoltDB on startup.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

// acornProtectionEntry mirrors the DB protection record for serialisation.
type acornProtectionEntry struct {
	FileGuid string `json:"fileGuid"`
	Expiry   int64  `json:"expiry"`
}

// acornSafeModeEntry holds one user's SAFEMode state.
type acornSafeModeEntry struct {
	PINHash string               `json:"pinHash"`
	Items   []users.SafeModeItem `json:"items"`
}

// acornStateData is the full JSON structure persisted to disk.
type acornStateData struct {
	Protections map[string]acornProtectionEntry `json:"protections"`
	SafeMode    map[string]acornSafeModeEntry   `json:"safeMode"`
}

var (
	acornStateMu   sync.Mutex
	acornStatePath string
	acornState     acornStateData
)

// InitAcornState derives the state-file path from the first configured source,
// loads any existing state, and restores protection records into BoltDB.
// Call once at startup, after settings are loaded and the DB store is ready.
func InitAcornState() {
	persistDir := "/srv" // safe fallback
	for _, src := range settings.Config.Server.SourceMap {
		persistDir = src.Path
		break
	}
	acornStatePath = filepath.Join(persistDir, ".acornstate.json")

	acornStateMu.Lock()
	defer acornStateMu.Unlock()

	data, err := os.ReadFile(acornStatePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Errorf("acornstate: could not read state file %s: %v", acornStatePath, err)
		}
		acornState = acornStateData{
			Protections: make(map[string]acornProtectionEntry),
			SafeMode:    make(map[string]acornSafeModeEntry),
		}
		return
	}
	if err := json.Unmarshal(data, &acornState); err != nil {
		logger.Errorf("acornstate: could not parse state file: %v", err)
		acornState = acornStateData{
			Protections: make(map[string]acornProtectionEntry),
			SafeMode:    make(map[string]acornSafeModeEntry),
		}
		return
	}
	if acornState.Protections == nil {
		acornState.Protections = make(map[string]acornProtectionEntry)
	}
	if acornState.SafeMode == nil {
		acornState.SafeMode = make(map[string]acornSafeModeEntry)
	}

	// Restore protection records into BoltDB so the rest of the app
	// can use the normal DB-backed lookup path.
	restored := 0
	for realPath, e := range acornState.Protections {
		if err := store.Protection.Save(realPath, e.FileGuid, e.Expiry); err != nil {
			logger.Errorf("acornstate: could not restore protection for %s: %v", realPath, err)
		} else {
			restored++
		}
	}
	if restored > 0 {
		logger.Infof("acornstate: restored %d protection record(s) from %s", restored, acornStatePath)
	}
}

// saveAcornStateLocked writes the in-memory state to disk.
// Caller must hold acornStateMu.
func saveAcornStateLocked() {
	if acornStatePath == "" {
		return
	}
	data, err := json.MarshalIndent(acornState, "", "  ")
	if err != nil {
		logger.Errorf("acornstate: marshal error: %v", err)
		return
	}
	if err := os.WriteFile(acornStatePath, data, 0600); err != nil {
		logger.Errorf("acornstate: could not write state file %s: %v", acornStatePath, err)
	}
}

// AcornStateSaveProtection adds or updates a protection record in both the
// in-memory state and the on-disk snapshot.
func AcornStateSaveProtection(realPath, fileGuid string, expiry int64) {
	acornStateMu.Lock()
	defer acornStateMu.Unlock()
	acornState.Protections[realPath] = acornProtectionEntry{FileGuid: fileGuid, Expiry: expiry}
	saveAcornStateLocked()
}

// AcornStateRemoveProtection deletes a protection record from state + disk.
func AcornStateRemoveProtection(realPath string) {
	acornStateMu.Lock()
	defer acornStateMu.Unlock()
	delete(acornState.Protections, realPath)
	saveAcornStateLocked()
}

// AcornStateSaveSafeMode persists a user's SAFEMode items and PIN hash.
func AcornStateSaveSafeMode(username, pinHash string, items []users.SafeModeItem) {
	acornStateMu.Lock()
	defer acornStateMu.Unlock()
	if items == nil {
		items = []users.SafeModeItem{}
	}
	acornState.SafeMode[username] = acornSafeModeEntry{PINHash: pinHash, Items: items}
	saveAcornStateLocked()
}

// AcornStateGetSafeMode returns the persisted SAFEMode state for a user, or nil if not found.
func AcornStateGetSafeMode(username string) *acornSafeModeEntry {
	acornStateMu.Lock()
	defer acornStateMu.Unlock()
	entry, ok := acornState.SafeMode[username]
	if !ok {
		return nil
	}
	return &entry
}

