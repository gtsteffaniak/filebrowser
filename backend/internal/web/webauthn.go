package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
)

// @Summary Begin passkey MFA login
// @Description Verifies the user's password and returns a WebAuthn assertion challenge for passkey MFA.
// @Tags Auth
// @Accept json
// @Produce json
// @Param username query string true "Username"
// @Param X-Password header string true "URL-encoded password"
// @Success 200 {object} map[string]interface{} "Passkey assertion options"
// @Failure 403 {object} map[string]string "Forbidden"
// @Router /api/auth/webauthn/begin-login [post]
func beginPasskeyLoginHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if !auth.IsWebAuthnEnabled() {
		return http.StatusForbidden, errors.ErrPasskeyNotEnabled
	}

	registerRequestOrigin(r)

	username := r.URL.Query().Get("username")
	password := r.Header.Get("X-Password")
	password, err := url.QueryUnescape(password)
	if err != nil {
		return http.StatusBadRequest, err
	}

	rpID := deriveRPID(r)
	svc := auth.GetWebAuthn()
	sessionID, assertion, err := svc.BeginMFALogin(username, password, rpID)
	if err != nil {
		if err == errors.ErrPasskeyNoCredential {
			return http.StatusForbidden, err
		}
		return http.StatusForbidden, err
	}

	response := map[string]interface{}{
		"sessionID": sessionID,
		"publicKey": assertion.Response,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	return http.StatusOK, json.NewEncoder(w).Encode(response)
}

// @Summary Finish passkey MFA login
// @Description Verifies the WebAuthn assertion and returns a JWT token.
// @Tags Auth
// @Accept json
// @Produce json
// @Param session_id query string true "Passkey session ID from begin-login"
// @Success 200 {string} string "JWT token"
// @Failure 403 {object} map[string]string "Forbidden"
// @Router /api/auth/webauthn/finish-login [post]
func finishPasskeyLoginHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if !auth.IsWebAuthnEnabled() {
		return http.StatusForbidden, errors.ErrPasskeyNotEnabled
	}

	registerRequestOrigin(r)

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		return http.StatusBadRequest, errors.ErrPasskeyInvalidSession
	}

	svc := auth.GetWebAuthn()
	user, err := svc.FinishMFALogin(sessionID, r)
	if err != nil {
		return http.StatusForbidden, err
	}
	if err = state.UpdateUser(user, "", "PasskeyCredentials"); err != nil {
		return http.StatusInternalServerError, err
	}

	d.User = user
	status, err := printToken(w, r, d.User)
	if err != nil || status != 0 {
		return status, err
	}
	activity.RecordLogin(r, d.User)
	return 0, nil
}

// @Summary Begin passkey registration
// @Description Returns WebAuthn credential creation options for adding a new passkey.
// @Tags Auth
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Passkey creation options"
// @Failure 403 {object} map[string]string "Forbidden"
// @Router /api/auth/webauthn/begin-register [post]
func beginPasskeyRegistrationHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if !auth.IsWebAuthnEnabled() {
		return http.StatusForbidden, errors.ErrPasskeyNotEnabled
	}

	registerRequestOrigin(r)

	rpID := deriveRPID(r)
	svc := auth.GetWebAuthn()
	sessionID, creation, err := svc.BeginRegistration(d.User, rpID)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	response := map[string]interface{}{
		"sessionID": sessionID,
		"publicKey": creation.Response,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	return http.StatusOK, json.NewEncoder(w).Encode(response)
}

// @Summary Finish passkey registration
// @Description Verifies the attestation and stores the new passkey credential.
// @Tags Auth
// @Accept json
// @Produce json
// @Param session_id query string true "Registration session ID"
// @Param name query string true "Display name for the passkey"
// @Success 200 {object} map[string]string "Success"
// @Failure 403 {object} map[string]string "Forbidden"
// @Router /api/auth/webauthn/finish-register [post]
func finishPasskeyRegistrationHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if !auth.IsWebAuthnEnabled() {
		return http.StatusForbidden, errors.ErrPasskeyNotEnabled
	}

	registerRequestOrigin(r)

	sessionID := r.URL.Query().Get("session_id")
	credentialName := r.URL.Query().Get("name")
	if sessionID == "" {
		return http.StatusBadRequest, errors.ErrPasskeyInvalidSession
	}
	if credentialName == "" {
		credentialName = "Passkey"
	}

	svc := auth.GetWebAuthn()
	if err := svc.FinishRegistration(d.User, sessionID, credentialName, r); err != nil {
		return http.StatusForbidden, err
	}

	if err := state.UpdateUser(d.User, "", "PasskeyCredentials"); err != nil {
		return http.StatusInternalServerError, err
	}

	activity.RecordAuth(r, d.User, activitydb.EventPasskeyRegister, activitydb.Details{
		PasskeyName: credentialName,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

// @Summary Delete a passkey credential
// @Description Removes a passkey credential by its ID.
// @Tags Auth
// @Param id path string true "Base64-encoded credential ID"
// @Success 200 {object} map[string]string "Success"
// @Failure 403 {object} map[string]string "Forbidden"
// @Router /api/auth/webauthn/{id} [delete]
func deletePasskeyCredentialHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if !auth.IsWebAuthnEnabled() {
		return http.StatusForbidden, errors.ErrPasskeyNotEnabled
	}

	credentialID := r.PathValue("id")
	if credentialID == "" {
		return http.StatusBadRequest, nil
	}

	passkeyName := credentialID
	for _, cred := range d.User.PasskeyCredentials {
		if cred.ID == credentialID {
			passkeyName = cred.Name
			break
		}
	}

	svc := auth.GetWebAuthn()
	if err := svc.DeleteCredential(d.User, credentialID); err != nil {
		return http.StatusNotFound, err
	}

	if err := state.UpdateUser(d.User, "", "PasskeyCredentials"); err != nil {
		return http.StatusInternalServerError, err
	}

	activity.RecordAuth(r, d.User, activitydb.EventPasskeyDelete, activitydb.Details{
		PasskeyName: passkeyName,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

func deriveRPID(r *http.Request) string {
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	if i := strings.LastIndex(host, ":"); i != -1 {
		host = host[:i]
	}
	return host
}

// registerRequestOrigin registers the request's origin and RP ID with the WebAuthn service.
func registerRequestOrigin(r *http.Request) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = strings.ToLower(proto)
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	origin := fmt.Sprintf("%s://%s", scheme, host)
	rpID := host
	if i := strings.LastIndex(rpID, ":"); i != -1 {
		rpID = rpID[:i]
	}
	svc := auth.GetWebAuthn()
	if svc != nil {
		svc.EnsureOrigin(origin)
		svc.EnsureRPID(rpID)
	}
}
