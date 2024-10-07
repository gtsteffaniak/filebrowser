package utils

import (
	"log"

	"github.com/gtsteffaniak/filebrowser/settings"
)

func CheckErr(source string, err error) {
	if err != nil {
		log.Fatalf("%s: %v", source, err)
	}
}

func GenerateKey() []byte {
	k, err := settings.GenerateKey()
	CheckErr("generateKey", err)
	return k
}
