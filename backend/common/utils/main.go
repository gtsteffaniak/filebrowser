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
