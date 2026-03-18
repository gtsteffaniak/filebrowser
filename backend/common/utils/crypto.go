package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

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
