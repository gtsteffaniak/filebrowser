package bolt

import (
	"path/filepath"
	"testing"

	"github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

func init() {
	users.BcryptCost = 4 // bcrypt.MinCost for faster tests
}

func createTestUsersBackend(t *testing.T) users.StorageBackend {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := storm.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open storm db: %v", err)
	}
	return NewUsersBackend(db)
}

func createTestUser(t *testing.T, backend users.StorageBackend, username string, isAdmin bool) *users.User {
	user := &users.User{
		Username: username,
		NonAdminEditable: users.NonAdminEditable{
			Password:    "testpass123",
			DarkMode:    false,
			OtpEnabled:  true,
			Locale:      "en",
			ViewMode:    "normal",
			SingleClick: false,
		},
		Permissions: users.Permissions{
			Admin: isAdmin,
		},
		LoginMethod: users.LoginMethodPassword,
	}

	err := backend.Save(user, true, false)
	if err != nil {
		t.Fatalf("failed to create user %s: %v", username, err)
	}
	return user
}

func TestUpdate_AdminModifyOtherUser(t *testing.T) {
	backend := createTestUsersBackend(t)

	// Create admin user
	_ = createTestUser(t, backend, "admin", true)

	// Create regular user
	regularUser := createTestUser(t, backend, "regular", false)

	// Admin modifies regular user
	updateData := &users.User{
		ID:       regularUser.ID,
		Username: regularUser.Username,
		NonAdminEditable: users.NonAdminEditable{
			DarkMode:   true,
			OtpEnabled: false,
			Locale:     "fr",
		},
		Permissions: users.Permissions{
			Admin: false, // Admin can modify permissions
		},
	}

	// Test with which=all
	err := backend.Update(updateData, true, "all")
	if err != nil {
		t.Errorf("Admin should be able to modify other user with which=all: %v", err)
	}

	// Verify changes were applied
	updatedUser, err := backend.GetBy(regularUser.ID)
	if err != nil {
		t.Fatalf("Failed to get updated user: %v", err)
	}

	if !updatedUser.DarkMode {
		t.Error("DarkMode should have been updated to true")
	}
	if updatedUser.OtpEnabled {
		t.Error("OtpEnabled should have been updated to false")
	}
	if updatedUser.Locale != "fr" {
		t.Errorf("Locale should have been updated to 'fr', got '%s'", updatedUser.Locale)
	}

	// Test with specific fields
	updateData2 := &users.User{
		ID:       regularUser.ID,
		Username: regularUser.Username,
		NonAdminEditable: users.NonAdminEditable{
			ViewMode: "list",
		},
	}

	err = backend.Update(updateData2, true, "ViewMode")
	if err != nil {
		t.Errorf("Admin should be able to modify specific field: %v", err)
	}

	updatedUser2, err := backend.GetBy(regularUser.ID)
	if err != nil {
		t.Fatalf("Failed to get updated user: %v", err)
	}

	if updatedUser2.ViewMode != "list" {
		t.Errorf("ViewMode should have been updated to 'list', got '%s'", updatedUser2.ViewMode)
	}
}

func TestUpdate_AdminModifySelf(t *testing.T) {
	backend := createTestUsersBackend(t)

	// Create admin user
	admin := createTestUser(t, backend, "admin", true)

	// Admin modifies themselves
	updateData := &users.User{
		ID:       admin.ID,
		Username: admin.Username,
		NonAdminEditable: users.NonAdminEditable{
			DarkMode:   true,
			OtpEnabled: false,
			Locale:     "de",
		},
	}

	// Test with which=all
	err := backend.Update(updateData, true, "all")
	if err != nil {
		t.Errorf("Admin should be able to modify themselves with which=all: %v", err)
	}

	// Verify changes were applied
	updatedUser, err := backend.GetBy(admin.ID)
	if err != nil {
		t.Fatalf("Failed to get updated user: %v", err)
	}

	if !updatedUser.DarkMode {
		t.Error("DarkMode should have been updated to true")
	}
	if updatedUser.OtpEnabled {
		t.Error("OtpEnabled should have been updated to false")
	}
	if updatedUser.Locale != "de" {
		t.Errorf("Locale should have been updated to 'de', got '%s'", updatedUser.Locale)
	}
}

func TestUpdate_NonAdminModifySelf(t *testing.T) {
	backend := createTestUsersBackend(t)

	// Create regular user
	regularUser := createTestUser(t, backend, "regular", false)

	// Non-admin modifies themselves
	updateData := &users.User{
		ID:       regularUser.ID,
		Username: regularUser.Username,
		NonAdminEditable: users.NonAdminEditable{
			DarkMode:   true,
			OtpEnabled: false, // This should be allowed now
			Locale:     "es",
			ViewMode:   "grid",
		},
	}

	// Test with which=all
	err := backend.Update(updateData, false, "all")
	if err != nil {
		t.Errorf("Non-admin should be able to modify themselves with which=all: %v", err)
	}

	// Verify changes were applied
	updatedUser, err := backend.GetBy(regularUser.ID)
	if err != nil {
		t.Fatalf("Failed to get updated user: %v", err)
	}

	if !updatedUser.DarkMode {
		t.Error("DarkMode should have been updated to true")
	}
	if updatedUser.OtpEnabled {
		t.Error("OtpEnabled should have been updated to false")
	}
	if updatedUser.Locale != "es" {
		t.Errorf("Locale should have been updated to 'es', got '%s'", updatedUser.Locale)
	}
	if updatedUser.ViewMode != "grid" {
		t.Errorf("ViewMode should have been updated to 'grid', got '%s'", updatedUser.ViewMode)
	}

	// Test with specific fields
	updateData2 := &users.User{
		ID:       regularUser.ID,
		Username: regularUser.Username,
		NonAdminEditable: users.NonAdminEditable{
			SingleClick: true,
		},
	}

	err = backend.Update(updateData2, false, "SingleClick")
	if err != nil {
		t.Errorf("Non-admin should be able to modify specific allowed field: %v", err)
	}

	updatedUser2, err := backend.GetBy(regularUser.ID)
	if err != nil {
		t.Fatalf("Failed to get updated user: %v", err)
	}

	if !updatedUser2.SingleClick {
		t.Error("SingleClick should have been updated to true")
	}
}

func TestUpdate_NonAdminModifySelfRestrictedFields(t *testing.T) {
	backend := createTestUsersBackend(t)

	// Create regular user
	regularUser := createTestUser(t, backend, "regular", false)

	// Non-admin tries to modify restricted fields (should be filtered out, not error)
	updateData := &users.User{
		ID:       regularUser.ID,
		Username: regularUser.Username,
		NonAdminEditable: users.NonAdminEditable{
			DarkMode: true, // This should be allowed
		},
		Permissions: users.Permissions{
			Admin: true, // This should be filtered out
		},
		Scopes: []users.SourceScope{ // This should be filtered out
			{Name: "test", Scope: "/test"},
		},
	}

	// Test with which=all - should not error, but should filter out restricted fields
	err := backend.Update(updateData, false, "all")
	if err != nil {
		t.Errorf("Non-admin should not error when trying to modify restricted fields (should be filtered): %v", err)
	}

	// Verify only allowed fields were updated
	updatedUser, err := backend.GetBy(regularUser.ID)
	if err != nil {
		t.Fatalf("Failed to get updated user: %v", err)
	}

	// Allowed field should be updated
	if !updatedUser.DarkMode {
		t.Error("DarkMode should have been updated to true")
	}

	// Restricted fields should NOT be updated
	if updatedUser.Permissions.Admin {
		t.Error("Permissions.Admin should not have been updated (restricted field)")
	}
	if len(updatedUser.Scopes) != 0 {
		t.Error("Scopes should not have been updated (restricted field)")
	}

	// Test with specific restricted fields - should be filtered out
	updateData2 := &users.User{
		ID:       regularUser.ID,
		Username: regularUser.Username,
		Permissions: users.Permissions{
			Admin: true,
		},
	}

	err = backend.Update(updateData2, false, "Permissions")
	if err == nil {
		t.Error("Non-admin should get error when trying to modify only restricted fields (all fields filtered out)")
	}
	if err.Error() != "no fields to update" {
		t.Errorf("Expected 'no fields to update' error, got: %v", err)
	}

	// Verify restricted field was not updated
	updatedUser2, err := backend.GetBy(regularUser.ID)
	if err != nil {
		t.Fatalf("Failed to get updated user: %v", err)
	}

	if updatedUser2.Permissions.Admin {
		t.Error("Permissions.Admin should not have been updated (restricted field)")
	}
}

func TestUpdate_NonAdminModifyOtherUser(t *testing.T) {
	backend := createTestUsersBackend(t)

	// Create two regular users
	_ = createTestUser(t, backend, "user1", false)
	user2 := createTestUser(t, backend, "user2", false)

	// Non-admin tries to modify another user (this should be handled at a higher level)
	// But if it gets to the backend, it should work the same as modifying self
	updateData := &users.User{
		ID:       user2.ID,
		Username: user2.Username,
		NonAdminEditable: users.NonAdminEditable{
			DarkMode: true,
		},
	}

	// This should work (filtering restricted fields) but the higher-level auth should prevent this
	err := backend.Update(updateData, false, "DarkMode")
	if err != nil {
		t.Errorf("Backend should handle non-admin modifying other user (filtering restricted fields): %v", err)
	}

	// Verify the change was applied
	updatedUser, err := backend.GetBy(user2.ID)
	if err != nil {
		t.Fatalf("Failed to get updated user: %v", err)
	}

	if !updatedUser.DarkMode {
		t.Error("DarkMode should have been updated to true")
	}
}

func TestUpdate_NonAdminModifyOtherUserOtp(t *testing.T) {
	backend := createTestUsersBackend(t)

	// Create two regular users
	_ = createTestUser(t, backend, "user1", false)
	user2 := createTestUser(t, backend, "user2", false)

	// Non-admin tries to modify another user's OTP
	updateData := &users.User{
		ID:       user2.ID,
		Username: user2.Username,
		NonAdminEditable: users.NonAdminEditable{
			OtpEnabled: false,
		},
	}

	// This should work (OtpEnabled is now in NonAdminEditable)
	err := backend.Update(updateData, false, "OtpEnabled")
	if err != nil {
		t.Errorf("Non-admin should be able to modify OTP field: %v", err)
	}

	// Verify the change was applied
	updatedUser, err := backend.GetBy(user2.ID)
	if err != nil {
		t.Fatalf("Failed to get updated user: %v", err)
	}

	if updatedUser.OtpEnabled {
		t.Error("OtpEnabled should have been updated to false")
	}
}

func TestUpdate_WhichAllVsSpecific(t *testing.T) {
	backend := createTestUsersBackend(t)

	// Create regular user
	regularUser := createTestUser(t, backend, "regular", false)

	// Test which=all
	updateDataAll := &users.User{
		ID:       regularUser.ID,
		Username: regularUser.Username,
		NonAdminEditable: users.NonAdminEditable{
			DarkMode:   true,
			OtpEnabled: false,
			Locale:     "fr",
			ViewMode:   "list",
		},
	}

	err := backend.Update(updateDataAll, false, "all")
	if err != nil {
		t.Errorf("which=all should work: %v", err)
	}

	// Verify changes
	updatedUser, err := backend.GetBy(regularUser.ID)
	if err != nil {
		t.Fatalf("Failed to get updated user: %v", err)
	}

	if !updatedUser.DarkMode || updatedUser.OtpEnabled || updatedUser.Locale != "fr" || updatedUser.ViewMode != "list" {
		t.Error("which=all should update all allowed fields")
	}

	// Reset user
	regularUser = createTestUser(t, backend, "regular2", false)

	// Test specific fields
	updateDataSpecific := &users.User{
		ID:       regularUser.ID,
		Username: regularUser.Username,
		NonAdminEditable: users.NonAdminEditable{
			DarkMode: true,
			Locale:   "de",
		},
	}

	err = backend.Update(updateDataSpecific, false, "DarkMode", "Locale")
	if err != nil {
		t.Errorf("specific fields should work: %v", err)
	}

	// Verify only specified fields were updated
	updatedUser2, err := backend.GetBy(regularUser.ID)
	if err != nil {
		t.Fatalf("Failed to get updated user: %v", err)
	}

	if !updatedUser2.DarkMode {
		t.Error("DarkMode should have been updated")
	}
	if updatedUser2.Locale != "de" {
		t.Error("Locale should have been updated")
	}
	if updatedUser2.ViewMode != "normal" { // Should remain default
		t.Error("ViewMode should not have been updated")
	}
}

func TestUpdate_EmptyFieldsAfterFiltering(t *testing.T) {
	backend := createTestUsersBackend(t)

	// Create regular user
	regularUser := createTestUser(t, backend, "regular", false)

	// Non-admin tries to update only restricted fields
	updateData := &users.User{
		ID:       regularUser.ID,
		Username: regularUser.Username,
		Permissions: users.Permissions{
			Admin: true, // This will be filtered out
		},
		Scopes: []users.SourceScope{ // This will be filtered out
			{Name: "test", Scope: "/test"},
		},
	}

	// This should result in "no fields to update" error
	err := backend.Update(updateData, false, "Permissions", "Scopes")
	if err == nil {
		t.Error("Should get error when all fields are filtered out")
	}

	if err.Error() != "no fields to update" {
		t.Errorf("Expected 'no fields to update' error, got: %v", err)
	}
}

func TestFilterRestrictedFields(t *testing.T) {
	// Test the filterRestrictedFields function directly
	testCases := []struct {
		name     string
		fields   []string
		expected []string
	}{
		{
			name:     "all allowed fields",
			fields:   []string{"DarkMode", "OtpEnabled", "Locale", "ViewMode"},
			expected: []string{"DarkMode", "OtpEnabled", "Locale", "ViewMode"},
		},
		{
			name:     "mixed allowed and restricted",
			fields:   []string{"DarkMode", "Permissions", "Scopes", "OtpEnabled"},
			expected: []string{"DarkMode", "OtpEnabled"},
		},
		{
			name:     "all restricted fields",
			fields:   []string{"Permissions", "Scopes", "Username", "ID"},
			expected: []string{},
		},
		{
			name:     "empty fields",
			fields:   []string{},
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := filterRestrictedFields(tc.fields)

			if len(result) != len(tc.expected) {
				t.Errorf("Expected %d fields, got %d", len(tc.expected), len(result))
			}

			for i, field := range result {
				if field != tc.expected[i] {
					t.Errorf("Expected field %s at position %d, got %s", tc.expected[i], i, field)
				}
			}
		})
	}
}
