package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	math "math/rand"
	"reflect"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/logger"
)

func CheckErr(source string, err error) {
	if err != nil {
		logger.Fatal(fmt.Sprintf("%s: %v", source, err))
	}
}

func GenerateKey() []byte {
	b := make([]byte, 64)
	_, err := rand.Read(b)
	if err != nil {
		return nil
	}
	return b
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
