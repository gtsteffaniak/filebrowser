package settings

import (
	"crypto/rand"
	"strings"

	"github.com/gtsteffaniak/filebrowser/rules"
)

const DefaultUsersHomeBasePath = "/users"

// AuthMethod describes an authentication method.
type AuthMethod string

// Settings contain the main settings of the application.
// GetRules implements rules.Provider.
func (s *Settings) GetRules() []rules.Rule {
	return s.Rules
}

// Server specific settings
// Clean cleans any variables that might need cleaning.
func (s *Server) Clean() {
	s.BaseURL = strings.TrimSuffix(s.BaseURL, "/")
}

// GenerateKey generates a key of 512 bits.
func GenerateKey() ([]byte, error) {
	b := make([]byte, 64) //nolint:gomnd
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}
