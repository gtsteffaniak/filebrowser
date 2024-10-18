package utils

import (
	"crypto/rand"
	"log"
	"strings"
)

func CheckErr(source string, err error) {
	if err != nil {
		log.Fatalf("%s: %v", source, err)
	}
}

func GenerateKey() []byte {
	b := make([]byte, 64)
	_, err := rand.Read(b)
	if err != nil {
		return nil
	}
	return b
}

// CapitalizeFirst returns the input string with the first letter capitalized.
func CapitalizeFirst(s string) string {
	if len(s) == 0 {
		return s // Return the empty string as is
	}
	return strings.ToUpper(string(s[0])) + s[1:]
}
