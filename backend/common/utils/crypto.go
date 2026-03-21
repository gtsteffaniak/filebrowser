package utils

import (
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
