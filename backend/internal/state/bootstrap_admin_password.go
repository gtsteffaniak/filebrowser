package state

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// bootstrapDefaultAdminPassword returns the plaintext password for initial admin creation.
// When config adminPassword is blank or "admin", a random speakable password is generated and logged once.
func bootstrapDefaultAdminPassword(username string) (plaintext string, generated bool, err error) {
	cfg := settings.Config.Auth.AdminPassword
	if cfg != "" && cfg != "admin" {
		return cfg, false, nil
	}

	plaintext, err = generateSpeakableBootstrapPassword()
	if err != nil {
		return "", false, fmt.Errorf("generate bootstrap admin password: %w", err)
	}
	logger.Infof(
		"Generated initial admin password for user %q (set auth.adminPassword in config to override on future resets): %s",
		username,
		plaintext,
	)
	return plaintext, true, nil
}

func generateSpeakableBootstrapPassword() (string, error) {
	if len(bootstrapWords) == 0 {
		return "", fmt.Errorf("bootstrap word list is empty")
	}
	wordIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(bootstrapWords))))
	if err != nil {
		return "", err
	}
	word := bootstrapWords[wordIdx.Int64()]

	primary, err := randomSpeakableCode(bootstrapSpeakablePrimaryCodeLen)
	if err != nil {
		return "", err
	}
	secondary, err := randomSpeakableCode(bootstrapSpeakableSecondaryCodeLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s-%s", word, string(primary), string(secondary)), nil
}

func randomSpeakableCode(n int) ([]byte, error) {
	codeRunes := make([]byte, n)
	charsetLen := big.NewInt(int64(len(bootstrapSpeakableCharset)))
	for i := range codeRunes {
		idx, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return nil, err
		}
		codeRunes[i] = bootstrapSpeakableCharset[idx.Int64()]
	}
	return codeRunes, nil
}
