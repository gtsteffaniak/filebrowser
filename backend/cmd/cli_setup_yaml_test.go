package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSetupConfigYAML(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadSetupConfigTemplateYAML_fromDist(t *testing.T) {
	root := t.TempDir()
	writeSetupConfigYAML(t, root, "internal/web/dist/config.generated.yaml", "dist-content")
	t.Chdir(root)

	got, err := readSetupConfigTemplateYAML()
	if err != nil {
		t.Fatal(err)
	}
	if got != "dist-content" {
		t.Fatalf("readSetupConfigTemplateYAML() = %q, want dist-content", got)
	}
}

func TestReadSetupConfigTemplateYAML_prefersFrontendPublic(t *testing.T) {
	root := t.TempDir()
	writeSetupConfigYAML(t, root, "frontend/public/config.generated.yaml", "from-public")
	writeSetupConfigYAML(t, root, "internal/web/dist/config.generated.yaml", "from-dist")
	t.Chdir(root)

	got, err := readSetupConfigTemplateYAML()
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-public" {
		t.Fatalf("readSetupConfigTemplateYAML() = %q, want from-public", got)
	}
}
