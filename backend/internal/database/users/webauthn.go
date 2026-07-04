package users

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthnID returns the user handle used by WebAuthn.
func (u *User) WebAuthnID() []byte {
	return []byte(fmt.Sprintf("%d", u.ID))
}

// WebAuthnName returns the human-readable name for the user.
func (u *User) WebAuthnName() string {
	return u.Username
}

// WebAuthnDisplayName returns the display name for the user.
func (u *User) WebAuthnDisplayName() string {
	return u.Username
}

// WebAuthnCredentials returns the stored credentials converted to the library's type.
func (u *User) WebAuthnCredentials() []webauthn.Credential {
	creds := make([]webauthn.Credential, len(u.PasskeyCredentials))
	for i, c := range u.PasskeyCredentials {
		creds[i] = *credentialToLibrary(&c)
	}
	return creds
}

// HasPasskeyMFA returns true if the user has at least one passkey credential configured.
func (u *User) HasPasskeyMFA() bool {
	return len(u.PasskeyCredentials) > 0
}

// credentialToLibrary converts our BoltDB-safe WebAuthnCredential to the library's Credential.
func credentialToLibrary(c *WebAuthnCredential) *webauthn.Credential {
	id, _ := base64.RawURLEncoding.DecodeString(c.ID)
	pubKey, _ := base64.RawURLEncoding.DecodeString(c.PublicKey)
	aaguid, _ := base64.RawURLEncoding.DecodeString(c.AAGUID)
	cdj, _ := base64.RawURLEncoding.DecodeString(c.ClientDataJSON)
	cdh, _ := base64.RawURLEncoding.DecodeString(c.ClientDataHash)
	authData, _ := base64.RawURLEncoding.DecodeString(c.AuthenticatorData)
	attObj, _ := base64.RawURLEncoding.DecodeString(c.AttestationObj)

	transports := make([]protocol.AuthenticatorTransport, len(c.Transport))
	for i, t := range c.Transport {
		transports[i] = protocol.AuthenticatorTransport(t)
	}

	return &webauthn.Credential{
		ID:                id,
		PublicKey:         pubKey,
		AttestationType:   c.AttestationType,
		AttestationFormat: c.AttestationFormat,
		Transport:         transports,
		Flags: webauthn.CredentialFlags{
			UserPresent:    c.Flags.UserPresent,
			UserVerified:   c.Flags.UserVerified,
			BackupEligible: c.Flags.BackupEligible,
			BackupState:    c.Flags.BackupState,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:       aaguid,
			SignCount:    c.SignCount,
			CloneWarning: c.CloneWarning,
		},
		Attestation: webauthn.CredentialAttestation{
			ClientDataJSON:     cdj,
			ClientDataHash:     cdh,
			AuthenticatorData:  authData,
			PublicKeyAlgorithm: c.PublicKeyAlg,
			Object:             attObj,
		},
	}
}

// CredentialFromLibrary converts a library Credential to our BoltDB-safe format.
func CredentialFromLibrary(name string, cred *webauthn.Credential) WebAuthnCredential {
	transports := make([]string, len(cred.Transport))
	for i, t := range cred.Transport {
		transports[i] = string(t)
	}

	now := time.Now().Unix()

	return WebAuthnCredential{
		ID:                base64.RawURLEncoding.EncodeToString(cred.ID),
		PublicKey:         base64.RawURLEncoding.EncodeToString(cred.PublicKey),
		AttestationType:   cred.AttestationType,
		AttestationFormat: cred.AttestationFormat,
		Transport:         transports,
		Flags: WebAuthnCredentialFlags{
			UserPresent:    cred.Flags.UserPresent,
			UserVerified:   cred.Flags.UserVerified,
			BackupEligible: cred.Flags.BackupEligible,
			BackupState:    cred.Flags.BackupState,
		},
		AAGUID:            base64.RawURLEncoding.EncodeToString(cred.Authenticator.AAGUID),
		SignCount:         cred.Authenticator.SignCount,
		CloneWarning:      cred.Authenticator.CloneWarning,
		Name:              name,
		CreatedAt:         now,
		LastUsedAt:        now,
		ClientDataJSON:    base64.RawURLEncoding.EncodeToString(cred.Attestation.ClientDataJSON),
		ClientDataHash:    base64.RawURLEncoding.EncodeToString(cred.Attestation.ClientDataHash),
		AuthenticatorData: base64.RawURLEncoding.EncodeToString(cred.Attestation.AuthenticatorData),
		PublicKeyAlg:      cred.Attestation.PublicKeyAlgorithm,
		AttestationObj:    base64.RawURLEncoding.EncodeToString(cred.Attestation.Object),
	}
}

// UpdateCredentialFromLibrary updates an existing WebAuthnCredential from the library's Credential
// (used after login to update sign count, flags, and last used time).
func UpdateCredentialFromLibrary(c *WebAuthnCredential, cred *webauthn.Credential) {
	c.SignCount = cred.Authenticator.SignCount
	c.Flags.UserPresent = cred.Flags.UserPresent
	c.Flags.UserVerified = cred.Flags.UserVerified
	c.Flags.BackupEligible = cred.Flags.BackupEligible
	c.Flags.BackupState = cred.Flags.BackupState
	c.LastUsedAt = time.Now().Unix()
}
