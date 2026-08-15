// Setup go-fitz MuPDF headers for preview CGO (-tags=mupdf).
// Run from the backend module root: go run ./scripts/setup-gofitz-cgo
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() {
	backendDir, err := backendRoot()
	if err != nil {
		fatal(err)
	}

	outPath := filepath.Join(backendDir, "internal", "preview", "gofitzinclude")
	includeDir, err := goFitzIncludeDir(backendDir)
	if err != nil {
		fatal(err)
	}

	if err := removeExisting(outPath); err != nil {
		fatal(err)
	}
	if err := linkInclude(includeDir, outPath); err != nil {
		fatal(err)
	}

	fmt.Println("linked go-fitz MuPDF headers at", outPath)
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

func goFitzIncludeDir(backendDir string) (string, error) {
	cmd := exec.Command("go", "mod", "download", "-json", "github.com/gen2brain/go-fitz")
	cmd.Dir = backendDir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("go mod download go-fitz: %w: %s", err, ee.Stderr)
		}
		return "", fmt.Errorf("go mod download go-fitz: %w", err)
	}

	var modInfo struct {
		Dir string `json:"Dir"`
	}
	if err := json.Unmarshal(out, &modInfo); err != nil {
		return "", fmt.Errorf("parse go mod download output: %w", err)
	}
	if modInfo.Dir == "" {
		return "", fmt.Errorf("go-fitz module path not found after download")
	}

	include := filepath.Join(modInfo.Dir, "include")
	if fi, err := os.Stat(include); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("go-fitz include directory not found: %s", include)
	}
	return include, nil
}

func removeExisting(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return os.Remove(path)
	}
	if runtime.GOOS == "windows" {
		// Junction points are removed with rmdir without deleting the target directory.
		if err := exec.Command("cmd", "/c", "rmdir", path).Run(); err == nil {
			return nil
		}
	}
	return os.RemoveAll(path)
}

func linkInclude(src, dst string) error {
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	absDst, err := filepath.Abs(dst)
	if err != nil {
		return err
	}

	if runtime.GOOS != "windows" {
		return os.Symlink(absSrc, absDst)
	}

	if err := os.Symlink(absSrc, absDst); err == nil {
		return nil
	}
	if err := exec.Command("cmd", "/c", "mklink", "/J", absDst, absSrc).Run(); err == nil {
		return nil
	}
	return copyDir(absSrc, absDst)
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err = os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
