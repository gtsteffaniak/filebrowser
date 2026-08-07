package users

import (
	"reflect"
	"testing"
)

func TestFrontendUserContainsOtpEnabled(t *testing.T) {
	structType := reflect.TypeOf(FrontendUser{})

	otpEnabledField, exists := structType.FieldByName("OtpEnabled")
	if !exists {
		t.Fatal("OtpEnabled field should exist on FrontendUser")
	}

	expectedTag := "otpEnabled"
	if otpEnabledField.Tag.Get("json") != expectedTag {
		t.Errorf("Expected JSON tag '%s', got '%s'", expectedTag, otpEnabledField.Tag.Get("json"))
	}

	if otpEnabledField.Type.Kind() != reflect.Bool {
		t.Errorf("Expected OtpEnabled to be bool, got %s", otpEnabledField.Type.Kind())
	}
}

func TestNonAdminEditableDoesNotDuplicateOtpEnabled(t *testing.T) {
	structType := reflect.TypeOf(NonAdminEditable{})
	if _, exists := structType.FieldByName("OtpEnabled"); exists {
		t.Fatal("OtpEnabled should not be duplicated on NonAdminEditable")
	}
}

func TestGetNonAdminEditableFieldNames(t *testing.T) {
	names := getNonAdminEditableFieldNames()

	for _, name := range names {
		if name == "OtpEnabled" {
			t.Error("OtpEnabled should not be in NonAdminEditable field names")
		}
	}

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

func getNonAdminEditableFieldNames() []string {
	var names []string
	t := reflect.TypeOf(NonAdminEditable{})
	for i := 0; i < t.NumField(); i++ {
		names = append(names, t.Field(i).Name)
	}
	return names
}
