package main

import (
	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/cmd"
)

func main() {
	jwt.DecodeStrict = true
	cmd.StartFilebrowser()
}
