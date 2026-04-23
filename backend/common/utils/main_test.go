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

func TestIndexPathParent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want string
	}{
		{"/Downloads/33-fbx", "Downloads"},
		{"Downloads/33-fbx", "Downloads"},
		{`Downloads\33-fbx`, "Downloads"},
		// Simulates filepath.Clean("/Downloads/33-fbx") on Windows (root-relative path)
		{`\Downloads\33-fbx`, "Downloads"},
		{"Downloads", "/"},
		{"a", "/"},
		{"", "/"},
		{"/a/b/c", "a/b"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got := IndexPathParent(tt.in)
			if got != tt.want {
				t.Fatalf("IndexPathParent(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIndexPathBase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want string
	}{
		{"/Downloads/33-fbx", "33-fbx"},
		{"Downloads/33-fbx", "33-fbx"},
		{`\Downloads\33-fbx`, "33-fbx"},
		{"archive.zip", "archive.zip"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got := IndexPathBase(tt.in)
			if got != tt.want {
				t.Fatalf("IndexPathBase(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
