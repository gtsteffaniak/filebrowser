package main

import (
	"github.com/gtsteffaniak/filebrowser/cmd"
	"github.com/gtsteffaniak/filebrowser/settings"
)

func main() {
	settings.Initialize()
	cmd.StartFilebrowser()
}
