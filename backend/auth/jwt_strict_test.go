package auth

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

const base64URLAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

func decodeJWTBase64URL(s string) ([]byte, error) {
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}

// equivalentSpelling returns a different compact JWT string with the same decoded signature bytes.
func equivalentSpelling(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		panic("equivalentSpelling: invalid JWT")
	}
	header, payload, signature := parts[0], parts[1], parts[2]
	orig, err := decodeJWTBase64URL(signature)
	if err != nil {
		panic("equivalentSpelling: decode canonical signature: " + err.Error())
	}

	last := signature[len(signature)-1]
	for _, c := range base64URLAlphabet {
		if byte(c) == last {
			continue
		}
		trial := signature[:len(signature)-1] + string(c)
		dec, err := decodeJWTBase64URL(trial)
		if err != nil || !bytes.Equal(dec, orig) {
			continue
		}
		respelled := header + "." + payload + "." + trial
		if respelled != token {
			return respelled
		}
	}
	panic("equivalentSpelling: no alternate spelling found")
}

func invalidSignatureSpelling(token string) string {
	parts := strings.Split(token, ".")
	header, payload, signature := parts[0], parts[1], parts[2]
	idx := strings.IndexByte(base64URLAlphabet, signature[len(signature)-1])
	replacement := base64URLAlphabet[((idx&0b110000)+16)%64]
	return header + "." + payload + "." + signature[:len(signature)-1] + string(replacement)
}

func parseSessionToken(tokenString string) error {
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		return []byte(settings.Config.Auth.Key), nil
	}
	var tk users.AuthToken
	_, err := jwt.ParseWithClaims(tokenString, &tk, keyFunc)
	return err
}

func TestDecodeStrictRejectsEquivalentSpelling(t *testing.T) {
	origKey := settings.Config.Auth.Key
	origStrict := jwt.DecodeStrict
	settings.Config.Auth.Key = "test-signing-key-strict-decode"
	jwt.DecodeStrict = true
	t.Cleanup(func() {
		settings.Config.Auth.Key = origKey
		jwt.DecodeStrict = origStrict
	})

	user := &users.User{ID: 1, Username: "alice"}
	canonical, _, err := MakeSignedTokenAPI(user, "WEB_TOKEN_test", time.Hour, user.Permissions, false)
	if err != nil {
		t.Fatalf("MakeSignedTokenAPI: %v", err)
	}

	if err := parseSessionToken(canonical); err != nil {
		t.Fatalf("canonical token should parse under strict mode: %v", err)
	}

	respelled := equivalentSpelling(canonical)
	if err := parseSessionToken(respelled); err == nil {
		t.Fatal("equivalent spelling should be rejected when jwt.DecodeStrict is true")
	}

	tampered := invalidSignatureSpelling(canonical)
	if err := parseSessionToken(tampered); err == nil {
		t.Fatal("tampered signature should be rejected")
	}
}

func TestEquivalentSpellingParsesWithoutStrictMode(t *testing.T) {
	origKey := settings.Config.Auth.Key
	origStrict := jwt.DecodeStrict
	settings.Config.Auth.Key = "test-signing-key-strict-decode"
	jwt.DecodeStrict = false
	t.Cleanup(func() {
		settings.Config.Auth.Key = origKey
		jwt.DecodeStrict = origStrict
	})

	user := &users.User{ID: 1, Username: "alice"}
	canonical, _, err := MakeSignedTokenAPI(user, "WEB_TOKEN_test", time.Hour, user.Permissions, false)
	if err != nil {
		t.Fatalf("MakeSignedTokenAPI: %v", err)
	}
	respelled := equivalentSpelling(canonical)
	if err := parseSessionToken(respelled); err != nil {
		t.Fatalf("equivalent spelling should parse without strict mode (documents pre-fix behavior): %v", err)
	}
}
