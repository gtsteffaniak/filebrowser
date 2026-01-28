package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// Constants for TOTP (Time-based One-Time Password) configuration.
const (
	// IssuerName is the name of the application or service.
	IssuerName = "FileBrowser Quantum"

	// TokenValidTime defines the total duration a token is considered valid.
	// Note: The actual validation window might be slightly longer depending on the period.
	TokenValidTime = time.Minute * 2

	// TOTPPeriod is the standard duration in seconds that a TOTP code is valid.
	TOTPPeriod uint = 30

	// TOTPSecretSize is the byte length of the shared secret.
	TOTPSecretSize uint = 20

	// TOTPDigits specifies the number of digits in the OTP code.
	TOTPDigits = otp.DigitsSix

	// TOTPAlgorithm is the hashing algorithm to use.
	TOTPAlgorithm = otp.AlgorithmSHA1
)

var (
	// TOTPSkew allows for a certain number of periods of clock drift.
	// We calculate it to best match the TokenValidTime.
	// (2 * Skew + 1) * Period >= TokenValidTime
	// For a 2-minute (120s) window with a 30s period, we need a Skew of 2.
	// (2*2 + 1) * 30s = 150s (2.5 minutes). This is the closest we can get.
	TOTPSkew      uint = uint(TokenValidTime.Seconds()) / (2 * uint(TOTPPeriod))
	encryptionKey []byte
	TotpCache     = cache.NewCache[string](5 * time.Minute)
)

func GenerateOtpForUser(user *users.User, userStore *users.Storage) (string, error) {
	// Generate a new TOTP key using the defined constants.
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      IssuerName,
		AccountName: user.Username,
		Period:      TOTPPeriod,
		SecretSize:  TOTPSecretSize,
		Digits:      TOTPDigits,
		Algorithm:   TOTPAlgorithm,
	})
	if err != nil {
		return "", fmt.Errorf("error generating TOTP key: %w", err)
	}

	secretText := key.Secret()
	nonce := ""
	secretToStore := secretText
	if settings.Config.Auth.TotpSecret != "" {
		// If an encryption key is provided, encrypt the secret.
		secretToStore, nonce, err = encryptSecret(secretText, encryptionKey)
		if err != nil {
			return "", fmt.Errorf("failed to encrypt TOTP secret: %w", err)
		}
	}
	// set cache so verify can attempt to use it but not require it for user yet.
	TotpCache.Set(user.Username, secretToStore+"||"+nonce)
	// Use the original base32 secret in the OTP URL, not the encrypted version
	url := fmt.Sprintf("otpauth://totp/%v?secret=%v", "FileBrowser Quantum: "+user.Username, secretText)
	return url, nil
}

// encryptSecret uses AES-GCM to encrypt a plaintext secret.
// It returns the base64-encoded ciphertext and nonce, or an error.
func encryptSecret(secret string, key []byte) (string, string, error) {
	if len(key) != 32 {
		return "", "", fmt.Errorf("invalid encryption key length: must be 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(secret), nil)

	return base64.StdEncoding.EncodeToString(ciphertext), base64.StdEncoding.EncodeToString(nonce), nil
}

// decryptSecret uses AES-GCM to decrypt a ciphertext using its key and nonce.
// It returns the plaintext secret or an error.
func decryptSecret(b64Ciphertext, b64Nonce string) (string, error) {
	if len(encryptionKey) != 32 {
		return "", fmt.Errorf("invalid encryption key length: must be 32 bytes")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(b64Ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(b64Nonce)
	if err != nil {
		return "", fmt.Errorf("failed to decode nonce: %w", err)
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// This error often means the key is wrong or the data is corrupt
		return "", fmt.Errorf("failed to decrypt secret: %w", err)
	}

	return string(plaintext), nil
}

func VerifyTotpCode(user *users.User, code string, userStore *users.Storage) error {
	// get data from cache
	cachedSecret, found := TotpCache.Get(user.Username)
	if !found && user.TOTPSecret == "" {
		return fmt.Errorf("OTP token not found in cache, please generate a new one")
	}
	totpSecret := user.TOTPSecret // The encrypted or plaintext secret
	totpNonce := user.TOTPNonce   // The nonce if encrypted, or empty if plaintext
	if found {
		splitSecret := strings.Split(cachedSecret, "||")
		if len(splitSecret) < 2 {
			return fmt.Errorf("invalid cached OTP token format")
		}
		totpSecret = splitSecret[0]
		totpNonce = splitSecret[1]
	}
	secretToValidate := totpSecret
	if settings.Config.Auth.TotpSecret != "" {
		// If an encryption key is configured, we must decrypt the secret first.
		if totpNonce == "" {
			return fmt.Errorf("secret is encrypted but nonce is missing")
		}
		decryptedSecret, err := decryptSecret(totpSecret, totpNonce)
		if err != nil {
			return fmt.Errorf("failed to decrypt TOTP secret: %w", err)
		}
		secretToValidate = decryptedSecret
	}
	// --- END: ADD THIS DECRYPTION LOGIC ---

	// Validate the token using the (now plaintext) secret.
	valid, err := totp.ValidateCustom(code, secretToValidate, time.Now().UTC(), totp.ValidateOpts{
		Period:    TOTPPeriod,
		Skew:      TOTPSkew,
		Digits:    TOTPDigits,
		Algorithm: TOTPAlgorithm,
	})
	if err != nil {
		logger.Errorf("error during TOTP validation for user %s: %v", user.Username, err)
	}
	if !valid {
		logger.Warningf("Invalid TOTP code for user %s", user.Username)
		return fmt.Errorf("invalid OTP token")
	}
	if totpSecret != "" {
		user.TOTPSecret = totpSecret // The encrypted or plaintext secret
		user.TOTPNonce = totpNonce   // The nonce if encrypted, or empty if plaintext
		user.OtpEnabled = true       // Enable OTP for the user
		// save user
		if err := userStore.Update(user, true, "TOTPSecret", "TOTPNonce", "OtpEnabled"); err != nil {
			logger.Error("error updating user with OTP token:", err)
			return fmt.Errorf("error updating user with OTP token: %w", err)
		}
	} else {
		return fmt.Errorf("opt secret is empty, cannot enable TOTP")
	}

	return nil
}
