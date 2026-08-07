package utils

import (
	"testing"
)

func TestGetParentDirectoryPath(t *testing.T) {
	tests := []struct {
		input          string
		expectedOutput string
	}{
		{input: "/", expectedOutput: ""},                                              // Root directory
		{input: "/subfolder", expectedOutput: "/"},                                    // Single subfolder
		{input: "/sub/sub/", expectedOutput: "/sub"},                                  // Nested subfolder with trailing slash
		{input: "/subfolder/", expectedOutput: "/"},                                   // Relative path with trailing slash
		{input: "", expectedOutput: ""},                                               // Empty string treated as root
		{input: "/sub/subfolder", expectedOutput: "/sub"},                             // Double slash in path
		{input: "/sub/subfolder/deep/nested/", expectedOutput: "/sub/subfolder/deep"}, // Double slash in path
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			actualOutput := GetParentDirectoryPath(test.input)
			if actualOutput != test.expectedOutput {
				t.Errorf("\n\tinput %q\n\texpected %q\n\tgot %q",
					test.input, test.expectedOutput, actualOutput)
			}
		})
	}
}

func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		input          string
		expectedOutput string
	}{
		{input: "", expectedOutput: ""},                               // Empty string
		{input: "a", expectedOutput: "A"},                             // Single lowercase letter
		{input: "A", expectedOutput: "A"},                             // Single uppercase letter
		{input: "hello", expectedOutput: "Hello"},                     // All lowercase
		{input: "Hello", expectedOutput: "Hello"},                     // Already capitalized
		{input: "123hello", expectedOutput: "123hello"},               // Non-alphabetic first character
		{input: "hELLO", expectedOutput: "HELLO"},                     // Mixed case
		{input: " hello", expectedOutput: " hello"},                   // Leading space, no capitalization
		{input: "hello world", expectedOutput: "Hello world"},         // Phrase with spaces
		{input: " hello world", expectedOutput: " hello world"},       // Phrase with leading space
		{input: "123 hello world", expectedOutput: "123 hello world"}, // Numbers before text
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			actualOutput := CapitalizeFirst(test.input)
			if actualOutput != test.expectedOutput {
				t.Errorf("\n\tinput %q\n\texpected %q\n\tgot %q",
					test.input, test.expectedOutput, actualOutput)
			}
		})
	}
}
