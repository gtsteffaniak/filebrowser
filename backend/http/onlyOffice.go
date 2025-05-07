package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/common/cache"
	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
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
	if settings.Config.Integrations.OnlyOffice.Url == "" {
		return http.StatusInternalServerError, errors.New("only-office integration must be configured in settings")
	}
	encodedUrl := r.URL.Query().Get("url")
	// Decode the URL-encoded path
	url, err := url.QueryUnescape(encodedUrl)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	// get path from url
	pathParts := strings.Split(url, "/api/raw?files=")
	origPathParts := strings.Split(encodedUrl, "/api/raw?files=")
	encodedPath := origPathParts[len(origPathParts)-1]
	sourceFile := pathParts[len(pathParts)-1]
	sourceSplit := strings.Split(sourceFile, "::")
	if len(sourceSplit) != 2 {
		return http.StatusBadRequest, fmt.Errorf("invalid url path %v", url)
	}
	source := sourceSplit[0]
	path := sourceSplit[1]
	urlFirst := pathParts[0]
	if settings.Config.Server.InternalUrl != "" {
		urlFirst = settings.Config.Server.InternalUrl
		replacement := strings.Split(url, "/api/raw")[0]
		url = strings.Replace(url, replacement, settings.Config.Server.InternalUrl, 1)
	}
	userscope, err := settings.GetScopeFromSourceName(d.user.Scopes, source)
	if err != nil {
		return http.StatusForbidden, err
	}
	fileInfo, err := files.FileInfoFaster(iteminfo.FileOptions{
		Path:   utils.JoinPathAsUnix(userscope, path),
		Modify: d.user.Permissions.Modify,
		Source: source,
		Expand: false,
	})
	if err != nil {
		return errToStatus(err), err
	}
	id, err := getOnlyOfficeId(source, fileInfo.Path)
	if err != nil {
		return http.StatusNotFound, err
	}
	split := strings.Split(fileInfo.Name, ".")
	fileType := split[len(split)-1]
	theme := "light"
	if d.user.DarkMode {
		theme = "dark"
	}
	callbackURL := fmt.Sprintf("%v/api/onlyoffice/callback?path=%v&auth=%v", urlFirst, encodedPath, d.token)
	clientConfig := map[string]interface{}{
		"document": map[string]interface{}{
			"fileType": fileType,
			"key":      id,
			"title":    fileInfo.Name,
			"url":      url + "&auth=" + d.token,
			"permissions": map[string]interface{}{
				"edit":     d.user.Permissions.Modify,
				"download": true,
				"print":    true,
			},
		},
		"editorConfig": map[string]interface{}{
			"callbackUrl": callbackURL,
			"user": map[string]interface{}{
				"id":   strconv.FormatUint(uint64(d.user.ID), 10),
				"name": d.user.Username,
			},
			"customization": map[string]interface{}{
				"autosave":  true,
				"forcesave": true,
				"uiTheme":   theme,
			},
			"lang": d.user.Locale,
			"mode": "edit",
		},
	}
	if settings.Config.Integrations.OnlyOffice.Secret != "" {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(clientConfig))
		signature, err := token.SignedString([]byte(settings.Config.Integrations.OnlyOffice.Secret))
		if err != nil {
			return http.StatusInternalServerError, fmt.Errorf("failed to sign JWT")
		}
		clientConfig["token"] = signature
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
	pathParts := strings.Split(encodedPath, "::")
	if len(pathParts) < 2 {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	source := pathParts[0]
	if source == "" {
		source = settings.Config.Server.DefaultSource.Name
	}
	// Decode the URL-encoded path
	path, err := url.QueryUnescape(pathParts[1])
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
		deleteOfficeId(source, path)
	}

	if data.Status == onlyOfficeStatusDocumentClosedWithChanges ||
		data.Status == onlyOfficeStatusForceSaveWhileDocumentStillOpen {
		if !d.user.Permissions.Modify {
			return http.StatusForbidden, nil
		}

		doc, err := http.Get(data.URL)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		defer doc.Body.Close()

		fileOpts := iteminfo.FileOptions{
			Path:   path,
			Source: source,
		}
		writeErr := files.WriteFile(fileOpts, doc.Body)
		if writeErr != nil {
			return http.StatusInternalServerError, writeErr
		}
	}

	resp := map[string]int{
		"error": 0,
	}
	return renderJSON(w, r, resp)
}

func getOnlyOfficeId(source, path string) (string, error) {
	idx := indexing.GetIndex(source)
	if idx == nil {
		return "", fmt.Errorf("source not found")
	}
	realpath, _, _ := idx.GetRealPath(path)
	// error is intentionally ignored in order treat errors
	// the same as a cache-miss
	cachedDocumentKey, ok := cache.OnlyOffice.Get(realpath).(string)
	if ok {
		return cachedDocumentKey, nil
	}
	return "", fmt.Errorf("document key not found")
}

func deleteOfficeId(source, path string) {
	idx := indexing.GetIndex(source)
	if idx == nil {
		logger.Error(fmt.Sprintf("deleteOfficeId: failed to find source index for user home dir creation: %s", source))
		return
	}
	realpath, _, _ := idx.GetRealPath(path)
	cache.OnlyOffice.Delete(realpath)
}

func onlyofficeGetTokenHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// get config from body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer r.Body.Close()

	var payload map[string]interface{}
	// marshall to struct
	err = json.Unmarshal(body, &payload)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if settings.Config.Integrations.OnlyOffice.Secret != "" {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(payload))
		ss, err := token.SignedString([]byte(settings.Config.Integrations.OnlyOffice.Secret))
		if err != nil {
			return 500, errors.New("could not generate a new jwt")
		}
		return renderJSON(w, r, map[string]string{"token": ss})
	}
	return 400, fmt.Errorf("bad request")
}
