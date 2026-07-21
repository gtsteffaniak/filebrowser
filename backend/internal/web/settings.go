package web

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// settingsGetHandler retrieves the current system settings.
// @Summary Get system settings
// @Description Returns the current configuration settings for signup, user directories, rules, frontend.
// @Tags Settings
// @Accept json
// @Produce json
// @Param property query string false "Property to retrieve: `userDefaults`, `frontend`, `auth`, `server`, `sources`"
// @Success 200 {object} settings.Settings "System settings data"
// @Router /api/settings [get]
func settingsGetHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	property := r.URL.Query().Get("property")
	if property != "" {
		// get property by name
		switch property {
		case "userDefaults":
			return RenderJSON(w, r, state.GetDefaultUserDefaults())
		case "frontend":
			return RenderJSON(w, r, settings.Config.Frontend)
		case "auth":
			return RenderJSON(w, r, settings.Config.Auth)
		case "server":
			return RenderJSON(w, r, settings.Config.Server)
		case "sources":
			return RenderJSON(w, r, settings.Config.Server.Sources)
		default:
			return http.StatusNotFound, nil
		}
	}
	return RenderJSON(w, r, &settings.Config)
}

// settingsConfigHandler returns the current system settings as YAML.
// @Summary Get system settings as YAML
// @Description Returns the current running configuration settings as YAML format with optional comments and filtering.
// @Tags Settings
// @Accept json
// @Produce text/plain
// @Param full query boolean false "Show all values (true) or only non-default values (false, default)"
// @Param comments query boolean false "Include comments in YAML (true) or plain YAML (false, default)"
// @Success 200 {string} string "System settings in YAML format"
// @Router /api/settings/config [get]
func settingsConfigHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	// Parse query parameters
	fullParam := r.URL.Query().Get("full")
	commentsParam := r.URL.Query().Get("comments")

	showFull := fullParam == "true"
	showComments := commentsParam == "true"

	var err error
	var yamlConfig string

	// Try to read the generated YAML file for comments (should always exist)
	var embeddedYaml []byte
	var readErr error

	if settings.Env.EmbeddedFs {
		embeddedYaml, readErr = fs.ReadFile(assetFs, "embed/config.generated.yaml")
	} else {
		embeddedYaml, readErr = os.ReadFile("internal/web/dist/config.generated.yaml")
		if readErr != nil {
			return http.StatusInternalServerError, fmt.Errorf("error reading generated YAML: %v", readErr)
		}
	}

	// If we successfully read the generated YAML, use it as the comment source
	if readErr == nil && len(embeddedYaml) > 0 {
		yamlConfig, err = settings.GenerateConfigYamlWithEmbedded(&settings.Config, showComments, showFull, false, string(embeddedYaml))
		if err != nil {
			return http.StatusInternalServerError, fmt.Errorf("error generating YAML: %v", err)
		}
	} else {
		// Fallback to Go source parsing if generated YAML doesn't exist
		yamlConfig, err = settings.GenerateConfigYaml(&settings.Config, showComments, showFull, false)
		if err != nil {
			return http.StatusInternalServerError, fmt.Errorf("error generating YAML: %v", err)
		}
	}

	// Set content type and write response
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write([]byte(yamlConfig)); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}

func getSourceInfoHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	sources := d.User.GetSourceNames()
	reducedIndexes := map[string]indexing.ReducedIndex{}
	for _, source := range sources {
		reducedIndex, err := indexing.GetIndexInfo(source, false)
		if err != nil {
			logger.Debugf("error getting index info: %v", err)
			continue
		}
		showScannerInfo := r.URL.Query().Get("scanners") == "true"
		if !showScannerInfo {
			reducedIndex.Scanners = nil
		}
		reducedIndexes[source] = reducedIndex
	}
	return RenderJSON(w, r, reducedIndexes)
}

type userDefaultsResponse struct {
	Values           *settings.UserDefaults            `json:"values,omitempty"`
	Enforced         settings.UserDefaultsEnforcement `json:"enforced"`
	LockedFromConfig bool                             `json:"lockedFromConfig,omitempty"`
	LockMessage      string                           `json:"lockMessage,omitempty"`
}

func userDefaultsLockMeta() (locked bool, message string) {
	if settings.UserDefaultsLockedFromConfig() {
		return true, settings.UserDefaultsConfigLockMessage
	}
	return false, ""
}

func settingsUserDefaultsGetHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if scope := r.URL.Query().Get("scope"); scope != "" && scope != "default" {
		return http.StatusBadRequest, fmt.Errorf("per-login user defaults scopes are no longer supported")
	}
	locked, lockMessage := userDefaultsLockMeta()
	enforced := state.GetEnforcedUserDefaults()
	if !d.User.Permissions.Admin {
		return RenderJSON(w, r, userDefaultsResponse{
			Enforced:         enforced,
			LockedFromConfig: locked,
			LockMessage:      lockMessage,
		})
	}
	values := state.GetUserDefaults()
	return RenderJSON(w, r, userDefaultsResponse{
		Values:           &values,
		Enforced:         enforced,
		LockedFromConfig: locked,
		LockMessage:      lockMessage,
	})
}

func settingsUserDefaultsPatchHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if settings.UserDefaultsLockedFromConfig() {
		return http.StatusForbidden, fmt.Errorf("%s", settings.UserDefaultsConfigLockMessage)
	}
	if scope := r.URL.Query().Get("scope"); scope != "" && scope != "default" {
		return http.StatusBadRequest, fmt.Errorf("per-login user defaults scopes are no longer supported")
	}
	patchJSON, err := io.ReadAll(r.Body)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("read user defaults patch: %w", err)
	}
	if len(patchJSON) == 0 {
		return http.StatusBadRequest, fmt.Errorf("empty user defaults patch body")
	}

	var top map[string]json.RawMessage
	if err = json.Unmarshal(patchJSON, &top); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid user defaults patch JSON: %w", err)
	}
	var enforcedPatch []byte
	if raw, ok := top["enforced"]; ok {
		enforcedPatch = raw
		delete(top, "enforced")
	}
	valuesPatch, err := json.Marshal(top)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("marshal user defaults values patch: %w", err)
	}

	hasValues := len(valuesPatch) > 2
	hasEnforced := len(enforcedPatch) > 0
	if hasValues && hasEnforced {
		return http.StatusBadRequest, fmt.Errorf("patch values and enforced in a single request is not supported")
	}
	if !hasValues && !hasEnforced {
		return http.StatusBadRequest, fmt.Errorf("empty user defaults patch body")
	}

	if hasValues {
		if err := settings.ValidateSinglePropertyUserDefaultsPatch(valuesPatch); err != nil {
			return http.StatusBadRequest, err
		}
		if err := state.PatchUserDefaults(valuesPatch); err != nil {
			logger.Errorf("failed to patch user defaults: %v", err)
			return http.StatusInternalServerError, fmt.Errorf("failed to update user defaults")
		}
	}
	if hasEnforced {
		if err := settings.ValidateSinglePropertyUserDefaultsPatch(enforcedPatch); err != nil {
			return http.StatusBadRequest, err
		}
		if err := state.PatchUserDefaultsEnforced(enforcedPatch); err != nil {
			logger.Errorf("failed to patch enforced user defaults: %v", err)
			return http.StatusInternalServerError, fmt.Errorf("failed to update enforced user defaults")
		}
	}
	return http.StatusNoContent, nil
}

type sourceSettingsPatch struct {
	DefaultPermissions *users.SourceFilePermissions `json:"defaultPermissions,omitempty"`
}

func settingsSourceGetHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	// Read-only global source file permission defaults (used by user edit/create UI).
	return RenderJSON(w, r, state.GetSourceSettings())
}

func settingsSourcePatchHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	var body sourceSettingsPatch
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, fmt.Errorf("failed to decode source settings patch: %w", err)
	}
	defer r.Body.Close()

	if body.DefaultPermissions == nil {
		return http.StatusBadRequest, fmt.Errorf("source settings patch must include defaultPermissions")
	}

	if err := state.SetSourceAccessDefaults(*body.DefaultPermissions); err != nil {
		logger.Errorf("failed to update source settings: %v", err)
		return http.StatusInternalServerError, fmt.Errorf("failed to update source settings")
	}
	return RenderJSON(w, r, state.GetSourceSettings())
}
