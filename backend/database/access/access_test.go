package access_test

import (
	"path/filepath"
	"testing"

	"github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/access"
	boltusers "github.com/gtsteffaniak/filebrowser/backend/database/storage/bolt"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

func createTestStorage(t *testing.T) (*access.Storage, *users.Storage) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := storm.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open storm db: %v", err)
	}
	userStore := users.NewStorage(boltusers.NewUsersBackend(db))
	return access.NewStorage(db, userStore), userStore
}

func createTestUser(t *testing.T, userStore *users.Storage, username string) {
	u := &users.User{NonAdminEditable: users.NonAdminEditable{Password: "test"}, Username: username}
	err := userStore.Save(u, false, false)
	if err != nil {
		t.Fatalf("failed to create user %s: %v", username, err)
	}
}

func setupTestSources() {
	// Setup default test sources with allow-by-default behavior
	settings.Config.Server.SourceMap = map[string]settings.Source{
		"mnt/storage": {
			Path: "mnt/storage",
			Name: "storage",
			Config: settings.SourceConfig{
				DenyByDefault: false,
			},
		},
		"mnt/open": {
			Path: "mnt/open",
			Name: "open",
			Config: settings.SourceConfig{
				DenyByDefault: false,
			},
		},
	}
}

func TestPermitted_UserBlacklist(t *testing.T) {
	setupTestSources()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "alice")
	createTestUser(t, userStore, "bob")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	if err := s.DenyUser("mnt/storage", "/secret", "alice"); err != nil {
		t.Errorf("DenyUser failed: %v", err)
	}
	if s.Permitted("mnt/storage", "/secret", "alice") {
		t.Error("alice should not be permitted (denied)")
	}
	if !s.Permitted("mnt/storage", "/secret", "bob") {
		t.Error("bob should be permitted (not denied)")
	}
}

func TestPermitted_UserWhitelist(t *testing.T) {
	setupTestSources()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "alice")
	createTestUser(t, userStore, "bob")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	if err := s.AllowUser("mnt/storage", "/vip", "bob"); err != nil {
		t.Errorf("AllowUser failed: %v", err)
	}
	if !s.Permitted("mnt/storage", "/vip", "bob") {
		t.Error("bob should be permitted (allowed)")
	}
	if !s.Permitted("mnt/storage", "/vip", "alice") {
		t.Error("alice should be permitted (default allow behavior when DenyByDefault=false)")
	}
}

func TestPermitted_GroupBlacklist(t *testing.T) {
	setupTestSources()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "alice")
	createTestUser(t, userStore, "bob")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	_ = s.AddUserToGroup("admins", "alice")
	if err := s.DenyGroup("mnt/storage", "/admin", "admins"); err != nil {
		t.Errorf("DenyGroup failed: %v", err)
	}
	if s.Permitted("mnt/storage", "/admin", "bob") == false {
		t.Error("bob should be permitted (not in denied group)")
	}
	if s.Permitted("mnt/storage", "/admin", "alice") {
		t.Error("alice should not be permitted (in denied group)")
	}
}

func TestPermitted_GroupWhitelist(t *testing.T) {
	setupTestSources()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "alice")
	createTestUser(t, userStore, "bob")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	_ = s.AddUserToGroup("vip", "bob")
	if err := s.AllowGroup("mnt/storage", "/vip", "vip"); err != nil {
		t.Errorf("AllowGroup failed: %v", err)
	}
	if !s.Permitted("mnt/storage", "/vip", "bob") {
		t.Error("bob should be permitted (in allowed group)")
	}
	if !s.Permitted("mnt/storage", "/vip", "alice") {
		t.Error("alice should be permitted (default allow behavior when DenyByDefault=false)")
	}
}

func TestPermitted_NoRule(t *testing.T) {
	setupTestSources()
	s, _ := createTestStorage(t)
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	if !s.Permitted("mnt/storage", "/public", "anyone") {
		t.Error("anyone should be permitted if no rule exists")
	}
}

func TestPermitted_CombinedRules(t *testing.T) {
	setupTestSources()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "alice")
	createTestUser(t, userStore, "bob")
	createTestUser(t, userStore, "carol")
	createTestUser(t, userStore, "eve")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	err = s.AddUserToGroup("vip", "bob")
	if err != nil {
		t.Errorf("AddUserToGroup failed: %v", err)
	}
	err = s.AddUserToGroup("admins", "alice")
	if err != nil {
		t.Errorf("AddUserToGroup failed: %v", err)
	}
	if err := s.DenyUser("mnt/storage", "/combo", "eve"); err != nil {
		t.Errorf("DenyUser failed: %v", err)
	}
	if err := s.AllowUser("mnt/storage", "/combo", "carol"); err != nil {
		t.Errorf("AllowUser failed: %v", err)
	}
	if err := s.DenyGroup("mnt/storage", "/combo", "admins"); err != nil {
		t.Errorf("DenyGroup failed: %v", err)
	}
	if err := s.AllowGroup("mnt/storage", "/combo", "vip"); err != nil {
		t.Errorf("AllowGroup failed: %v", err)
	}
	if s.Permitted("mnt/storage", "/combo", "eve") {
		t.Error("eve should not be permitted (user denied)")
	}
	if !s.Permitted("mnt/storage", "/combo", "carol") {
		t.Error("carol should be permitted (user allowed)")
	}
	if s.Permitted("mnt/storage", "/combo", "alice") {
		t.Error("alice should not be permitted (in group denied)")
	}
	if !s.Permitted("mnt/storage", "/combo", "bob") {
		t.Error("bob should be permitted (in group allowed)")
	}
}

func TestPermitted_DenyAll(t *testing.T) {
	setupTestSources()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "alice")
	createTestUser(t, userStore, "bob")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// Test DenyAll
	if err = s.DenyAll("mnt/storage", "/private"); err != nil {
		t.Errorf("DenyAll failed: %v", err)
	}
	if s.Permitted("mnt/storage", "/private", "alice") {
		t.Error("alice should not be permitted (deny all)")
	}
	if s.Permitted("mnt/storage", "/private", "bob") {
		t.Error("bob should not be permitted (deny all)")
	}

	// Test that Allow rule overrides DenyAll
	if err = s.AllowUser("mnt/storage", "/private", "alice"); err != nil {
		t.Errorf("AllowUser failed: %v", err)
	}
	if !s.Permitted("mnt/storage", "/private", "alice") {
		t.Error("alice should be permitted (allow overrides deny all)")
	}

	// Test removing DenyAll
	removed, err := s.RemoveDenyAll("mnt/storage", "/private")
	if err != nil {
		t.Errorf("RemoveDenyAll failed: %v", err)
	}
	if !removed {
		t.Error("RemoveDenyAll should have removed the rule")
	}

	// After removing DenyAll, alice should be permitted due to the Allow rule
	if !s.Permitted("mnt/storage", "/private", "alice") {
		t.Error("alice should be permitted after removing deny all")
	}

	// Bob should be permitted because DenyByDefault=false makes allow lists additive (not exclusive)
	if !s.Permitted("mnt/storage", "/private", "bob") {
		t.Error("bob should be permitted after removing deny all (DenyByDefault=false makes allow lists additive)")
	}
}

func TestPermitted_DenyByDefault(t *testing.T) {
	// Clear access cache to prevent test pollution
	access.ClearCache()

	// Create isolated storage for this test
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "alice")
	createTestUser(t, userStore, "bob")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// Setup test configuration with DenyByDefault enabled for one source
	originalSourceMap := settings.Config.Server.SourceMap
	defer func() {
		settings.Config.Server.SourceMap = originalSourceMap
		access.ClearCache()
	}()

	settings.Config.Server.SourceMap = map[string]settings.Source{
		"mnt/storage": {
			Path: "mnt/storage",
			Name: "storage",
			Config: settings.SourceConfig{
				DenyByDefault: true,
			},
		},
		"mnt/open": {
			Path: "mnt/open",
			Name: "open",
			Config: settings.SourceConfig{
				DenyByDefault: false,
			},
		},
	}

	// Test DenyByDefault: When no rules exist, users should be denied
	if s.Permitted("mnt/storage", "/private", "alice") {
		t.Error("alice should not be permitted (DenyByDefault is true and no rules exist)")
	}
	if s.Permitted("mnt/storage", "/private", "bob") {
		t.Error("bob should not be permitted (DenyByDefault is true and no rules exist)")
	}

	// Test that explicit allow rules override DenyByDefault
	if err := s.AllowUser("mnt/storage", "/allowed", "alice"); err != nil {
		t.Errorf("AllowUser failed: %v", err)
	}
	if !s.Permitted("mnt/storage", "/allowed", "alice") {
		t.Error("alice should be permitted (explicit allow rule overrides DenyByDefault)")
	}
	if s.Permitted("mnt/storage", "/allowed", "bob") {
		t.Error("bob should not be permitted (not in allow list and DenyByDefault is true)")
	}

	// Test that explicit deny rules work with DenyByDefault
	if err := s.DenyUser("mnt/storage", "/denied", "alice"); err != nil {
		t.Errorf("DenyUser failed: %v", err)
	}
	if s.Permitted("mnt/storage", "/denied", "alice") {
		t.Error("alice should not be permitted (explicit deny rule)")
	}
	if s.Permitted("mnt/storage", "/denied", "bob") {
		t.Error("bob should not be permitted (DenyByDefault is true)")
	}

	// Test that DenyByDefault doesn't affect sources where it's disabled
	if !s.Permitted("mnt/open", "/public", "alice") {
		t.Error("alice should be permitted in open source (DenyByDefault is false)")
	}
	if !s.Permitted("mnt/open", "/public", "bob") {
		t.Error("bob should be permitted in open source (DenyByDefault is false)")
	}

	// Test recursive path checking with DenyByDefault
	// Allow alice at parent path, should work for child paths too
	if err := s.AllowUser("mnt/storage", "/parent", "alice"); err != nil {
		t.Errorf("AllowUser failed: %v", err)
	}
	if !s.Permitted("mnt/storage", "/parent/child", "alice") {
		t.Error("alice should be permitted for child path (inherits from parent allow rule)")
	}
	if s.Permitted("mnt/storage", "/parent/child", "bob") {
		t.Error("bob should not be permitted for child path (DenyByDefault and not in allow list)")
	}

	// Test that group rules work with DenyByDefault
	_ = s.AddUserToGroup("vip", "alice")
	if err := s.AllowGroup("mnt/storage", "/vip", "vip"); err != nil {
		t.Errorf("AllowGroup failed: %v", err)
	}
	if !s.Permitted("mnt/storage", "/vip", "alice") {
		t.Error("alice should be permitted (in allowed group)")
	}
	if s.Permitted("mnt/storage", "/vip", "bob") {
		t.Error("bob should not be permitted (not in allowed group and DenyByDefault is true)")
	}
}

func TestPermitted_DenyByDefaultWithDenyAll(t *testing.T) {
	// Clear access cache to prevent test pollution
	access.ClearCache()

	// Create isolated storage for this test
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "alice")
	createTestUser(t, userStore, "bob")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// Setup test configuration with DenyByDefault enabled
	originalSourceMap := settings.Config.Server.SourceMap
	defer func() {
		settings.Config.Server.SourceMap = originalSourceMap
		access.ClearCache()
	}()

	settings.Config.Server.SourceMap = map[string]settings.Source{
		"mnt/storage": {
			Path: "mnt/storage",
			Name: "storage",
			Config: settings.SourceConfig{
				DenyByDefault: true,
			},
		},
	}

	// Test that explicit DenyAll rule also works when DenyByDefault is enabled
	if err = s.DenyAll("mnt/storage", "/restricted"); err != nil {
		t.Errorf("DenyAll failed: %v", err)
	}
	if s.Permitted("mnt/storage", "/restricted", "alice") {
		t.Error("alice should not be permitted (DenyAll rule)")
	}
	if s.Permitted("mnt/storage", "/restricted", "bob") {
		t.Error("bob should not be permitted (DenyAll rule)")
	}

	// Test that Allow rule overrides DenyAll even with DenyByDefault
	if err = s.AllowUser("mnt/storage", "/restricted", "alice"); err != nil {
		t.Errorf("AllowUser failed: %v", err)
	}
	if !s.Permitted("mnt/storage", "/restricted", "alice") {
		t.Error("alice should be permitted (allow overrides DenyAll)")
	}

	// Remove DenyAll and test that DenyByDefault takes effect
	removed, err := s.RemoveDenyAll("mnt/storage", "/restricted")
	if err != nil {
		t.Errorf("RemoveDenyAll failed: %v", err)
	}
	if !removed {
		t.Error("RemoveDenyAll should have removed the rule")
	}

	// After removing DenyAll, alice should be permitted due to Allow rule
	if !s.Permitted("mnt/storage", "/restricted", "alice") {
		t.Error("alice should be permitted after removing DenyAll (has allow rule)")
	}
	// Bob should not be permitted due to DenyByDefault
	if s.Permitted("mnt/storage", "/restricted", "bob") {
		t.Error("bob should not be permitted (DenyByDefault is true and no allow rule)")
	}
}
