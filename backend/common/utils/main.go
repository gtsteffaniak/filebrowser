package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	math "math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
)

func CheckErr(source string, err error) {
	if err != nil {
		logger.Fatal(fmt.Sprintf("%s: %v", source, err))
	}
}

func GenerateKey() string {
	b := make([]byte, 64)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	return string(b)
}

// CapitalizeFirst returns the input string with the first letter capitalized.
func CapitalizeFirst(s string) string {
	if len(s) == 0 {
		return s // Return the empty string as is
	}
	return strings.ToUpper(string(s[0])) + s[1:]
}

func InsecureRandomIdentifier(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	math.New(math.NewSource(time.Now().UnixNano()))
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[math.Intn(len(charset))]
	}
	return string(result)
}

func PrintStructFields(v interface{}) {
	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	// Ensure the input is a struct
	if val.Kind() != reflect.Struct {
		logger.Debug("Provided value is not a struct")
		return
	}

	// Iterate over the fields of the struct
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Convert field value to string, if possible
		fieldValue := fmt.Sprintf("%v", field.Interface())

		// Limit to 50 characters
		if len(fieldValue) > 100 {
			fieldValue = fieldValue[:100] + "..."
		}

		logger.Debug(fmt.Sprintf("Field: %s, %s\n", fieldType.Name, fieldValue))
	}
}

func GetParentDirectoryPath(path string) string {
	if path == "/" || path == "" {
		return ""
	}
	path = strings.TrimSuffix(path, "/") // Remove trailing slash if any
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash == -1 {
		return "" // No parent directory for a relative path without slashes
	}
	if lastSlash == 0 {
		return "/" // If the last slash is the first character, return root
	}
	return path[:lastSlash]
}

func HashSHA256(data string) string {
	bytes := sha256.Sum256([]byte(data))
	return hex.EncodeToString(bytes[:])
}

func GetLastComponent(path string) string {
	if path == "" {
		return ""
	}
	path = strings.TrimSuffix(path, "/") // Remove trailing slash if any
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash == -1 {
		return path // No parent directory for a relative path without slashes
	}
	return path[lastSlash+1:]
}

func JoinPathAsUnix(parts ...string) string {
	joinedPath := filepath.Join(parts...)
	if runtime.GOOS == "windows" {
		joinedPath = strings.ReplaceAll(joinedPath, "\\", "/")
	}
	return joinedPath
}

// resolveSymlinks resolves symlinks in the given path
func ResolveSymlinks(path string) (string, bool, error) {
	for {
		// Get the file info using os.Lstat to handle symlinks
		info, err := os.Lstat(path)
		if err != nil {
			return path, false, fmt.Errorf("could not stat path: %s, %v", path, err)
		}

		// Check if the path is a symlink
		if info.Mode()&os.ModeSymlink != 0 {
			// Read the symlink target
			target, err := os.Readlink(path)
			if err != nil {
				return path, false, fmt.Errorf("could not read symlink: %s, %v", path, err)
			}

			// Resolve the symlink's target relative to its directory
			// This ensures the resolved path is absolute and correctly calculated
			path = filepath.Join(filepath.Dir(path), target)
		} else {
			// Not a symlink, so return the resolved path and whether it's a directory
			isDir := info.IsDir()
			return path, isDir, nil
		}
	}
}
