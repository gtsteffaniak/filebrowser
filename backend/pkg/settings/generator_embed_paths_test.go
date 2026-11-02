package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func writeGeneratedConfigYAML(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadEmbeddedYaml_fromDist(t *testing.T) {
	root := t.TempDir()
	writeGeneratedConfigYAML(t, root, "internal/web/dist/config.generated.yaml", "dist-content")
	t.Chdir(root)

	got, err := readEmbeddedYaml()
	if err != nil {
		t.Fatal(err)
	}
	if got != "dist-content" {
		t.Fatalf("readEmbeddedYaml() = %q, want dist-content", got)
	}
}

func TestReadEmbeddedYaml_prefersRepoRootDist(t *testing.T) {
	root := t.TempDir()
	writeGeneratedConfigYAML(t, root, "internal/web/dist/config.generated.yaml", "from-internal")
	writeGeneratedConfigYAML(t, root, "dist/config.generated.yaml", "from-cwd")
	t.Chdir(root)

	got, err := readEmbeddedYaml()
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-internal" {
		t.Fatalf("readEmbeddedYaml() = %q, want from-internal", got)
	}
}
