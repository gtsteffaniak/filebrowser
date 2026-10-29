package users

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DarkMode is the user's theme preference: "auto" (follow the client's
// system preference), "dark", or "light". It unmarshals from legacy JSON
// booleans for backward compatibility with existing stored users.
type DarkMode string

const (
	DarkModeAuto  DarkMode = "auto"
	DarkModeDark  DarkMode = "dark"
	DarkModeLight DarkMode = "light"
)

// UnmarshalJSON accepts legacy boolean values (true -> dark, false -> light),
// the tri-state strings ("auto", "dark", "light"), and null (unset).
func (d *DarkMode) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch val := v.(type) {
	case nil:
		*d = ""
	case bool:
		if val {
			*d = DarkModeDark
		} else {
			*d = DarkModeLight
		}
	case string:
		switch DarkMode(strings.ToLower(val)) {
		case "":
			*d = ""
		case "true":
			// Some YAML decoders cast scalar booleans to strings.
			*d = DarkModeDark
		case "false":
			*d = DarkModeLight
		case DarkModeAuto:
			*d = DarkModeAuto
		case DarkModeDark:
			*d = DarkModeDark
		case DarkModeLight:
			*d = DarkModeLight
		default:
			return fmt.Errorf("invalid darkMode value %q: must be \"auto\", \"dark\" or \"light\"", val)
		}
	default:
		return fmt.Errorf("invalid darkMode value: must be a boolean or one of \"auto\", \"dark\", \"light\"")
	}
	return nil
}

// IsDark reports the effective boolean value for server-side rendering
// contexts. "auto" cannot be resolved server-side; callers that can know the
// client preference should handle DarkModeAuto explicitly.
func (d DarkMode) IsDark() bool {
	return d == DarkModeDark
}

// Or returns d unless it is unset (""), in which case it returns def.
func (d DarkMode) Or(def DarkMode) DarkMode {
	if d == "" {
		return def
	}
	return d
}
