package users

// StoreToken indexes a token by both its display name and raw JWT string for O(1) lookup either way.
func StoreToken(tokens map[string]AuthToken, token AuthToken) {
	if tokens == nil || token.Name == "" {
		return
	}
	tokens[token.Name] = token
	if token.Token != "" && token.Token != token.Name {
		tokens[token.Token] = token
	}
}

// RemoveTokenByName deletes both the name key and raw-token key for a stored API token.
func RemoveTokenByName(tokens map[string]AuthToken, name string) {
	if tokens == nil {
		return
	}
	tok, ok := tokens[name]
	if !ok {
		return
	}
	delete(tokens, name)
	if tok.Token != "" {
		delete(tokens, tok.Token)
	}
}

// IndexTokensForLookup adds raw-token keys for tokens loaded from persistence (name-keyed only).
func IndexTokensForLookup(tokens map[string]AuthToken) {
	if len(tokens) == 0 {
		return
	}
	for key, tok := range tokens {
		if key != tok.Name {
			continue
		}
		if tok.Token != "" && tok.Token != key {
			tokens[tok.Token] = tok
		}
	}
}

// TokensForPersist returns only name-keyed entries suitable for JSON storage.
func TokensForPersist(tokens map[string]AuthToken) map[string]AuthToken {
	if len(tokens) == 0 {
		return nil
	}
	out := make(map[string]AuthToken)
	for key, tok := range tokens {
		if tok.Name != "" && key == tok.Name {
			out[key] = tok
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// TokenNameByRaw returns the token name when raw matches a stored JWT key.
func TokenNameByRaw(tokens map[string]AuthToken, raw string) (string, bool) {
	if tokens == nil || raw == "" {
		return "", false
	}
	tok, ok := tokens[raw]
	if !ok || tok.Name == "" {
		return "", false
	}
	return tok.Name, true
}

// EachNamedToken invokes fn for every token keyed by name (skips raw JWT alias keys).
func EachNamedToken(tokens map[string]AuthToken, fn func(name string, token AuthToken)) {
	if tokens == nil || fn == nil {
		return
	}
	for key, tok := range tokens {
		if key != tok.Name {
			continue
		}
		fn(key, tok)
	}
}
