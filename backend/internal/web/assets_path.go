package web

import (
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func indexTemplatePath() string {
	if settings.Env.IsDevMode {
		return "index.html"
	}
	return "public/index.html"
}
