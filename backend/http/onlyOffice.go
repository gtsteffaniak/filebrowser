package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/files"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
	"github.com/gtsteffaniak/filebrowser/backend/utils"
)

const (
	onlyOfficeStatusDocumentClosedWithChanges       = 2
	onlyOfficeStatusDocumentClosedWithNoChanges     = 4
	onlyOfficeStatusForceSaveWhileDocumentStillOpen = 6
	trueString                                      = "true"         // linter-enforced constant
	twoDays                                         = 48 * time.Hour // linter enforced constant
)

var (
	OnlyOfficeCache = utils.NewCache(48*time.Hour, 48*time.Hour)
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
			"key":      getDocumentKey(source + fileInfo.Path),
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

	docPath := r.URL.Query().Get("save")
	if docPath == "" {
		return http.StatusInternalServerError, errors.New("unable to get file save path")
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
		OnlyOfficeCache.Delete(source + path)
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
				Path: docPath,
			}
			writeErr := files.WriteFile(fileOpts, doc.Body)
			if writeErr != nil {
				return writeErr
			}
			return nil
		}, "save", docPath, "", d.user)

		if err != nil {
			return http.StatusInternalServerError, err
		}
	}

	resp := map[string]int{
		"error": 0,
	}
	return renderJSON(w, r, resp)
}

func getDocumentKey(realPath string) string {
	// error is intentionally ignored in order treat errors
	// the same as a cache-miss
	cachedDocumentKey, ok := OnlyOfficeCache.Get(realPath).(string)
	if ok {
		return cachedDocumentKey
	}

	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	documentKey := hashSHA256(realPath + timestamp)
	OnlyOfficeCache.Set(realPath, documentKey)
	return documentKey
}

func hashSHA256(data string) string {
	bytes := sha256.Sum256([]byte(data))
	return hex.EncodeToString(bytes[:])
}

func ternary(condition bool, a, b string) string {
	if condition {
		return a
	}
	return b
}
