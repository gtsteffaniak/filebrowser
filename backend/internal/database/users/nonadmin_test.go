package users

import (
	"reflect"
	"testing"
)

func TestNonAdminEditableContainsOtpEnabled(t *testing.T) {
	// Test that OtpEnabled is included in NonAdminEditable struct
	structType := reflect.TypeOf(NonAdminEditable{})

	// Check if OtpEnabled field exists
	otpEnabledField, exists := structType.FieldByName("OtpEnabled")
	if !exists {
		t.Fatal("OtpEnabled field should exist in NonAdminEditable struct")
	}

	// Check the JSON tag
	expectedTag := "otpEnabled"
	if otpEnabledField.Tag.Get("json") != expectedTag {
		t.Errorf("Expected JSON tag '%s', got '%s'", expectedTag, otpEnabledField.Tag.Get("json"))
	}

	// Check the field type
	if otpEnabledField.Type.Kind() != reflect.Bool {
		t.Errorf("Expected OtpEnabled to be bool, got %s", otpEnabledField.Type.Kind())
	}
}

func TestGetNonAdminEditableFieldNames(t *testing.T) {
	// This is a helper function that should be tested
	// We'll test it indirectly by checking if OtpEnabled is in the list
	names := getNonAdminEditableFieldNames()

	// Check that OtpEnabled is in the list
	found := false
	for _, name := range names {
		if name == "OtpEnabled" {
			found = true
			break
		}
	}

	if !found {
		t.Error("OtpEnabled should be in the list of non-admin editable field names")
	}

	// Check that some other expected fields are present
	expectedFields := []string{"DarkMode", "Locale", "ViewMode", "SingleClick"}
	for _, expected := range expectedFields {
		found := false
		for _, name := range names {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected field %s should be in non-admin editable fields", expected)
		}
	}
}

// Helper function to get field names (copied from bolt/users.go for testing)
func getNonAdminEditableFieldNames() []string {
	var names []string
	t := reflect.TypeOf(NonAdminEditable{})
	for i := 0; i < t.NumField(); i++ {
		names = append(names, t.Field(i).Name)
	}
	return names
}
