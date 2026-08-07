package users

import "testing"

func TestStoreTokenDualKeys(t *testing.T) {
	tokens := make(map[string]AuthToken)
	tok := AuthToken{Name: "ci-key", Token: "jwt-abc-123"}
	StoreToken(tokens, tok)

	if len(tokens) != 2 {
		t.Fatalf("expected 2 map keys, got %d", len(tokens))
	}
	if _, ok := tokens["ci-key"]; !ok {
		t.Fatal("missing name key")
	}
	if _, ok := tokens["jwt-abc-123"]; !ok {
		t.Fatal("missing raw token key")
	}

	name, ok := TokenNameByRaw(tokens, "jwt-abc-123")
	if !ok || name != "ci-key" {
		t.Fatalf("TokenNameByRaw: got (%q, %v)", name, ok)
	}
}

func TestRemoveTokenByNameClearsBothKeys(t *testing.T) {
	tokens := make(map[string]AuthToken)
	StoreToken(tokens, AuthToken{Name: "a", Token: "raw-a"})
	RemoveTokenByName(tokens, "a")
	if len(tokens) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(tokens))
	}
}

func TestIndexTokensForLookup(t *testing.T) {
	tokens := map[string]AuthToken{
		"my-key": {Name: "my-key", Token: "jwt-xyz"},
	}
	IndexTokensForLookup(tokens)
	if _, ok := tokens["jwt-xyz"]; !ok {
		t.Fatal("expected raw key after index")
	}
}

func TestTokensForPersistStripsRawKeys(t *testing.T) {
	tokens := make(map[string]AuthToken)
	StoreToken(tokens, AuthToken{Name: "n", Token: "raw"})
	persisted := TokensForPersist(tokens)
	if len(persisted) != 1 {
		t.Fatalf("expected 1 persisted entry, got %d", len(persisted))
	}
	if _, ok := persisted["raw"]; ok {
		t.Fatal("raw token key should not be persisted")
	}
}
