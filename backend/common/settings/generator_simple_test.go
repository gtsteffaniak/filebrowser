package settings

import (
	"regexp"
	"strings"
	"testing"
)

func TestGenerateConfigYaml_Basic(t *testing.T) {
	reNumber := regexp.MustCompile(`^-?\d+(\.\d+)?$`)
	// Test using the actual source directory structure
	trueVal := true
	config := &Settings{
		UserDefaults: UserDefaults{
			Locale:                  "es",          // Non-default value
			DarkMode:                &trueVal,      // Default value
			DisableOfficePreviewExt: ".docx .xlsx", // This field is deprecated
		},
		Auth: Auth{
			Key:           "secret123",    // This is a secret field
			AdminUsername: "testadmin",    // This is a secret field
			AdminPassword: "testpassword", // This is a secret field
		},
	}

	// Test different combinations
	testCases := []struct {
		name             string
		showComments     bool
		showFull         bool
		filterDeprecated bool
		expectSecrets    bool
		expectDeprecated bool
	}{
		{
			name:             "API_mode_all_fields",
			showComments:     false,
			showFull:         true,
			filterDeprecated: false,
			expectSecrets:    true, // Secrets should be hidden
			expectDeprecated: true, // Deprecated should be shown
		},
		{
			name:             "Static_config_filter_deprecated",
			showComments:     true,
			showFull:         true,
			filterDeprecated: true,
			expectSecrets:    true,  // Secrets should be hidden
			expectDeprecated: false, // Deprecated should be filtered
		},
		{
			name:             "Filtered_non_defaults",
			showComments:     false,
			showFull:         false,
			filterDeprecated: false,
			expectSecrets:    true,  // Secrets should be hidden
			expectDeprecated: false, // Might not appear if matches default
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use the current directory structure for real testing
			yamlOutput, err := GenerateConfigYaml(config, tc.showComments, tc.showFull, tc.filterDeprecated)
			if err != nil {
				t.Fatalf("GenerateConfigYaml failed: %v", err)
			}

			// Basic validation - should produce some output
			if len(yamlOutput) == 0 {
				t.Fatal("Generated empty YAML")
			}

			// Test string quoting - string values should be quoted, numbers and booleans should not
			lines := strings.Split(yamlOutput, "\n")
			for _, line := range lines {
				if strings.Contains(line, ":") && !strings.HasSuffix(strings.TrimSpace(line), ":") {
					// This is a key-value line, skip comment portion
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						value := strings.TrimSpace(parts[1])
						// Remove comments
						if commentIdx := strings.Index(value, "#"); commentIdx >= 0 {
							value = strings.TrimSpace(value[:commentIdx])
						}

						// Skip empty values, arrays, and objects
						if value == "" || strings.HasPrefix(value, "[") || strings.HasPrefix(value, "{") {
							continue
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

						// Check if it's null (should NOT be quoted - it's a YAML literal for nil)
						if value == "null" {
							continue
						}

						// Everything else should be a quoted string
						if !strings.HasPrefix(value, "\"") || !strings.HasSuffix(value, "\"") {
							// Add debug info to see what's wrong
							t.Logf("DEBUG: Checking value '%s' from line '%s'", value, strings.TrimSpace(line))
							t.Errorf("String value should be quoted in line: %s (value: %s)", strings.TrimSpace(line), value)
						}
					}
				}
			}

			// Test secret hiding
			if tc.expectSecrets {
				if !strings.Contains(yamlOutput, "**hidden**") {
					t.Error("Expected secrets to be hidden with **hidden**")
				}
				if strings.Contains(yamlOutput, "secret123") || strings.Contains(yamlOutput, "testadmin") || strings.Contains(yamlOutput, "testpassword") {
					t.Error("Secret values should not appear in output")
				}
			}

			// Test deprecated field filtering
			hasDeprecated := strings.Contains(yamlOutput, "disableOfficePreviewExt")
			if tc.expectDeprecated && !hasDeprecated {
				t.Error("Expected deprecated field to be present")
			}
			if !tc.expectDeprecated && hasDeprecated && tc.filterDeprecated {
				t.Error("Expected deprecated field to be filtered out when filterDeprecated=true")
			}

			// Test comments
			hasComments := strings.Contains(yamlOutput, "#")
			if tc.showComments && !hasComments {
				t.Error("Expected comments to be present when showComments=true")
			}

			t.Logf("Test case: %s\nGenerated YAML:\n%s\n", tc.name, yamlOutput)
		})
	}
}

func TestCollectCommentsAndSecrets_Basic(t *testing.T) {
	// Test the comment and secret collection directly
	comments, secrets, deprecated, err := CollectCommentsAndSecrets(".")
	if err != nil {
		t.Fatalf("CollectCommentsAndSecrets failed: %v", err)
	}

	// Should find some secrets in the actual source files
	foundSecrets := false
	for typeName, typeSecrets := range secrets {
		for fieldName := range typeSecrets {
			t.Logf("Found secret: %s.%s", typeName, fieldName)
			foundSecrets = true
		}
	}
	if !foundSecrets {
		t.Error("Expected to find some secret fields in the actual source code")
	}

	// Should find some deprecated fields
	foundDeprecated := false
	for typeName, typeDeprecated := range deprecated {
		for fieldName := range typeDeprecated {
			t.Logf("Found deprecated: %s.%s", typeName, fieldName)
			foundDeprecated = true
		}
	}
	if !foundDeprecated {
		t.Error("Expected to find some deprecated fields in the actual source code")
	}

	t.Logf("Comments: %d types", len(comments))
	t.Logf("Secrets: %d types", len(secrets))
	t.Logf("Deprecated: %d types", len(deprecated))
}

func TestGenerateYaml_StaticGeneration(t *testing.T) {
	// Test the static generation function that's used by FILEBROWSER_GENERATE_CONFIG=true

	// Create a temporary config for testing
	config := &Settings{
		UserDefaults: UserDefaults{
			Locale:                  "en",
			DisableOfficePreviewExt: ".docx .xlsx", // This should be filtered out
		},
		Auth: Auth{
			Key: "test-secret", // This should be redacted
		},
	}

	// Test static generation with comments and deprecated filtering
	yamlOutput, err := GenerateConfigYamlWithSource(config, true, true, true, ".")
	if err != nil {
		t.Fatalf("Static generation failed: %v", err)
	}

	// Verify comments are included
	if !strings.Contains(yamlOutput, "#") {
		t.Error("Static generation should include comments")
	}

	// Verify deprecated field is filtered out
	if strings.Contains(yamlOutput, "disableOfficePreviewExt") {
		t.Error("Static generation should filter out deprecated field disableOfficePreviewExt")
	}

	// Verify secrets are redacted
	if !strings.Contains(yamlOutput, "**hidden**") {
		t.Error("Static generation should redact secrets")
	}
	if strings.Contains(yamlOutput, "test-secret") {
		t.Error("Static generation should not contain actual secret values")
	}

	// Verify it has proper structure
	if !strings.Contains(yamlOutput, "userDefaults:") {
		t.Error("Static generation should contain userDefaults section")
	}
	if !strings.Contains(yamlOutput, "auth:") {
		t.Error("Static generation should contain auth section")
	}

	t.Logf("Static generation successful with comments and proper filtering")
}
