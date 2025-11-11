package settings

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Test structs for comprehensive testing
type TestConfig struct {
	PublicField     string     `json:"publicField"`     // A regular public field
	SecretField     string     `json:"secretField"`     // secret: this should be hidden
	DeprecatedField string     `json:"deprecatedField"` // deprecated: this field is old
	EmptyField      string     `json:"emptyField"`      // An empty string field
	DefaultField    string     `json:"defaultField"`    // A field that matches default
	NonDefaultField string     `json:"nonDefaultField"` // A field that differs from default
	NestedStruct    TestNested `json:"nestedStruct"`
}

type TestNested struct {
	NestedPublic     string `json:"nestedPublic"`     // A nested public field
	NestedSecret     string `json:"nestedSecret"`     // secret: nested secret field
	NestedDeprecated string `json:"nestedDeprecated"` // deprecated: nested deprecated field
}

// Helper to create test source files for comment parsing
func createTestSourceFile(t *testing.T, content string) (string, func()) {
	tmpFile, err := os.CreateTemp("", "test_*.go")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	cleanup := func() {
		os.Remove(tmpFile.Name())
	}

	return tmpFile.Name(), cleanup
}

func TestGenerateConfigYaml_StringQuoting(t *testing.T) {
	reNumber := regexp.MustCompile(`^-?\d+(\.\d+)?$`)
	// Create a simple test source file for this test
	sourceContent := `package settings

type Settings struct {
	UserDefaults UserDefaults ` + "`json:\"userDefaults\"`" + `
	Auth Auth ` + "`json:\"auth\"`" + `
}

type UserDefaults struct {
	Locale string ` + "`json:\"locale\"`" + `
}

type Auth struct {
	AdminUsername string ` + "`json:\"adminUsername\"`" + `
}
`
	tmpFile, cleanup := createTestSourceFile(t, sourceContent)
	defer cleanup()
	sourcePath := tmpFile[:strings.LastIndex(tmpFile, "/")]

	// Create real Settings with string values to test quoting
	settings := &Settings{
		UserDefaults: UserDefaults{
			Locale: "en-US",
		},
		Auth: Auth{
			AdminUsername: "admin",
		},
	}

	// Test with no filtering - should quote all strings
	yamlOutput, err := GenerateConfigYamlWithSource(settings, false, true, false, sourcePath)
	if err != nil {
		t.Fatalf("GenerateConfigYaml failed: %v", err)
	}

	// All string values should be quoted
	lines := strings.Split(yamlOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, ":") && !strings.HasSuffix(strings.TrimSpace(line), ":") {
			// This is a key-value line
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])

				// Remove comments
				if commentIdx := strings.Index(value, "#"); commentIdx >= 0 {
					value = strings.TrimSpace(value[:commentIdx])
				}

				// Skip empty values, arrays, objects, booleans
				if value == "" || value == "false" || value == "true" || strings.HasPrefix(value, "[") || strings.HasPrefix(value, "{") {
					continue
				}

				// Check if it's a number (should NOT be quoted)
				if reNumber.MatchString(value) {
					if strings.HasPrefix(value, "\"") {
						t.Errorf("Numeric value should not be quoted: %s (value: '%s')", line, value)
					}
					continue
				}

				// Check if it's null (should NOT be quoted - it's a YAML literal for nil)
				if value == "null" {
					continue
				}

				// Everything else should be a quoted string
				if !strings.HasPrefix(value, "\"") || !strings.HasSuffix(value, "\"") {
					t.Errorf("String value should be quoted: %s (value: '%s')", line, value)
				}
			}
		}
	}
}

func TestGenerateConfigYaml_SecretHiding(t *testing.T) {
	// Create temporary source file with our test structs
	sourceContent := `package settings

type TestConfig struct {
	PublicField string ` + "`json:\"publicField\"`" + ` // A regular public field
	SecretField string ` + "`json:\"secretField\"`" + ` // secret: this should be hidden
}

type TestNested struct {
	NestedPublic string ` + "`json:\"nestedPublic\"`" + ` // A nested public field
	NestedSecret string ` + "`json:\"nestedSecret\"`" + ` // secret: nested secret field
}
`

	tmpFile, cleanup := createTestSourceFile(t, sourceContent)
	defer cleanup()

	// Test secret detection
	_, secrets, _, err := CollectCommentsAndSecrets(tmpFile)
	if err != nil {
		t.Fatalf("CollectCommentsAndSecrets failed: %v", err)
	}

	// Verify secret fields are detected
	if !secrets["TestConfig"]["SecretField"] {
		t.Error("SecretField should be marked as secret")
	}
	if !secrets["TestNested"]["NestedSecret"] {
		t.Error("NestedSecret should be marked as secret")
	}
	if secrets["TestConfig"]["PublicField"] {
		t.Error("PublicField should not be marked as secret")
	}

	// Test that secrets are properly collected
	if len(secrets["TestConfig"]) != 1 {
		t.Errorf("Expected 1 secret field in TestConfig, got %d", len(secrets["TestConfig"]))
	}
}

func TestGenerateConfigYaml_DeprecatedFiltering(t *testing.T) {
	// Create temporary source file with deprecated fields
	sourceContent := `package settings

type TestConfig struct {
	PublicField     string ` + "`json:\"publicField\"`" + ` // A regular public field
	DeprecatedField string ` + "`json:\"deprecatedField\"`" + ` // deprecated: this field is old
}
`

	tmpFile, cleanup := createTestSourceFile(t, sourceContent)
	defer cleanup()

	// Test deprecated detection
	_, _, deprecated, err := CollectCommentsAndSecrets(tmpFile)
	if err != nil {
		t.Fatalf("CollectCommentsAndSecrets failed: %v", err)
	}

	// Verify deprecated fields are detected
	if !deprecated["TestConfig"]["DeprecatedField"] {
		t.Error("DeprecatedField should be marked as deprecated")
	}
	if deprecated["TestConfig"]["PublicField"] {
		t.Error("PublicField should not be marked as deprecated")
	}
}

func TestGenerateConfigYaml_FullVsFiltered(t *testing.T) {
	// Create minimal test source
	sourceContent := `package settings

type Settings struct {
	UserDefaults UserDefaults ` + "`json:\"userDefaults\"`" + `
}

type UserDefaults struct {
	DarkMode bool ` + "`json:\"darkMode\"`" + `
	Locale string ` + "`json:\"locale\"`" + `
}
`
	tmpFile, cleanup := createTestSourceFile(t, sourceContent)
	defer cleanup()
	sourcePath := tmpFile[:strings.LastIndex(tmpFile, "/")]

	// Create a config with both default and non-default values
	trueVal := true
	config := &Settings{
		UserDefaults: UserDefaults{
			DarkMode: &trueVal, // This matches default
			Locale:   "es",     // This differs from default ("en")
		},
	}

	// Test full=true - should show all fields
	fullYaml, err := GenerateConfigYamlWithSource(config, false, true, false, sourcePath)
	if err != nil {
		t.Fatalf("GenerateConfigYaml with full=true failed: %v", err)
	}

	// Test full=false - should only show non-default fields
	filteredYaml, err := GenerateConfigYamlWithSource(config, false, false, false, sourcePath)
	if err != nil {
		t.Fatalf("GenerateConfigYaml with full=false failed: %v", err)
	}

	// Full output should be longer than filtered
	if len(fullYaml) <= len(filteredYaml) {
		t.Error("Full YAML should be longer than filtered YAML")
	}

	// Filtered output should contain the non-default field
	if !strings.Contains(filteredYaml, "locale") {
		t.Error("Filtered YAML should contain non-default locale field")
	}

	// Full output should contain default fields
	if !strings.Contains(fullYaml, "darkMode") {
		t.Error("Full YAML should contain darkMode field")
	}
}

func TestGenerateConfigYaml_CommentsOnOff(t *testing.T) {
	// Use the actual settings source directory since comments work there
	config := &Settings{
		UserDefaults: UserDefaults{
			Locale: "en",
		},
	}

	// Test with comments=true using the real source
	withComments, err := GenerateConfigYaml(config, true, true, false)
	if err != nil {
		t.Fatalf("GenerateConfigYaml with comments=true failed: %v", err)
	}

	// Test with comments=false using the real source
	withoutComments, err := GenerateConfigYaml(config, false, true, false)
	if err != nil {
		t.Fatalf("GenerateConfigYaml with comments=false failed: %v", err)
	}

	// Output with comments should be longer
	if len(withComments) <= len(withoutComments) {
		t.Errorf("YAML with comments (%d chars) should be longer than without comments (%d chars)", len(withComments), len(withoutComments))
	}

	// With comments should contain '#' characters
	if !strings.Contains(withComments, "#") {
		t.Error("YAML with comments should contain '#' characters")
	}

	// Without comments should not contain '#' characters
	if strings.Contains(withoutComments, "#") {
		t.Error("YAML without comments should not contain '#' characters")
	}
}

func TestGenerateConfigYaml_IntegrationTest(t *testing.T) {
	reNumber := regexp.MustCompile(`^-?\d+(\.\d+)?$`)
	// Comprehensive test with all features
	trueVal := true
	config := &Settings{
		UserDefaults: UserDefaults{
			Locale:                  "es",          // Non-default
			DarkMode:                &trueVal,      // Default
			DisableOfficePreviewExt: ".docx .xlsx", // This is deprecated
		},
		Auth: Auth{
			Key:           "secret123", // This is secret
			AdminUsername: "admin",     // This is secret
			AdminPassword: "password",  // This is secret
		},
	}

	tests := []struct {
		name             string
		showComments     bool
		showFull         bool
		filterDeprecated bool
		expectSecret     bool
		expectDeprecated bool
		expectFull       bool
		expectComments   bool
	}{
		{
			name:             "API mode: show all including deprecated",
			showComments:     false,
			showFull:         true,
			filterDeprecated: false,
			expectSecret:     true,  // secrets should be hidden
			expectDeprecated: true,  // deprecated should be shown
			expectFull:       true,  // all fields shown
			expectComments:   false, // no comments
		},
		{
			name:             "Static config mode: filter deprecated",
			showComments:     true,
			showFull:         true,
			filterDeprecated: true,
			expectSecret:     true,  // secrets should be hidden
			expectDeprecated: false, // deprecated should be filtered
			expectFull:       true,  // all non-deprecated fields shown
			expectComments:   true,  // comments included
		},
		{
			name:             "Filtered output: only changes",
			showComments:     false,
			showFull:         false,
			filterDeprecated: false,
			expectSecret:     true,  // secrets should be hidden
			expectDeprecated: false, // might not appear if matches default
			expectFull:       false, // only non-default fields
			expectComments:   false, // no comments
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlOutput, err := GenerateConfigYaml(config, tt.showComments, tt.showFull, tt.filterDeprecated)
			if err != nil {
				t.Fatalf("GenerateConfigYaml failed: %v", err)
			}

			// Check secret hiding
			if tt.expectSecret {
				if !strings.Contains(yamlOutput, "**hidden**") {
					t.Error("Expected secrets to be hidden with **hidden**")
				}
				if strings.Contains(yamlOutput, "secret123") {
					t.Error("Secret values should not appear in output")
				}
			}

			// Check deprecated field filtering
			hasDeprecated := strings.Contains(yamlOutput, "disableOfficePreviewExt")
			if tt.expectDeprecated && !hasDeprecated {
				t.Error("Expected deprecated field to be present")
			}
			if !tt.expectDeprecated && hasDeprecated && tt.filterDeprecated {
				t.Error("Expected deprecated field to be filtered out")
			}

			// Check comments
			hasComments := strings.Contains(yamlOutput, "#")
			if tt.expectComments && !hasComments {
				t.Error("Expected comments to be present")
			}
			if !tt.expectComments && hasComments {
				t.Error("Expected no comments")
			}

			// Check string quoting - all string values should be quoted
			lines := strings.Split(yamlOutput, "\n")
			for _, line := range lines {
				if strings.Contains(line, ":") && !strings.HasSuffix(strings.TrimSpace(line), ":") {
					// This is a key-value line
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						value := strings.TrimSpace(parts[1])
						// Skip comments and empty values
						if commentIdx := strings.Index(value, "#"); commentIdx >= 0 {
							// If comment starts at the beginning, there's no value, just a comment
							if commentIdx == 0 {
								continue
							}
							value = strings.TrimSpace(value[:commentIdx])
						}
						// Check if it's a number (should NOT be quoted)
						if reNumber.MatchString(value) {
							if strings.HasPrefix(value, "\"") {
								t.Errorf("Numeric value should not be quoted in line: %s (value: '%s')", line, value)
							}
							continue
						}

						// Check if it's a boolean (should NOT be quoted)
						if value == "true" || value == "false" {
							if strings.HasPrefix(value, "\"") {
								t.Errorf("Boolean value should not be quoted in line: %s (value: '%s')", line, value)
							}
							continue
						}

						// Skip arrays, objects, empty values, and null
						if value == "" || value == "null" || strings.HasPrefix(value, "[") || strings.HasPrefix(value, "{") || strings.Contains(value, ":") {
							continue
						}

						// Everything else should be a quoted string
						if !strings.HasPrefix(value, "\"") || !strings.HasSuffix(value, "\"") {
							t.Errorf("String value should be quoted in line: %s (value: '%s')", line, value)
						}
					}
				}
			}

			t.Logf("Generated YAML for %s:\n%s", tt.name, yamlOutput)
		})
	}
}

func TestGenerateConfigYaml_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		config *Settings
		desc   string
	}{
		{
			name:   "empty_config",
			config: &Settings{},
			desc:   "Empty config should generate valid YAML",
		},
		{
			name: "only_defaults",
			config: &Settings{
				UserDefaults: UserDefaults{
					DarkMode: boolPtr(true), // default value
					Locale:   "en",          // default value
				},
			},
			desc: "Config with only default values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test all combinations
			for _, showComments := range []bool{false, true} {
				for _, showFull := range []bool{false, true} {
					for _, filterDeprecated := range []bool{false, true} {
						yamlOutput, err := GenerateConfigYaml(tt.config, showComments, showFull, filterDeprecated)
						if err != nil {
							t.Fatalf("GenerateConfigYaml failed for %s (comments=%v, full=%v, filterDeprecated=%v): %v",
								tt.desc, showComments, showFull, filterDeprecated, err)
						}

						// Basic validation - should produce valid YAML structure
						if yamlOutput == "" {
							t.Errorf("Generated empty YAML for %s", tt.desc)
						}

						// Should not contain obvious error messages (but allow "error" in log levels etc)
						if strings.Contains(yamlOutput, "Error:") || strings.Contains(yamlOutput, "ERROR:") {
							t.Errorf("YAML contains error for %s", tt.desc)
						}
					}
				}
			}
		})
	}
}
