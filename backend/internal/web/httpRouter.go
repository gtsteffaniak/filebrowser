package web

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

const (
	time60s = 60 * time.Second
	time30s = 30 * time.Second
	time10s = 10 * time.Second
	time5s  = 5 * time.Second
)

// configureHTTPRouter registers all API, public, and static routes.
func configureHTTPRouter(router, api, publicRoutes, publicApi *http.ServeMux) {
	// health routes
	api.HandleFunc("GET /health", healthHandler)
	publicApi.HandleFunc("GET /health", healthHandler)

	// ========================================
	// User Routes - /api/users/ (with public routes)
	// ========================================
	api.HandleFunc("GET /users", withUser(userGetHandler))
	api.HandleFunc("POST /users", withSelfOrAdmin(usersPostHandler))
	api.HandleFunc("PATCH /users", withUser(userPatchHandler))
	api.HandleFunc("PATCH /users/pinned-items", withUser(userPatchPinnedItemsHandler))
	api.HandleFunc("DELETE /users", withSelfOrAdmin(userDeleteHandler))
	publicApi.HandleFunc("GET /users", withUser(userGetHandler))

	// ========================================
	// Auth Routes - /api/auth/
	// ========================================
	api.HandleFunc("POST /auth/login", withRateLimit(AuthRateLimitCredentialLockout, loginHelper(loginHandler)))
	api.HandleFunc("POST /auth/logout", withOrWithoutUser(withRateLimitChain(AuthRateLimitModerate, logoutHandler)))
	api.HandleFunc("POST /auth/signup", withoutUser(withRateLimitChain(AuthRateLimitModerate, signupHandler)))
	api.HandleFunc("POST /auth/otp/generate", withOrWithoutUser(withRateLimitChain(AuthRateLimitModerate, generateOTPHandler)))
	api.HandleFunc("POST /auth/otp/verify", withOrWithoutUser(withRateLimitChain(AuthRateLimitCredentialLockout, verifyOTPHandler)))
	api.HandleFunc("POST /auth/renew", withUser(withRateLimitChain(AuthRateLimitAuthenticated, renewHandler)))
	api.HandleFunc("POST /auth/token", withUser(withRateLimitChain(AuthRateLimitAuthenticated, createApiTokenHandler)))
	api.HandleFunc("DELETE /auth/token", withUser(withRateLimitChain(AuthRateLimitAuthenticated, deleteApiTokenHandler)))
	api.HandleFunc("GET /auth/token/list", withUser(withRateLimitChain(AuthRateLimitAuthenticated, listApiTokensHandler)))
	api.HandleFunc("GET /auth/token", withUser(withRateLimitChain(AuthRateLimitAuthenticated, getApiTokenHandler)))
	api.HandleFunc("GET /auth/oidc/callback", withRateLimit(AuthRateLimitOIDC, oidcCallbackHandler))
	api.HandleFunc("GET /auth/oidc/login", withRateLimit(AuthRateLimitOIDC, oidcLoginHandler))
	api.HandleFunc("POST /auth/webauthn/begin-login", withoutUser(withRateLimitChain(AuthRateLimitCredential, beginPasskeyLoginHandler)))
	api.HandleFunc("POST /auth/webauthn/finish-login", withoutUser(withRateLimitChain(AuthRateLimitCredential, finishPasskeyLoginHandler)))
	api.HandleFunc("POST /auth/webauthn/begin-register", withUser(withRateLimitChain(AuthRateLimitAuthenticated, beginPasskeyRegistrationHandler)))
	api.HandleFunc("POST /auth/webauthn/finish-register", withUser(withRateLimitChain(AuthRateLimitAuthenticated, finishPasskeyRegistrationHandler)))
	api.HandleFunc("DELETE /auth/webauthn/{id}", withUser(withRateLimitChain(AuthRateLimitAuthenticated, deletePasskeyCredentialHandler)))

	// ========================================
	// Resources Routes - /api/resources/ (with public routes)
	// ========================================
	api.HandleFunc("GET /resources", withUser(resourceGetHandler))
	api.HandleFunc("GET /resources/items", withUser(itemsGetHandler))
	api.HandleFunc("DELETE /resources", withUser(resourceDeleteHandler))
	api.HandleFunc("POST /resources", withUser(ResourcePostHandler))
	api.HandleFunc("PUT /resources", withUser(resourcePutHandler))
	api.HandleFunc("PATCH /resources", withUser(ResourcePatchHandler))
	api.HandleFunc("DELETE /resources/bulk", withUser(ResourceBulkDeleteHandler))
	api.HandleFunc("POST /resources/archive", withUser(archiveCreateHandler))
	api.HandleFunc("POST /resources/unarchive", withUser(unarchiveHandler))
	api.HandleFunc("GET /resources/download", withUser(downloadHandler))
	api.HandleFunc("GET /resources/view", withTimeout(time60s, withUserHelper(viewHandler)))
	api.HandleFunc("GET /resources/preview", withTimeout(time30s, withUserHelper(previewHandler)))
	api.HandleFunc("POST /resources/pause", withUser(resourcePauseHandler))
	publicApi.HandleFunc("GET /resources", withHashFile(publicGetResourceHandler))
	publicApi.HandleFunc("GET /resources/items", withHashFile(publicItemsGetHandler))
	publicApi.HandleFunc("POST /resources", withHashFile(publicUploadHandler))
	publicApi.HandleFunc("PUT /resources", withHashFile(publicPutHandler))
	publicApi.HandleFunc("DELETE /resources", withHashFile(publicDeleteHandler))
	publicApi.HandleFunc("DELETE /resources/bulk", withHashFile(publicBulkDeleteHandler))
	publicApi.HandleFunc("PATCH /resources", withHashFile(publicPatchHandler))
	publicApi.HandleFunc("GET /resources/download", withHashFile(publicDownloadHandler))
	publicApi.HandleFunc("GET /resources/view", withTimeout(time60s, withHashFileHelper(PublicViewHandler)))
	publicApi.HandleFunc("GET /resources/preview", withTimeout(time30s, withHashFileHelper(publicPreviewHandler)))
	publicApi.HandleFunc("POST /resources/pause", withHashFile(PublicPauseHandler))
	// Legacy routes (backwards compatibility)
	api.HandleFunc("GET /raw", withUser(downloadHandler))
	publicApi.HandleFunc("GET /raw", withHashFile(publicDownloadHandler))

	// ========================================
	// Access Routes - /api/access/
	// ========================================
	api.HandleFunc("GET /access", withAdmin(accessGetHandler))
	api.HandleFunc("POST /access", withAdmin(accessPostHandler))
	api.HandleFunc("PATCH /access", withAdmin(accessPatchHandler))
	api.HandleFunc("DELETE /access", withAdmin(accessDeleteHandler))
	api.HandleFunc("GET /access/groups", withAdmin(groupGetHandler))
	api.HandleFunc("POST /access/group", withAdmin(groupPostHandler))
	api.HandleFunc("DELETE /access/group", withAdmin(groupDeleteHandler))

	// ========================================
	// Share Routes - /api/share/
	// ========================================
	api.HandleFunc("GET /share/list", withPermShare(shareListHandler))
	api.HandleFunc("GET /share/direct", withPermShare(shareDirectDownloadHandler))
	api.HandleFunc("GET /share", withUser(shareGetHandler))
	api.HandleFunc("POST /share", withPermShare(sharePostHandler))
	api.HandleFunc("PATCH /share", withPermShare(sharePatchHandler))
	api.HandleFunc("DELETE /share", withPermShare(shareDeleteHandler))
	publicApi.HandleFunc("GET /share/info", withOrWithoutUser(shareInfoHandler))
	publicApi.HandleFunc("PATCH /share/pinned-items", withPermShare(sharePatchPinnedItemsHandler))
	publicApi.HandleFunc("GET /share/image", withHashFile(getShareImage))

	// ========================================
	// Settings Routes - /api/settings/
	// ========================================
	api.HandleFunc("GET /settings", withAdmin(settingsGetHandler))
	api.HandleFunc("GET /settings/config", withAdmin(settingsConfigHandler))
	api.HandleFunc("GET /settings/analytics", withTimeout(time5s, withAdminHelper(settingsAnalyticsGetHandler)))
	api.HandleFunc("PUT /settings/analytics", withTimeout(time5s, withAdminHelper(settingsAnalyticsUpdateHandler)))
	api.HandleFunc("PATCH /settings/analytics", withTimeout(time5s, withAdminHelper(settingsAnalyticsUpdateHandler)))
	api.HandleFunc("GET /settings/analytics/preview", withTimeout(time30s, withAdminHelper(settingsAnalyticsPreviewHandler)))
	api.HandleFunc("GET /settings/user-defaults", withTimeout(time5s, withUserHelper(settingsUserDefaultsGetHandler)))
	api.HandleFunc("PATCH /settings/user-defaults", withTimeout(time5s, withAdminHelper(settingsUserDefaultsPatchHandler)))
	publicApi.HandleFunc("GET /settings/user-defaults", withTimeout(time5s, withUserHelper(settingsUserDefaultsGetHandler)))
	api.HandleFunc("GET /settings/source", withTimeout(time5s, withUserHelper(settingsSourceGetHandler)))
	api.HandleFunc("PATCH /settings/source", withTimeout(time5s, withAdminHelper(settingsSourcePatchHandler)))
	api.HandleFunc("GET /settings/sources", withUser(getSourceInfoHandler))

	// ========================================
	// Tools Routes - /api/tools/
	// ========================================
	api.HandleFunc("GET /tools/search", withUser(searchHandler))
	api.HandleFunc("GET /tools/duplicate-finder", withUser(duplicatesHandler))
	api.HandleFunc("GET /tools/file-watcher", withUser(fileWatchHandler))
	api.HandleFunc("GET /tools/file-watcher/sse", withUser(fileWatchSSEHandler))
	api.HandleFunc("GET /tools/activity", withUser(ListHandler))
	api.HandleFunc("GET /tools/activity/grouped", withUser(GroupedHandler))
	api.HandleFunc("GET /tools/activity/export", withUser(ExportHandler))

	// ========================================
	// Media Routes - /api/media/ (with public routes)
	// ========================================
	api.HandleFunc("GET /media/subtitles", withTimeout(time60s, withUserHelper(subtitlesHandler)))
	api.HandleFunc("GET /media/metadata", withTimeout(time60s, withUserHelper(metadataHandler)))
	api.HandleFunc("GET /media/lyrics", withTimeout(time60s, withUserHelper(lyricsHandler)))
	api.HandleFunc("GET /media/stream", withTimeout(time60s, withUserHelper(streamHandler)))
	publicApi.HandleFunc("GET /media/metadata", withTimeout(time60s, withHashFileHelper(publicMetadataHandler)))
	publicApi.HandleFunc("GET /media/lyrics", withTimeout(time60s, withHashFileHelper(publicLyricsHandler)))
	publicApi.HandleFunc("GET /media/stream", withTimeout(time60s, withHashFileHelper(publicStreamHandler)))

	// ========================================
	// OnlyOffice Routes - /api/office/ (with public routes)
	// ========================================
	api.HandleFunc("GET /office/config", withUser(onlyofficeClientConfigGetHandler))
	api.HandleFunc("POST /office/callback", withUser(onlyofficeCallbackHandler))
	api.HandleFunc("GET /office/callback", withUser(onlyofficeCallbackHandler))
	publicApi.HandleFunc("POST /office/callback", withHashFile(onlyofficeCallbackHandler))
	publicApi.HandleFunc("GET /office/callback", withHashFile(onlyofficeCallbackHandler))
	publicApi.HandleFunc("GET /office/config", withHashFile(onlyofficeClientConfigGetHandler))

	// ========================================
	// Misc Routes
	// ========================================
	api.HandleFunc("GET /events", withUser(SSEHandler))
	if settings.Env.IsDevMode {
		api.HandleFunc("GET /inspect-index", inspectIndex)
		api.HandleFunc("GET /mock-data", mockData)
	}

	// Mount public API
	publicRoutes.Handle("/api/", http.StripPrefix("/api", publicApi))

	// ========================================
	// Configure Main Router
	// ========================================
	apiPath := settings.Config.Http.BaseURL + "api"
	publicPath := settings.Config.Http.BaseURL + "public"
	webDavPath := settings.Config.Http.BaseURL + "dav"

	// Mount primary API and public routes
	router.Handle(apiPath+"/", http.StripPrefix(apiPath, api))
	router.Handle(publicPath+"/", http.StripPrefix(publicPath, publicRoutes))

	// WebDAV handler
	if !settings.Config.Http.DisableWebDAV {
		// Uses Basic Auth where password is JWT token
		// Note: do not trim /dav prefix here - webdav library requires it
		router.Handle(webDavPath+"/{source}/{path...}", withBasicAuth(webDAVHandler))
	}

	publicRoutes.HandleFunc("GET /share/", withOrWithoutUser(indexHandler))

	// Static assets
	publicRoutes.Handle("GET /static/", http.HandlerFunc(staticAssetHandler))
	router.HandleFunc("GET /favicon.svg", http.HandlerFunc(staticAssetHandler))

	// Index and utility routes
	router.HandleFunc(settings.Config.Http.BaseURL, withOrWithoutUser(indexHandler))
	router.HandleFunc(fmt.Sprintf("GET %vhealth", settings.Config.Http.BaseURL), healthHandler)
	router.Handle(fmt.Sprintf("%vswagger/", settings.Config.Http.BaseURL), withUser(swaggerHandler))

	// Base URL redirect (non-root deployments)
	if settings.Config.Http.BaseURL != "/" {
		router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, settings.Config.Http.BaseURL, http.StatusMovedPermanently)
		})
	}
}
