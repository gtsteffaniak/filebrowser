package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// RandomHex returns a hex-encoded string from byteLen bytes of crypto/rand (length 2*byteLen).
func RandomHex(byteLen int) (string, error) {
	b, err := randomBytes(byteLen)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

var BcryptCost = bcrypt.DefaultCost
var InvalidPasswordHash = ""

// HashPwd hashes a password.
func HashPwd(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	return string(bytes), err
}

// CheckPwd checks if a password is correct.
func CheckPwd(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func SetInvalidPasswordHash() error {
	passwordHash, err := HashPwd(InsecureRandomIdentifier(16))
	if err != nil {
		return err
	}
	InvalidPasswordHash = passwordHash
	return nil
}

func HashSHA256(data string) string {
	bytes := sha256.Sum256([]byte(data))
	return hex.EncodeToString(bytes[:])
}

func GenerateKey() string {
	b := make([]byte, 64)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	return string(b)
}

// CSPNonce returns a base64-encoded random value suitable for Content-Security-Policy nonces
// and matching HTML nonce="" attributes (cryptographically random, URL/header safe characters).
func CSPNonce() (string, error) {
	b, err := randomBytes(16)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
