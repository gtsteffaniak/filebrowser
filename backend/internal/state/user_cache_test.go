package state

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestCloneUserPtr_passkeyCredentialsNotAliased(t *testing.T) {
	orig := &users.User{
		FrontendUser: users.FrontendUser{Username: "u1"},
		PasskeyCredentials: []users.WebAuthnCredential{
			{ID: "id1", PublicKey: "secret", Transport: []string{"usb"}},
		},
	}
	cloned := cloneUserPtr(orig)
	if cloned == orig {
		t.Fatal("expected distinct pointer")
	}
	cloned.PasskeyCredentials[0].PublicKey = ""
	if orig.PasskeyCredentials[0].PublicKey != "secret" {
		t.Fatal("PrepForFrontend-style mutation must not affect original")
	}
	cloned.PasskeyCredentials[0].Transport[0] = "nfc"
	if orig.PasskeyCredentials[0].Transport[0] != "usb" {
		t.Fatal("transport slice must be deep copied")
	}
}
