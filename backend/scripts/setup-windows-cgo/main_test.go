package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWindowsCCompilerRespectsCC(t *testing.T) {
	t.Setenv("CC", "custom-cc -flag")
	cc, mupdf := resolveWindowsCCompiler()
	if cc != "custom-cc -flag" || !mupdf {
		t.Fatalf("resolveWindowsCCompiler() = (%q, %v), want custom CC with mupdf", cc, mupdf)
	}
}

func TestWriteBatchFilesWithoutMupdf(t *testing.T) {
	dir := t.TempDir()
	if err := writeBatchFiles(dir, "", false); err != nil {
		t.Fatal(err)
	}

	env, err := os.ReadFile(filepath.Join(dir, "windows-cgo-env.bat"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(env)
	if !strings.Contains(content, "set CGO_ENABLED=0") {
		t.Fatalf("expected CGO_ENABLED=0, got:\n%s", content)
	}
	if strings.Contains(content, "--tags=mupdf") {
		t.Fatalf("did not expect mupdf tags in env batch:\n%s", content)
	}

	build, err := os.ReadFile(filepath.Join(dir, "windows-air-build.bat"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(build), "%MUPDF_BUILD_TAGS%") {
		t.Fatalf("build batch should reference MUPDF_BUILD_TAGS:\n%s", build)
	}
}

func TestWriteBatchFilesWithMupdf(t *testing.T) {
	dir := t.TempDir()
	if err := writeBatchFiles(dir, `zig cc -target x86_64-windows-gnu -lc`, true); err != nil {
		t.Fatal(err)
	}

	env, err := os.ReadFile(filepath.Join(dir, "windows-cgo-env.bat"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(env)
	if !strings.Contains(content, "set CGO_ENABLED=1") {
		t.Fatalf("expected CGO_ENABLED=1, got:\n%s", content)
	}
	if !strings.Contains(content, `set "CC=zig cc -target x86_64-windows-gnu -lc"`) {
		t.Fatalf("expected zig CC, got:\n%s", content)
	}
	if !strings.Contains(content, "set MUPDF_BUILD_TAGS=--tags=mupdf") {
		t.Fatalf("expected mupdf build tags, got:\n%s", content)
	}
}
