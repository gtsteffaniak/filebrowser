// Setup Windows CGO toolchain for air dev builds (-tags=mupdf).
// Writes tmp/windows-*.bat used by .air.windows.toml.
// Falls back to a CGO-free build when only incompatible MinGW (e.g. Scoop MSVCRT) is present.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	ldflags = `-w -s -X github.com/gtsteffaniak/filebrowser/backend/internal/version.CommitSHA=commitSHA -X github.com/gtsteffaniak/filebrowser/backend/internal/version.Version=version`
)

func main() {
	if runtime.GOOS != "windows" {
		return
	}

	backendDir, err := backendRoot()
	if err != nil {
		fatal(err)
	}

	tmpDir := filepath.Join(backendDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		fatal(err)
	}

	cc, mupdf := resolveWindowsCCompiler()
	if err := writeBatchFiles(tmpDir, cc, mupdf); err != nil {
		fatal(err)
	}

	if mupdf {
		fmt.Printf("Windows dev build: MuPDF enabled (CC=%s)\n", cc)
		return
	}

	fmt.Println("Windows dev build: MuPDF disabled (no UCRT-capable C compiler found).")
	fmt.Println("Install zig (scoop install zig) or MSYS2 UCRT gcc for PDF preview support:")
	fmt.Println("  https://packages.msys2.org/packages/mingw-w64-ucrt-x86_64-gcc")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func backendRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
		return wd, nil
	}
	backend := filepath.Join(wd, "backend")
	if _, err := os.Stat(filepath.Join(backend, "go.mod")); err == nil {
		return backend, nil
	}
	return "", fmt.Errorf("could not find backend module root from %s", wd)
}

func resolveWindowsCCompiler() (cc string, mupdf bool) {
	if cc := strings.TrimSpace(os.Getenv("CC")); cc != "" {
		return cc, true
	}

	if _, err := exec.LookPath("zig"); err == nil {
		return "zig cc -target x86_64-windows-gnu -lc", true
	}

	for _, candidate := range []string{
		`C:\msys64\ucrt64\bin\gcc.exe`,
		`C:\msys64\clang64\bin\clang.exe`,
	} {
		if fileExists(candidate) {
			return candidate, true
		}
	}

	if gcc, err := exec.LookPath("gcc"); err == nil {
		lower := strings.ToLower(gcc)
		if strings.Contains(lower, "ucrt") || strings.Contains(lower, "llvm-mingw") {
			return gcc, true
		}
	}

	return "", false
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func writeBatchFiles(tmpDir, cc string, mupdf bool) error {
	envBat := filepath.Join(tmpDir, "windows-cgo-env.bat")
	buildBat := filepath.Join(tmpDir, "windows-air-build.bat")
	runBat := filepath.Join(tmpDir, "windows-air-run.bat")

	var envLines []string
	envLines = append(envLines, "@echo off")
	if mupdf {
		envLines = append(envLines,
			"set CGO_ENABLED=1",
			fmt.Sprintf(`set "CC=%s"`, cc),
			"set MUPDF_BUILD_TAGS=--tags=mupdf",
		)
	} else {
		envLines = append(envLines,
			"set CGO_ENABLED=0",
			"set CC=",
			"set MUPDF_BUILD_TAGS=",
		)
	}

	buildLines := []string{
		"@echo off",
		`call "%~dp0windows-cgo-env.bat"`,
		fmt.Sprintf("go build -o ./tmp/filebrowser.exe %%MUPDF_BUILD_TAGS%% --ldflags=\"%s\" .", ldflags),
		"if errorlevel 1 exit /b 1",
	}

	runLines := []string{
		"@echo off",
		`call "%~dp0windows-cgo-env.bat"`,
		"set FILEBROWSER_DEVMODE=true",
		".\\tmp\\filebrowser.exe -c test_config.yaml",
	}

	if err := os.WriteFile(envBat, []byte(strings.Join(envLines, "\r\n")+"\r\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(buildBat, []byte(strings.Join(buildLines, "\r\n")+"\r\n"), 0o644); err != nil {
		return err
	}
	return os.WriteFile(runBat, []byte(strings.Join(runLines, "\r\n")+"\r\n"), 0o644)
}
