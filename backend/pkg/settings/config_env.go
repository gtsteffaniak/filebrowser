package settings

import (
	"os"
	"strconv"
	"strings"
)

// expandConfigEnv walks a decoded YAML value tree and substitutes $VAR / ${VAR}
// in string scalars using the process environment. Non-string scalars (numbers,
// booleans, null) are left unchanged so typed config fields stay intact.
// Expansion happens after YAML decode so secrets may contain quotes, backslashes,
// or newlines without breaking the document.
// Maps and slices are mutated in place.
func expandConfigEnv(v interface{}) {
	switch x := v.(type) {
	case map[string]interface{}:
		for k, val := range x {
			x[k] = expandConfigEnvValue(val)
		}
	case []interface{}:
		for i, val := range x {
			x[i] = expandConfigEnvValue(val)
		}
	}
}

func expandConfigEnvValue(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		expandConfigEnv(x)
		return x
	case []interface{}:
		expandConfigEnv(x)
		return x
	case string:
		return expandConfigEnvString(x)
	default:
		// int, float, bool, nil, etc.
		return v
	}
}

// expandConfigEnvString expands environment references in a YAML string scalar.
// When the entire scalar is a single ${VAR} or $VAR reference, the expanded
// value is coerced to bool/int/float when unambiguous so config fields of those
// types keep working (e.g. port: ${PORT}). Partial expansions and ordinary
// strings always remain strings so secrets like "true" or "123" stay safe.
func expandConfigEnvString(s string) interface{} {
	if !strings.ContainsAny(s, "$") {
		return s
	}
	expanded := os.ExpandEnv(s)
	if isSingleEnvReference(s) {
		if coerced, ok := coerceEnvScalar(expanded); ok {
			return coerced
		}
	}
	return expanded
}

func isSingleEnvReference(s string) bool {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		inner := s[2 : len(s)-1]
		return isEnvName(inner)
	}
	if strings.HasPrefix(s, "$") && !strings.HasPrefix(s, "${") {
		return isEnvName(s[1:])
	}
	return false
}

func isEnvName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && r != '_' {
				return false
			}
			continue
		}
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func coerceEnvScalar(s string) (interface{}, bool) {
	switch s {
	case "true", "True", "TRUE":
		return true, true
	case "false", "False", "FALSE":
		return false, true
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i, true
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, true
	}
	return nil, false
}
