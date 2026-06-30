package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
)

func CheckErr(source string, err error) {
	if err != nil {
		logger.Fatalf("%s: %v", source, err)
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
	result := make([]byte, length)
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		// fallback: return hex-encoded random bytes
		b := make([]byte, length)
		_, _ = rand.Read(b)
		return hex.EncodeToString(b)[:length]
	}
	for i := range result {
		result[i] = charset[int(buf[i])%len(charset)]
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

		logger.Debugf("Field: %s, %s\n", fieldType.Name, fieldValue)
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

// SafeScopedJoin joins a trusted scope prefix with an untrusted, user-supplied relative
// path and guarantees the result cannot escape the scope via ".." traversal. It returns
// an error if the cleaned path would resolve outside the scope. Use this anywhere a
// request-supplied path is combined with a user scope or a share root.
func SafeScopedJoin(scope, userPath string) (string, error) {
	cleanScope := filepath.Clean("/" + strings.Trim(strings.ReplaceAll(scope, "\\", "/"), "/"))
	joined := filepath.Clean(filepath.Join(cleanScope, filepath.FromSlash(userPath)))
	if runtime.GOOS == "windows" {
		joined = strings.ReplaceAll(joined, "\\", "/")
		cleanScope = strings.ReplaceAll(cleanScope, "\\", "/")
	}
	if joined != cleanScope && !strings.HasPrefix(joined, strings.TrimRight(cleanScope, "/")+"/") {
		return "", fmt.Errorf("path %q escapes its permitted scope", userPath)
	}
	return joined, nil
}

// WithinRoot reports whether the resolved candidate path is the root itself or lies
// beneath it. Both paths should be absolute. Used to confirm a symlink-resolved real
// path has not escaped its source root.
func WithinRoot(root, candidate string) bool {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	if candidate == root {
		return true
	}
	sep := string(filepath.Separator)
	return strings.HasPrefix(candidate, strings.TrimRight(root, sep)+sep)
}

func NonNilSlice[T any](in []T) []T {
	if in == nil {
		return []T{}
	}
	return in
}

func Ternary[T any](cond bool, vtrue, vfalse T) T {
	if cond {
		return vtrue
	}
	return vfalse
}

// NormalizeRulePath ensures directory paths have trailing slashes for consistent rule storage
func AddTrailingSlashIfNotExists(indexPath string) string {
	// Root path stays as "/"
	if indexPath == "/" {
		return "/"
	}
	// For all other paths, ensure they have trailing slashes
	if !strings.HasSuffix(indexPath, "/") {
		return indexPath + "/"
	}
	return indexPath
}

func CheckPathExists(realPath string) bool {
	if _, err := os.Stat(realPath); os.IsNotExist(err) {
		return false
	}
	return true
}
