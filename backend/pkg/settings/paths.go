package settings

import (
	"fmt"
	"os"
	"path/filepath"
)

// ExpandTilde replaces a leading ~ or ~/ with the user home directory.
func ExpandTilde(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to expand home directory: %w", err)
	}

	if path == "~" {
		return homeDir, nil
	}

	if path[1] == '/' || path[1] == '\\' {
		return filepath.Join(homeDir, path[2:]), nil
	}

	return path, nil
}

// AbsPath expands a leading tilde, then resolves the path to an absolute path.
func AbsPath(path string) (string, error) {
	expanded, err := ExpandTilde(path)
	if err != nil {
		return "", err
	}
	return filepath.Abs(expanded)
}
