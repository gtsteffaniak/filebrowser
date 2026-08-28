package utils

import "testing"

func TestCSPNonceFromData_ReusesExistingNonce(t *testing.T) {
	data := map[string]interface{}{
		"cspNonce": "existing-nonce",
	}
	nonce, err := CSPNonceFromData(data)
	if err != nil {
		t.Fatalf("CSPNonceFromData: %v", err)
	}
	if nonce != "existing-nonce" {
		t.Fatalf("nonce = %q, want existing-nonce", nonce)
	}
}

func TestCSPNonceFromData_GeneratesAndStoresMissingNonce(t *testing.T) {
	data := map[string]interface{}{}
	nonce, err := CSPNonceFromData(data)
	if err != nil {
		t.Fatalf("CSPNonceFromData: %v", err)
	}
	if nonce == "" {
		t.Fatal("expected generated nonce")
	}
	stored, ok := data["cspNonce"].(string)
	if !ok || stored != nonce {
		t.Fatalf("stored nonce = %q, want %q", stored, nonce)
	}

	reused, err := CSPNonceFromData(data)
	if err != nil {
		t.Fatalf("CSPNonceFromData reuse: %v", err)
	}
	if reused != nonce {
		t.Fatalf("reused nonce = %q, want %q", reused, nonce)
	}
}

func TestCSPNonceFromData_NonMapGeneratesWithoutMutation(t *testing.T) {
	type pageData struct {
		Title string
	}
	data := pageData{Title: "test"}
	nonce, err := CSPNonceFromData(data)
	if err != nil {
		t.Fatalf("CSPNonceFromData: %v", err)
	}
	if nonce == "" {
		t.Fatal("expected generated nonce for non-map data")
	}
}
