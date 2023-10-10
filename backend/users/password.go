package users

import (
	"log"

	"golang.org/x/crypto/bcrypt"
)

// HashPwd hashes a password.
func HashPwd(password string) (string, error) {
	log.Println("hashing password", password)
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPwd checks if a password is correct.
func CheckPwd(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
