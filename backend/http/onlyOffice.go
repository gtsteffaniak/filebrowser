package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/files"
	"github.com/gtsteffaniak/filebrowser/backend/logger"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
)

const (
	onlyOfficeStatusDocumentClosedWithChanges       = 2
	onlyOfficeStatusDocumentClosedWithNoChanges     = 4
	onlyOfficeStatusForceSaveWhileDocumentStillOpen = 6
)

type OnlyOfficeCallback struct {
	ChangesURL string   `json:"changesurl,omitempty"`
	Key        string   `json:"key,omitempty"`
	Status     int      `json:"status,omitempty"`
	URL        string   `json:"url,omitempty"`
	Users      []string `json:"users,omitempty"`
	UserData   string   `json:"userdata,omitempty"`
}

func onlyofficeClientConfigGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !settings.Config.Integrations.OnlyOffice.Enabled {
		return http.StatusInternalServerError, errors.New("only-office integration must be configured in settings")
	}

	if !d.user.Perm.Modify {
		return http.StatusForbidden, nil
	}
	encodedPath := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	if source == "" {
		source = "default"
	}
	// Decode the URL-encoded path
	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}

	fileInfo, err := files.FileInfoFaster(files.FileOptions{
		Path:       filepath.Join(d.user.Scope, path),
		Modify:     d.user.Perm.Modify,
		Source:     source,
		Expand:     false,
		ReadHeader: config.Server.TypeDetectionByHeader,
		Checker:    d.user,
	})

	if err != nil {
		return errToStatus(err), err
	}

	clientConfig := map[string]interface{}{
		"document": map[string]interface{}{
			"fileType": fileInfo.Type,
			"key":      files.getDocumentKey(source + fileInfo.Path),
			"title":    fileInfo.Name,
			"permissions": map[string]interface{}{
				"edit":     d.user.Perm.Modify,
				"download": d.user.Perm.Download,
				"print":    d.user.Perm.Download,
			},
		},
		"editorConfig": map[string]interface{}{
			"user": map[string]interface{}{
				"id":   strconv.FormatUint(uint64(d.user.ID), 10),
				"name": d.user.Username,
			},
			"customization": map[string]interface{}{
				"autosave":  true,
				"forcesave": true,
				"uiTheme":   ternary(d.user.DarkMode, "default-dark", "default-light"),
			},
			"lang": d.user.Locale,
			"mode": "edit",
		},
		"type": "desktop",
	}
	if settings.Config.Integrations.OnlyOffice.Enabled && settings.Config.Integrations.OnlyOffice.Secret != "" {
		claims := jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 10000)),
			Issuer:    "FileBrowser Quantum",
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signature, err := token.SignedString([]byte(settings.Config.Integrations.OnlyOffice.Secret))
		if err != nil {
			logger.Fatal(fmt.Sprintf("Error creating JWT signature: %v", err))
		}
		Config.Integrations.OnlyOffice.Secret = signature // Avoid overwriting the secret
	}
	return renderJSON(w, r, clientConfig)
}

func onlyofficeCallbackHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	var data OnlyOfficeCallback
	err = json.Unmarshal(body, &data)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	encodedPath := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	if source == "" {
		source = "default"
	}
	// Decode the URL-encoded path
	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	if data.Status == onlyOfficeStatusDocumentClosedWithChanges ||
		data.Status == onlyOfficeStatusDocumentClosedWithNoChanges {
		// Refer to only-office documentation
		// - https://api.onlyoffice.com/editors/coedit
		// - https://api.onlyoffice.com/editors/callback
		//
		// When the document is fully closed by all editors,
		// then the document key should no longer be re-used.
		files.OnlyOfficeCache.Delete(source + path)
	}

	if data.Status == onlyOfficeStatusDocumentClosedWithChanges ||
		data.Status == onlyOfficeStatusForceSaveWhileDocumentStillOpen {
		if !d.user.Perm.Modify {
			return http.StatusForbidden, nil
		}

		doc, err := http.Get(data.URL)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		defer doc.Body.Close()

		err = d.Runner.RunHook(func() error {
			fileOpts := files.FileOptions{
				Path:   path,
				Source: source,
			}
			writeErr := files.WriteFile(fileOpts, doc.Body)
			if writeErr != nil {
				return writeErr
			}
			return nil
		}, "save", path, "", d.user)

		if err != nil {
			return http.StatusInternalServerError, err
		}
	}

	resp := map[string]int{
		"error": 0,
	}
	return renderJSON(w, r, resp)
}
