package rifs

import (
	"os"
	"path"
)

var (
	appPath string
)

func init() {
	goPath := os.Getenv("GOPATH")
	appPath = path.Join(goPath, "src", "github.com", "dsoprea", "go-utility", "filesystem", "v2")
}
