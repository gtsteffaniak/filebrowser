package http

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
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
func settingsGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	property := r.URL.Query().Get("property")
	if property != "" {
		// get property by name
		switch property {
		case "userDefaults":
			return renderJSON(w, r, config.UserDefaults)
		case "frontend":
			return renderJSON(w, r, config.Frontend)
		case "auth":
			return renderJSON(w, r, config.Auth)
		case "server":
			return renderJSON(w, r, config.Server)
		case "sources":
			return renderJSON(w, r, config.Server.Sources)
		default:
			return http.StatusNotFound, nil
		}
	}
	return renderJSON(w, r, config)
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
func settingsConfigHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// Parse query parameters
	fullParam := r.URL.Query().Get("full")
	commentsParam := r.URL.Query().Get("comments")

	showFull := fullParam == "true"
	showComments := commentsParam == "true"

	log.Printf("[DEBUG] settingsConfigHandler: full=%s, comments=%s, showFull=%v, showComments=%v", fullParam, commentsParam, showFull, showComments)
	log.Printf("[DEBUG] settingsConfigHandler: EmbeddedFs=%v", config.Server.EmbeddedFs)

	var err error
	var yamlConfig string

	// Always try to use embedded YAML for comments to avoid parsing source files
	log.Printf("[DEBUG] settingsConfigHandler: Attempting to use embedded YAML for comments")
	embeddedYaml, readErr := assets.ReadFile("embed/config.generated.yaml")
	if readErr != nil {
		log.Printf("[DEBUG] settingsConfigHandler: Error reading embedded YAML, falling back to file system: %v", readErr)
		// Try to read from file system as fallback
		embeddedYamlBytes, fsErr := os.ReadFile("frontend/public/config.generated.yaml")
		if fsErr != nil {
			embeddedYamlBytes, fsErr = os.ReadFile("../frontend/public/config.generated.yaml")
			if fsErr != nil {
				log.Printf("[DEBUG] settingsConfigHandler: Error reading from file system: %v", fsErr)
				return http.StatusInternalServerError, fmt.Errorf("error reading embedded YAML: %v", readErr)
			}
		}
		embeddedYaml = embeddedYamlBytes
	}

	log.Printf("[DEBUG] settingsConfigHandler: Using embedded YAML, length: %d bytes", len(embeddedYaml))
	yamlConfig, err = settings.GenerateConfigYamlWithEmbedded(config, showComments, showFull, false, string(embeddedYaml))
	if err != nil {
		log.Printf("[DEBUG] settingsConfigHandler: Error in GenerateConfigYamlWithEmbedded: %v", err)
		return http.StatusInternalServerError, fmt.Errorf("error generating YAML: %v", err)
	}

	// Set content type and write response
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write([]byte(yamlConfig)); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}
