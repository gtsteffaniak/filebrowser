package utils

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestCSPNonceIsURLSafeUnpadded(t *testing.T) {
	for i := 0; i < 200; i++ {
		nonce, err := CSPNonce()
		if err != nil {
			t.Fatalf("CSPNonce: %v", err)
		}
		if len(nonce) != 22 {
			t.Fatalf("expected 22-char nonce for 16 random bytes, got %q (%d chars)", nonce, len(nonce))
		}
		if strings.ContainsAny(nonce, "+/=") {
			t.Fatalf("nonce %q must not contain '+', '/' or '=' (html/template escapes '+' as &#43;)", nonce)
		}
		raw, err := base64.RawURLEncoding.DecodeString(nonce)
		if err != nil {
			t.Fatalf("nonce %q is not valid RawURLEncoding: %v", nonce, err)
		}
		if len(raw) != 16 {
			t.Fatalf("expected 16 decoded bytes, got %d", len(raw))
		}
	}
}

func TestCSPNonceFromDataReusesExisting(t *testing.T) {
	data := map[string]interface{}{"cspNonce": "existing-nonce"}
	nonce, err := CSPNonceFromData(data)
	if err != nil {
		t.Fatalf("CSPNonceFromData: %v", err)
	}
	if nonce != "existing-nonce" {
		t.Fatalf("expected existing nonce to be reused, got %q", nonce)
	}

	fresh := map[string]interface{}{}
	nonce, err = CSPNonceFromData(fresh)
	if err != nil {
		t.Fatalf("CSPNonceFromData: %v", err)
	}
	if nonce == "" {
		t.Fatal("expected a generated nonce")
	}
	if stored, ok := fresh["cspNonce"].(string); !ok || stored != nonce {
		t.Fatalf("expected generated nonce to be stored back, got %v", fresh["cspNonce"])
	}
}
