package web

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestIndexTemplatePath_DevMode(t *testing.T) {
	prev := settings.Env.IsDevMode
	settings.Env.IsDevMode = true
	t.Cleanup(func() { settings.Env.IsDevMode = prev })

	if got := indexTemplatePath(); got != "index.html" {
		t.Fatalf("indexTemplatePath() = %q, want index.html", got)
	}
}

func TestIndexTemplatePath_Production(t *testing.T) {
	prev := settings.Env.IsDevMode
	settings.Env.IsDevMode = false
	t.Cleanup(func() { settings.Env.IsDevMode = prev })

	if got := indexTemplatePath(); got != "public/index.html" {
		t.Fatalf("indexTemplatePath() = %q, want public/index.html", got)
	}
}
