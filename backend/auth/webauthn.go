package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

type webAuthnSession struct {
	SessionData *webauthn.SessionData
	UserID      uint
	ExpiresAt   time.Time
}

type WebAuthnService struct {
	wa        *webauthn.WebAuthn
	sessions  sync.Map
	stopCh    chan struct{}
	originMu  sync.Mutex
}

var webAuthnInstance *WebAuthnService

func InitWebAuthn() error {
	cfg := &settings.Config.Auth.Methods.PasskeyAuth
	if !cfg.Enabled {
		webAuthnInstance = nil
		return nil
	}

	waCfg, err := deriveWebAuthnConfig(cfg)
	if err != nil {
		return fmt.Errorf("webauthn config derivation failed: %w", err)
	}

	wa, err := webauthn.New(waCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize WebAuthn: %w", err)
	}

	webAuthnInstance = &WebAuthnService{
		wa:     wa,
		stopCh: make(chan struct{}),
	}
	go webAuthnInstance.cleanupExpiredSessions()
	return nil
}

func GetWebAuthn() *WebAuthnService {
	return webAuthnInstance
}

func IsWebAuthnEnabled() bool {
	return webAuthnInstance != nil
}

// EnsureOrigin dynamically adds the given origin to the allowed RP origins list.
// This supports self-hosted deployments where the access domain may vary.
func (s *WebAuthnService) EnsureOrigin(origin string) {
	s.originMu.Lock()
	defer s.originMu.Unlock()
	for _, o := range s.wa.Config.RPOrigins {
		if o == origin {
			return
		}
	}
	s.wa.Config.RPOrigins = append(s.wa.Config.RPOrigins, origin)
}

// EnsureRPID dynamically updates the relying party ID to match the request host.
func (s *WebAuthnService) EnsureRPID(rpID string) {
	s.originMu.Lock()
	defer s.originMu.Unlock()
	s.wa.Config.RPID = rpID
}

func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return hex.EncodeToString(b)
}

func (s *WebAuthnService) BeginMFALogin(username, password string, rpID string, userStore *users.Storage) (sessionID string, assertion *protocol.CredentialAssertion, err error) {
	user, getErr := userStore.Get(username)
	passwordHash := utils.InvalidPasswordHash
	if getErr == nil {
		passwordHash = user.Password
	}
	// Always run password check to prevent timing-based username enumeration
	if err = utils.CheckPwd(password, passwordHash); err != nil {
		return "", nil, errors.ErrUnauthorized
	}
	if getErr != nil {
		return "", nil, errors.ErrUnauthorized
	}

	if !user.HasPasskeyMFA() {
		return "", nil, errors.ErrPasskeyNoCredential
	}

	var opts []webauthn.LoginOption
	if rpID != "" {
		opts = append(opts, webauthn.WithLoginRelyingPartyID(rpID))
	}
	assertion, sessionData, err := s.wa.BeginLogin(user, opts...)
	if err != nil {
		return "", nil, fmt.Errorf("failed to begin WebAuthn login: %w", err)
	}

	sessionID = generateSessionID()
	s.sessions.Store(sessionID, &webAuthnSession{
		SessionData: sessionData,
		UserID:      user.ID,
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	})

	return sessionID, assertion, nil
}

func (s *WebAuthnService) FinishMFALogin(sessionID string, r *http.Request, userStore *users.Storage) (*users.User, error) {
	sessionVal, ok := s.sessions.Load(sessionID)
	if !ok {
		return nil, errors.ErrPasskeyInvalidSession
	}
	sess, ok := sessionVal.(*webAuthnSession)
	if !ok {
		return nil, errors.ErrPasskeyInvalidSession
	}
	defer s.sessions.Delete(sessionID)

	if time.Now().After(sess.ExpiresAt) {
		return nil, errors.ErrPasskeyInvalidSession
	}

	user, err := userStore.Get(sess.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	credential, err := s.wa.FinishLogin(user, *sess.SessionData, r)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errors.ErrPasskeyVerification, err)
	}

	credB64 := base64.RawURLEncoding.EncodeToString(credential.ID)
	for i := range user.PasskeyCredentials {
		if user.PasskeyCredentials[i].ID == credB64 {
			users.UpdateCredentialFromLibrary(&user.PasskeyCredentials[i], credential)
			if err := userStore.Update(user, false, "PasskeyCredentials"); err != nil {
				return nil, fmt.Errorf("failed to update credential: %w", err)
			}
			return user, nil
		}
	}

	return user, nil
}

func (s *WebAuthnService) BeginRegistration(user *users.User, rpID string) (sessionID string, creation *protocol.CredentialCreation, err error) {
	var opts []webauthn.RegistrationOption
	if rpID != "" {
		opts = append(opts, webauthn.WithRegistrationRelyingPartyID(rpID))
	}
	creation, sessionData, err := s.wa.BeginRegistration(user, opts...)
	if err != nil {
		return "", nil, fmt.Errorf("failed to begin WebAuthn registration: %w", err)
	}

	sessionID = generateSessionID()
	s.sessions.Store(sessionID, &webAuthnSession{
		SessionData: sessionData,
		UserID:      user.ID,
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	})

	return sessionID, creation, nil
}

func (s *WebAuthnService) FinishRegistration(user *users.User, sessionID string, credentialName string, r *http.Request) error {
	sessionVal, ok := s.sessions.Load(sessionID)
	if !ok {
		return errors.ErrPasskeyInvalidSession
	}
	sess, ok := sessionVal.(*webAuthnSession)
	if !ok {
		return errors.ErrPasskeyInvalidSession
	}
	defer s.sessions.Delete(sessionID)

	if time.Now().After(sess.ExpiresAt) {
		return errors.ErrPasskeyInvalidSession
	}

	if sess.UserID != user.ID {
		return errors.ErrUnauthorized
	}

	credential, err := s.wa.FinishRegistration(user, *sess.SessionData, r)
	if err != nil {
		return fmt.Errorf("failed to finish WebAuthn registration: %w", err)
	}

	credB64 := base64.RawURLEncoding.EncodeToString(credential.ID)
	for _, c := range user.PasskeyCredentials {
		if c.ID == credB64 {
			return errors.ErrPasskeyExists
		}
	}

	newCred := users.CredentialFromLibrary(credentialName, credential)
	user.PasskeyCredentials = append(user.PasskeyCredentials, newCred)
	return nil
}

func (s *WebAuthnService) DeleteCredential(user *users.User, credentialID string) error {
	for i, c := range user.PasskeyCredentials {
		if c.ID == credentialID {
			user.PasskeyCredentials = append(user.PasskeyCredentials[:i], user.PasskeyCredentials[i+1:]...)
			return nil
		}
	}
	return errors.ErrPasskeyNoCredential
}

func (s *WebAuthnService) cleanupExpiredSessions() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			s.sessions.Range(func(key, value interface{}) bool {
				sess, ok := value.(*webAuthnSession)
				if !ok {
					s.sessions.Delete(key)
					return true
				}
				if now.After(sess.ExpiresAt) {
					s.sessions.Delete(key)
				}
				return true
			})
		case <-s.stopCh:
			return
		}
	}
}

func deriveWebAuthnConfig(cfg *settings.PasskeyAuthConfig) (*webauthn.Config, error) {
	rpID := cfg.RPID
	if rpID == "" {
		if settings.Config.Server.ExternalUrl != "" {
			if u, err := url.Parse(settings.Config.Server.ExternalUrl); err == nil {
				rpID = u.Hostname()
			}
		}
		if rpID == "" {
			rpID = "localhost"
		}
	}

	displayName := cfg.RPDisplayName
	if displayName == "" {
		displayName = settings.Config.Frontend.Name
	}
	if displayName == "" {
		displayName = "FileBrowser Quantum"
	}

	origins := cfg.RPOrigins
	if len(origins) == 0 {
		if settings.Config.Server.ExternalUrl != "" {
			origins = append(origins, settings.Config.Server.ExternalUrl)
		}
		if settings.Config.Server.InternalUrl != "" && settings.Config.Server.InternalUrl != settings.Config.Server.ExternalUrl {
			origins = append(origins, settings.Config.Server.InternalUrl)
		}
		if len(origins) == 0 {
			origins = append(origins, "http://localhost")
		}
	}

	return &webauthn.Config{
		RPID:          rpID,
		RPDisplayName: displayName,
		RPOrigins:     origins,
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			UserVerification: protocol.VerificationPreferred,
		},
	}, nil
}
