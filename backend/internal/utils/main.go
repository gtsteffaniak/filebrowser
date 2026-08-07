package utils

import (
	"cmp"
	"fmt"
	math "math/rand"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
)

func CheckErr(source string, err error) {
	if err != nil {
		logger.Fatalf("%s: %v", source, err)
	}
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

// Clamp returns value clamped between min and max.
// If value < min, returns min.
// If value > max, returns max.
// Otherwise, returns value.
func Clamp[T cmp.Ordered](value, min, max T) T {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func CheckPathExists(realPath string) bool {
	if _, err := os.Stat(realPath); os.IsNotExist(err) {
		return false
	}
	return true
}
