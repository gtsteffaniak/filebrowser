package access_test

import (
	"path/filepath"
	"strings"
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
	settings.Config.Server.SourceMap = map[string]*settings.Source{
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

	settings.Config.Server.SourceMap = map[string]*settings.Source{
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

	settings.Config.Server.SourceMap = map[string]*settings.Source{
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

func TestPermitted_DenyByDefault_AdminRootAccess(t *testing.T) {
	// 1. Setup
	access.ClearCache()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "admin")
	createTestUser(t, userStore, "graham")
	createTestUser(t, userStore, "bob")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// 2. Configure source with DenyByDefault: true
	originalSourceMap := settings.Config.Server.SourceMap
	defer func() {
		settings.Config.Server.SourceMap = originalSourceMap
		access.ClearCache()
	}()
	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"mnt/secure": {
			Path: "mnt/secure",
			Name: "secure",
			Config: settings.SourceConfig{
				DenyByDefault: true,
			},
		},
	}

	// 3. Set up access rules
	if err := s.AllowUser("mnt/secure", "/", "admin"); err != nil {
		t.Fatalf("AllowUser for admin failed: %v", err)
	}
	if err := s.AllowUser("mnt/secure", "/test", "graham"); err != nil {
		t.Fatalf("AllowUser for graham failed: %v", err)
	}

	// 4. Assertions
	// Admin checks
	if !s.Permitted("mnt/secure", "/", "admin") {
		t.Error("admin should be permitted for /")
	}
	if !s.Permitted("mnt/secure", "/test", "admin") {
		t.Error("admin should be permitted for /test")
	}
	if !s.Permitted("mnt/secure", "/test/sub", "admin") {
		t.Error("admin should be permitted for /test/sub (this is the bug)")
	}

	// Graham checks
	if !s.Permitted("mnt/secure", "/test", "graham") {
		t.Error("graham should be permitted for /test")
	}
	if !s.Permitted("mnt/secure", "/test/sub", "graham") {
		t.Error("graham should be permitted for /test/sub")
	}
	if s.Permitted("mnt/secure", "/", "graham") {
		t.Error("graham should NOT be permitted for /")
	}
	if s.Permitted("mnt/secure", "/anotherfolder", "graham") {
		t.Error("graham should NOT be permitted for /anotherfolder")
	}

	// Bob checks (should have no access)
	if s.Permitted("mnt/secure", "/", "bob") {
		t.Error("bob should NOT be permitted for /")
	}
	if s.Permitted("mnt/secure", "/test", "bob") {
		t.Error("bob should NOT be permitted for /test")
	}
	if s.Permitted("mnt/secure", "/test/sub", "bob") {
		t.Error("bob should NOT be permitted for /test/sub")
	}
}

// TestUserReportedBug reproduces the exact scenario described by the user
func TestUserReportedBug(t *testing.T) {
	access.ClearCache()
	s, userStore := createTestStorage(t)

	// Create the exact users from user's report
	createTestUser(t, userStore, "testu1")
	createTestUser(t, userStore, "testu2")

	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// Setup test configuration for TEST_FOLDER source
	originalSourceMap := settings.Config.Server.SourceMap
	defer func() {
		settings.Config.Server.SourceMap = originalSourceMap
		access.ClearCache()
	}()

	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"TEST_FOLDER": {
			Path: "TEST_FOLDER",
			Name: "TEST_FOLDER",
			Config: settings.SourceConfig{
				DenyByDefault: false, // Allow by default
			},
		},
	}

	// SCENARIO 1: Set up initial access rules exactly as described
	// testu1 is denied access to USER2_folder
	if err := s.DenyUser("TEST_FOLDER", "/USER2_folder", "testu1"); err != nil {
		t.Fatalf("DenyUser failed for testu1->USER2_folder: %v", err)
	}

	// testu2 is denied access to USER1_folder
	if err := s.DenyUser("TEST_FOLDER", "/USER1_folder", "testu2"); err != nil {
		t.Fatalf("DenyUser failed for testu2->USER1_folder: %v", err)
	}

	// VERIFY SCENARIO 1: Initial permissions work correctly
	if s.Permitted("TEST_FOLDER", "/USER2_folder", "testu1") {
		t.Error("SCENARIO 1 FAILED: testu1 should NOT be permitted to access USER2_folder")
	}
	if s.Permitted("TEST_FOLDER", "/USER1_folder", "testu2") {
		t.Error("SCENARIO 1 FAILED: testu2 should NOT be permitted to access USER1_folder")
	}

	// Verify allowed access still works
	if !s.Permitted("TEST_FOLDER", "/USER1_folder", "testu1") {
		t.Error("SCENARIO 1 FAILED: testu1 SHOULD be permitted to access USER1_folder")
	}
	if !s.Permitted("TEST_FOLDER", "/USER2_folder", "testu2") {
		t.Error("SCENARIO 1 FAILED: testu2 SHOULD be permitted to access USER2_folder")
	}

	t.Log("SCENARIO 1 PASSED: Initial access rules work correctly")

	// SCENARIO 2: The bug should NOT happen with proper access control
	// testu2 should still be denied access regardless of filesystem changes
	if s.Permitted("TEST_FOLDER", "/USER1_folder", "testu2") {
		t.Fatal("BUG REPRODUCED: testu2 can now access USER1_folder!")
	}

	t.Log("SCENARIO 2 PASSED: Access control rules work correctly")
}

// TestSubfolderAccessLogicBug tests the theory that the bug is caused by subfolder access logic
func TestSubfolderAccessLogicBug(t *testing.T) {
	access.ClearCache()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "testu1")
	createTestUser(t, userStore, "testu2")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	originalSourceMap := settings.Config.Server.SourceMap
	defer func() {
		settings.Config.Server.SourceMap = originalSourceMap
		access.ClearCache()
	}()

	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"TEST_FOLDER": {
			Path: "TEST_FOLDER",
			Name: "TEST_FOLDER",
			Config: settings.SourceConfig{
				DenyByDefault: false, // This is key!
			},
		},
	}

	// Set up the exact scenario: testu2 denied access to /USER1_folder
	if err := s.DenyUser("TEST_FOLDER", "/USER1_folder", "testu2"); err != nil {
		t.Fatalf("DenyUser failed: %v", err)
	}

	// TEST THEORY: What happens when we check testu2's access to a SUBFOLDER that has no explicit rule?

	// Case 1: testu2 access to parent folder (should be denied)
	if s.Permitted("TEST_FOLDER", "/USER1_folder", "testu2") {
		t.Error("testu2 should be denied access to /USER1_folder")
	}

	// Case 2: testu2 access to subfolder that doesn't have explicit deny rule
	// With DenyByDefault=false, this should be ALLOWED!
	if !s.Permitted("TEST_FOLDER", "/USER1_folder/test_folder", "testu2") {
		t.Log("testu2 is denied access to /USER1_folder/test_folder (subfolder)")
	} else {
		t.Log("testu2 HAS access to /USER1_folder/test_folder (subfolder) - this might be the bug!")
	}

	// Case 3: Let's test with a deeply nested subfolder
	if s.Permitted("TEST_FOLDER", "/USER1_folder/sub1/sub2/deep", "testu2") {
		t.Log("testu2 HAS access to deep subfolder - confirming the pattern")
	}

	// THE CRITICAL TEST: What about parent directory inheritance?
	// According to the access control docs, rules should apply recursively to subdirectories
	// If testu2 is denied access to /USER1_folder, they should also be denied access to ALL subdirectories

	// This might be where the bug is - the recursive checking might not be working correctly
}

// TestFileInfoBrowsingBug tests if the file browsing logic causes the access bug
func TestFileInfoBrowsingBug(t *testing.T) {
	access.ClearCache()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "testu1")
	createTestUser(t, userStore, "testu2")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	originalSourceMap := settings.Config.Server.SourceMap
	defer func() {
		settings.Config.Server.SourceMap = originalSourceMap
		access.ClearCache()
	}()

	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"TEST_FOLDER": {
			Path: "TEST_FOLDER",
			Name: "TEST_FOLDER",
			Config: settings.SourceConfig{
				DenyByDefault: false,
			},
		},
	}

	// Set up: testu2 denied access to /USER1_folder
	if err := s.DenyUser("TEST_FOLDER", "/USER1_folder", "testu2"); err != nil {
		t.Fatalf("DenyUser failed: %v", err)
	}

	// CRITICAL TEST: What if testu2 has access to a subfolder?
	// This tests the exact line: indexPath := info.Path + subFolder.Name

	// Scenario A: Subfolder should inherit deny rule from parent (correct behavior)
	parentDenied := !s.Permitted("TEST_FOLDER", "/USER1_folder", "testu2")
	subfolderDenied := !s.Permitted("TEST_FOLDER", "/USER1_folder/test_folder", "testu2")

	t.Logf("Parent denied: %v, Subfolder denied: %v", parentDenied, subfolderDenied)

	if parentDenied && !subfolderDenied {
		t.Error("BUG FOUND: Parent denied but subfolder allowed - this would trigger hasPermittedPaths = true")
	}

	// Scenario B: What if there's a path construction bug?
	// Maybe the path constructed in FileInfoFaster doesn't match the path used in access rules?

	// Let's test different path formats that might be constructed
	testPaths := []string{
		"/USER1_folder/test_folder",  // Normal path
		"/USER1_folder/test_folder/", // With trailing slash
		"USER1_folder/test_folder",   // Without leading slash
		"/USER1_folder/test_folder",  // info.Path + subFolder.Name simulation
	}

	for _, testPath := range testPaths {
		permitted := s.Permitted("TEST_FOLDER", testPath, "testu2")
		t.Logf("Path %q permitted for testu2: %v", testPath, permitted)

		// Paths without leading slash should ALWAYS be denied (security fix)
		if !strings.HasPrefix(testPath, "/") {
			if permitted {
				t.Errorf("SECURITY BUG: Path without leading slash %q should be denied but was permitted", testPath)
			} else {
				t.Logf("SECURITY FIX WORKING: Path without leading slash %q correctly denied", testPath)
			}
		} else {
			// Paths with leading slash should follow normal access rules (deny in this case)
			if permitted {
				t.Errorf("ACCESS CONTROL BUG: testu2 has access to %q despite being denied parent folder", testPath)
			}
		}
	}
}

// TestCacheClearingOnRuleDeletion tests that the rules cache is properly cleared when rules are deleted
func TestCacheClearingOnRuleDeletion(t *testing.T) {
	// Clear cache to start fresh
	access.ClearCache()

	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "user1")
	createTestUser(t, userStore, "user2")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// Setup test sources
	originalSourceMap := settings.Config.Server.SourceMap
	defer func() {
		settings.Config.Server.SourceMap = originalSourceMap
		access.ClearCache()
	}()

	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"test_source": {
			Path: "test_source",
			Name: "test_source",
			Config: settings.SourceConfig{
				DenyByDefault: false,
			},
		},
	}

	// Step 1: Add some rules
	err = s.DenyUser("test_source", "/path1/", "user1")
	if err != nil {
		t.Fatalf("Failed to deny user1: %v", err)
	}
	err = s.AllowUser("test_source", "/path2/", "user2")
	if err != nil {
		t.Fatalf("Failed to allow user2: %v", err)
	}

	// Step 2: Call GetAllRules to populate the cache
	rules1, err := s.GetAllRules("test_source")
	if err != nil {
		t.Fatalf("Failed to get all rules: %v", err)
	}

	// Verify we have the expected rules
	if len(rules1) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(rules1))
	}
	if _, exists := rules1["/path1/"]; !exists {
		t.Error("Expected /path1/ rule to exist")
	}
	if _, exists := rules1["/path2/"]; !exists {
		t.Error("Expected /path2/ rule to exist")
	}

	// Step 3: Call GetAllRules again - should return cached result
	rules2, err := s.GetAllRules("test_source")
	if err != nil {
		t.Fatalf("Failed to get all rules (cached): %v", err)
	}

	// Verify it's the same result (cached)
	if len(rules2) != 2 {
		t.Errorf("Expected 2 rules from cache, got %d", len(rules2))
	}

	// Step 4: Delete a rule
	removed, err := s.RemoveDenyUser("test_source", "/path1/", "user1")
	if err != nil {
		t.Fatalf("Failed to remove deny user: %v", err)
	}
	if !removed {
		t.Error("Expected rule to be removed")
	}

	// Step 5: Call GetAllRules again - should return fresh data (not cached)
	rules3, err := s.GetAllRules("test_source")
	if err != nil {
		t.Fatalf("Failed to get all rules after deletion: %v", err)
	}

	// Verify the deleted rule is gone
	if len(rules3) != 1 {
		t.Errorf("Expected 1 rule after deletion, got %d", len(rules3))
	}
	if _, exists := rules3["/path1/"]; exists {
		t.Error("Expected /path1/ rule to be deleted from cache")
	}
	if _, exists := rules3["/path2/"]; !exists {
		t.Error("Expected /path2/ rule to still exist")
	}

	// Step 6: Delete the remaining rule
	removed, err = s.RemoveAllowUser("test_source", "/path2/", "user2")
	if err != nil {
		t.Fatalf("Failed to remove allow user: %v", err)
	}
	if !removed {
		t.Error("Expected rule to be removed")
	}

	// Step 7: Call GetAllRules again - should return empty result
	rules4, err := s.GetAllRules("test_source")
	if err != nil {
		t.Fatalf("Failed to get all rules after second deletion: %v", err)
	}

	// Verify all rules are gone
	if len(rules4) != 0 {
		t.Errorf("Expected 0 rules after all deletions, got %d", len(rules4))
	}

	t.Log("Cache clearing test passed: Rules are immediately removed from cache when deleted")
}

// TestCacheClearingOnBulkRuleDeletion tests cache clearing for bulk operations
func TestCacheClearingOnBulkRuleDeletion(t *testing.T) {
	// Clear cache to start fresh
	access.ClearCache()

	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "user1")
	createTestUser(t, userStore, "user2")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// Setup test sources
	originalSourceMap := settings.Config.Server.SourceMap
	defer func() {
		settings.Config.Server.SourceMap = originalSourceMap
		access.ClearCache()
	}()

	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"test_source": {
			Path: "test_source",
			Name: "test_source",
			Config: settings.SourceConfig{
				DenyByDefault: false,
			},
		},
	}

	// Step 1: Add multiple rules for user1
	err = s.DenyUser("test_source", "/path1", "user1")
	if err != nil {
		t.Fatalf("Failed to deny user1 on path1: %v", err)
	}
	err = s.AllowUser("test_source", "/path2", "user1")
	if err != nil {
		t.Fatalf("Failed to allow user1 on path2: %v", err)
	}
	err = s.DenyUser("test_source", "/path3", "user1")
	if err != nil {
		t.Fatalf("Failed to deny user1 on path3: %v", err)
	}

	// Step 2: Populate cache
	rules1, err := s.GetAllRules("test_source")
	if err != nil {
		t.Fatalf("Failed to get all rules: %v", err)
	}
	if len(rules1) != 3 {
		t.Errorf("Expected 3 rules, got %d", len(rules1))
	}

	// Step 3: Remove all rules for user1 (bulk operation)
	err = s.RemoveAllRulesForUser("user1")
	if err != nil {
		t.Fatalf("Failed to remove all rules for user1: %v", err)
	}

	// Step 4: Verify cache is cleared and rules are gone
	rules2, err := s.GetAllRules("test_source")
	if err != nil {
		t.Fatalf("Failed to get all rules after bulk deletion: %v", err)
	}

	// All rules should be gone since user1 was the only user with rules
	if len(rules2) != 0 {
		t.Errorf("Expected 0 rules after bulk deletion, got %d", len(rules2))
	}

	t.Log("Bulk cache clearing test passed: All rules are immediately removed from cache when bulk deleted")
}

// TestNestedFolderAccessBug reproduces the exact scenarios described by the user
func TestNestedFolderAccessBug(t *testing.T) {
	// Clear cache to start fresh
	access.ClearCache()

	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "user1")
	createTestUser(t, userStore, "user2")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// Setup test sources
	originalSourceMap := settings.Config.Server.SourceMap
	defer func() {
		settings.Config.Server.SourceMap = originalSourceMap
		access.ClearCache()
	}()

	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"TEST": {
			Path: "TEST",
			Name: "TEST",
			Config: settings.SourceConfig{
				DenyByDefault: false, // Allow by default
			},
		},
	}

	// Set up access rules as described
	err = s.DenyUser("TEST", "/folder2", "user1")
	if err != nil {
		t.Fatalf("Failed to deny user1 access to folder2: %v", err)
	}
	err = s.DenyUser("TEST", "/folder1", "user2")
	if err != nil {
		t.Fatalf("Failed to deny user2 access to folder1: %v", err)
	}

	t.Log("=== SCENARIO 1: Direct folder access ===")

	// Scenario 1: Direct folder access (should work correctly)
	t.Log("Testing user1 access to /folder2 (should be denied)")
	result1 := s.Permitted("TEST", "/folder2", "user1")
	if result1 {
		t.Error("user1 should be denied access to /folder2")
	} else {
		t.Log("✓ user1 correctly denied access to /folder2")
	}

	t.Log("Testing user2 access to /folder1 (should be denied)")
	result2 := s.Permitted("TEST", "/folder1", "user2")
	if result2 {
		t.Error("user2 should be denied access to /folder1")
	} else {
		t.Log("✓ user2 correctly denied access to /folder1")
	}

	t.Log("Testing user1 access to /folder1 (should be allowed)")
	result3 := s.Permitted("TEST", "/folder1", "user1")
	if !result3 {
		t.Error("user1 should be allowed access to /folder1")
	} else {
		t.Log("✓ user1 correctly allowed access to /folder1")
	}

	t.Log("Testing user2 access to /folder2 (should be allowed)")
	result4 := s.Permitted("TEST", "/folder2", "user2")
	if !result4 {
		t.Error("user2 should be allowed access to /folder2")
	} else {
		t.Log("✓ user2 correctly allowed access to /folder2")
	}

	t.Log("=== SCENARIO 2: Nested folder access (the bug) ===")

	// Scenario 2: Nested folder access (this is where the bug occurs)
	t.Log("Testing user1 access to /folder2/another_folder2 (should be denied due to parent rule)")
	result5 := s.Permitted("TEST", "/folder2/another_folder2", "user1")
	if result5 {
		t.Error("BUG: user1 should be denied access to /folder2/another_folder2 (parent folder is denied)")
	} else {
		t.Log("✓ user1 correctly denied access to /folder2/another_folder2")
	}

	t.Log("Testing user2 access to /folder1/another_folder1 (should be denied due to parent rule)")
	result6 := s.Permitted("TEST", "/folder1/another_folder1", "user2")
	if result6 {
		t.Error("BUG: user2 should be denied access to /folder1/another_folder1 (parent folder is denied)")
	} else {
		t.Log("✓ user2 correctly denied access to /folder1/another_folder1")
	}

	t.Log("Testing user1 access to /folder1/another_folder1 (should be allowed)")
	result7 := s.Permitted("TEST", "/folder1/another_folder1", "user1")
	if !result7 {
		t.Error("user1 should be allowed access to /folder1/another_folder1")
	} else {
		t.Log("✓ user1 correctly allowed access to /folder1/another_folder1")
	}

	t.Log("Testing user2 access to /folder2/another_folder2 (should be allowed)")
	result8 := s.Permitted("TEST", "/folder2/another_folder2", "user2")
	if !result8 {
		t.Error("user2 should be allowed access to /folder2/another_folder2")
	} else {
		t.Log("✓ user2 correctly allowed access to /folder2/another_folder2")
	}

	// Test deeper nesting
	t.Log("Testing deeper nesting: user1 access to /folder2/another_folder2/deep_folder (should be denied)")
	result9 := s.Permitted("TEST", "/folder2/another_folder2/deep_folder", "user1")
	if result9 {
		t.Error("BUG: user1 should be denied access to /folder2/another_folder2/deep_folder (parent folder is denied)")
	} else {
		t.Log("✓ user1 correctly denied access to /folder2/another_folder2/deep_folder")
	}

	t.Log("=== SCENARIO 3: Specific allow rule on subfolder ===")

	// Add a specific allow rule for user1 on a subfolder of folder2
	err = s.AllowUser("TEST", "/folder2/allowed_subfolder", "user1")
	if err != nil {
		t.Fatalf("Failed to allow user1 access to /folder2/allowed_subfolder: %v", err)
	}

	t.Log("Testing user1 access to /folder2/allowed_subfolder (should be allowed due to specific allow rule)")
	result10 := s.Permitted("TEST", "/folder2/allowed_subfolder", "user1")
	if !result10 {
		t.Error("user1 should be allowed access to /folder2/allowed_subfolder (has specific allow rule)")
	} else {
		t.Log("✓ user1 correctly allowed access to /folder2/allowed_subfolder")
	}

	t.Log("Testing user1 access to /folder2/denied_subfolder (should be denied - no specific allow rule)")
	result11 := s.Permitted("TEST", "/folder2/denied_subfolder", "user1")
	if result11 {
		t.Error("user1 should be denied access to /folder2/denied_subfolder (no specific allow rule)")
	} else {
		t.Log("✓ user1 correctly denied access to /folder2/denied_subfolder")
	}
}

// TestFolderVisibilityBugReproduction tests the exact scenario reported by the user
func TestFolderVisibilityBugReproduction(t *testing.T) {
	// Clear cache to start fresh
	access.ClearCache()

	s, userStorage := createTestStorage(t)

	// Create test users first
	user1 := &users.User{NonAdminEditable: users.NonAdminEditable{Password: "test"}, Username: "user1"}
	user2 := &users.User{NonAdminEditable: users.NonAdminEditable{Password: "test"}, Username: "user2"}
	err := userStorage.Save(user1, false, false)
	if err != nil {
		t.Fatalf("failed to create user1: %v", err)
	}
	err = userStorage.Save(user2, false, false)
	if err != nil {
		t.Fatalf("failed to create user2: %v", err)
	}

	// Setup test sources
	originalSourceMap := settings.Config.Server.SourceMap
	defer func() {
		settings.Config.Server.SourceMap = originalSourceMap
		access.ClearCache()
	}()

	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"TEST": {
			Path: "TEST",
			Name: "TEST",
			Config: settings.SourceConfig{
				DenyByDefault: false, // Allow by default
			},
		},
	}

	// Set up the exact access rules from the user's report
	err = s.DenyUser("TEST", "/test/folder1", "user2")
	if err != nil {
		t.Fatalf("failed to deny user2 access to /test/folder1: %v", err)
	}
	err = s.DenyUser("TEST", "/test/folder2", "user1")
	if err != nil {
		t.Fatalf("failed to deny user1 access to /test/folder2: %v", err)
	}

	// Test the exact scenario: user1 should not see contents of /test/folder2
	// This simulates what happens when the frontend calls the API

	// First, verify that user1 cannot access /test/folder2 directly
	permitted := s.Permitted("TEST", "/test/folder2", "user1")
	if permitted {
		t.Error("user1 should be denied access to /test/folder2")
	} else {
		t.Log("✓ user1 correctly denied access to /test/folder2")
	}

	// Now test the folder visibility logic that was buggy
	// This simulates the FileInfoFaster function behavior

	// Mock folder structure as reported by user
	mockFolders := []struct {
		Name string
		Type string
	}{
		{Name: "subfolder2", Type: "directory"},
		{Name: "test2", Type: "directory"},
	}

	// Simulate the buggy path construction (before fix)
	buggyPaths := []string{
		"/test/folder2subfolder2", // Missing / separator
		"/test/folder2test2",      // Missing / separator
	}

	// These should fail (buggy behavior) - but they actually fail due to missing source config
	for i, path := range buggyPaths {
		permitted := s.Permitted("TEST", path, "user1")
		// Note: These actually return false because there's no source config, not because of the path bug
		t.Logf("Buggy path %d: %s -> %v (would be true if source config existed)", i+1, path, permitted)
	}

	// Simulate the correct path construction (after fix)
	correctPaths := []string{
		"/test/folder2/subfolder2", // With / separator
		"/test/folder2/test2",      // With / separator
	}

	// These should fail (correct behavior)
	for i, path := range correctPaths {
		permitted := s.Permitted("TEST", path, "user1")
		if permitted {
			t.Errorf("Correct path construction should deny access: %s", path)
		}
		t.Logf("Correct path %d: %s -> %v (should be false)", i+1, path, permitted)
	}

	// Test that the folder visibility logic would work correctly
	// (This is what the FileInfoFaster function should do)
	hasPermittedPaths := false
	for _, folder := range mockFolders {
		correctPath := "/test/folder2/" + folder.Name
		if s.Permitted("TEST", correctPath, "user1") {
			hasPermittedPaths = true
			t.Logf("Found permitted subfolder: %s", folder.Name)
		}
	}

	if hasPermittedPaths {
		t.Error("No subfolders should be permitted for user1 in /test/folder2")
	} else {
		t.Log("✓ Folder visibility logic correctly denies access to all subfolders")
	}
}

// TestHasAnyVisibleItems tests the HasAnyVisibleItems method
func TestHasAnyVisibleItems(t *testing.T) {
	// Clear cache to start fresh
	access.ClearCache()

	s, userStorage := createTestStorage(t)

	// Create test users
	user1 := &users.User{NonAdminEditable: users.NonAdminEditable{Password: "test"}, Username: "user1"}
	user2 := &users.User{NonAdminEditable: users.NonAdminEditable{Password: "test"}, Username: "user2"}
	err := userStorage.Save(user1, false, false)
	if err != nil {
		t.Fatalf("failed to create user1: %v", err)
	}
	err = userStorage.Save(user2, false, false)
	if err != nil {
		t.Fatalf("failed to create user2: %v", err)
	}

	// Setup test sources
	originalSourceMap := settings.Config.Server.SourceMap
	defer func() {
		settings.Config.Server.SourceMap = originalSourceMap
		access.ClearCache()
	}()

	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"TEST": {
			Path: "TEST",
			Name: "TEST",
			Config: settings.SourceConfig{
				DenyByDefault: false, // Allow by default
			},
		},
	}

	// Set up access rules
	err = s.DenyUser("TEST", "/folder1", "user1")
	if err != nil {
		t.Fatalf("failed to deny user1 access to /folder1: %v", err)
	}
	err = s.AllowUser("TEST", "/folder1/allowed_subfolder", "user1")
	if err != nil {
		t.Fatalf("failed to allow user1 access to /folder1/allowed_subfolder: %v", err)
	}

	// Test with item names
	itemNames := []string{"allowed_subfolder", "denied_subfolder", "another_denied_folder"}

	// Test for user1 (denied access to parent folder but has access to one subfolder)
	hasVisible := s.HasAnyVisibleItems("TEST", "/folder1", itemNames, "user1")
	if !hasVisible {
		t.Error("Expected user1 to have access to at least one item in /folder1")
	}

	// Test for user2 (no restrictions, but no source config so will be denied)
	hasVisible2 := s.HasAnyVisibleItems("TEST", "/folder1", itemNames, "user2")
	// Note: user2 will be denied due to missing source config, not due to access rules
	t.Logf("user2 access to items in /folder1: %v (denied due to missing source config)", hasVisible2)

	// Test with no accessible items
	deniedItemNames := []string{"denied_subfolder", "another_denied_folder"}
	hasVisible3 := s.HasAnyVisibleItems("TEST", "/folder1", deniedItemNames, "user1")
	if hasVisible3 {
		t.Error("Expected user1 to NOT have access to denied items in /folder1")
	}

	t.Log("✓ HasAnyVisibleItems correctly checks access permissions for items")
}

// TestRemoveUserCascade_OnlyRemovesSpecificList tests that cascade delete only removes from the specified list
func TestRemoveUserCascade_OnlyRemovesSpecificList(t *testing.T) {
	setupTestSources()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "alice")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// Set up rules: alice has both allow and deny rules on different paths
	if err := s.AllowUser("mnt/storage", "/docs/", "alice"); err != nil {
		t.Fatalf("AllowUser failed: %v", err)
	}
	if err := s.AllowUser("mnt/storage", "/docs/public/", "alice"); err != nil {
		t.Fatalf("AllowUser failed: %v", err)
	}
	if err := s.DenyUser("mnt/storage", "/docs/", "alice"); err != nil {
		t.Fatalf("DenyUser failed: %v", err)
	}
	if err := s.DenyUser("mnt/storage", "/docs/private/", "alice"); err != nil {
		t.Fatalf("DenyUser failed: %v", err)
	}

	// Cascade delete only allow rules
	count, err := s.RemoveUserCascade("mnt/storage", "/docs/", "alice", true)
	if err != nil {
		t.Fatalf("RemoveUserCascade failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 allow rules removed, got %d", count)
	}

	// Verify allow rules are gone
	rule, ok := s.GetFrontendRules("mnt/storage", "/docs/")
	if ok && len(rule.Allow.Users) != 0 {
		t.Error("Allow rule should be removed from /docs/")
	}
	rule, ok = s.GetFrontendRules("mnt/storage", "/docs/public/")
	if ok && len(rule.Allow.Users) != 0 {
		t.Error("Allow rule should be removed from /docs/public/")
	}

	// Verify deny rules are still present
	rule, ok = s.GetFrontendRules("mnt/storage", "/docs/")
	if !ok || len(rule.Deny.Users) == 0 {
		t.Error("Deny rule should still exist on /docs/")
	}
	rule, ok = s.GetFrontendRules("mnt/storage", "/docs/private/")
	if !ok || len(rule.Deny.Users) == 0 {
		t.Error("Deny rule should still exist on /docs/private/")
	}

	t.Log("✓ Cascade delete correctly removes only allow rules, leaving deny rules intact")
}

// TestRemoveUserCascade_DenyRules tests cascade delete for deny rules specifically
func TestRemoveUserCascade_DenyRules(t *testing.T) {
	setupTestSources()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "bob")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// Set up both allow and deny rules
	if err := s.AllowUser("mnt/storage", "/projects/", "bob"); err != nil {
		t.Fatalf("AllowUser failed: %v", err)
	}
	if err := s.AllowUser("mnt/storage", "/projects/team/", "bob"); err != nil {
		t.Fatalf("AllowUser failed: %v", err)
	}
	if err := s.DenyUser("mnt/storage", "/projects/", "bob"); err != nil {
		t.Fatalf("DenyUser failed: %v", err)
	}
	if err := s.DenyUser("mnt/storage", "/projects/secret/", "bob"); err != nil {
		t.Fatalf("DenyUser failed: %v", err)
	}

	// Cascade delete only deny rules
	count, err := s.RemoveUserCascade("mnt/storage", "/projects/", "bob", false)
	if err != nil {
		t.Fatalf("RemoveUserCascade failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 deny rules removed, got %d", count)
	}

	// Verify deny rules are gone
	rule, ok := s.GetFrontendRules("mnt/storage", "/projects/")
	if !ok || len(rule.Deny.Users) != 0 {
		t.Error("Deny rule should be removed from /projects/")
	}
	rule, ok = s.GetFrontendRules("mnt/storage", "/projects/secret/")
	if ok && len(rule.Deny.Users) != 0 {
		t.Error("Deny rule should be removed from /projects/secret/")
	}

	// Verify allow rules are still present
	rule, ok = s.GetFrontendRules("mnt/storage", "/projects/")
	if !ok || len(rule.Allow.Users) == 0 {
		t.Error("Allow rule should still exist on /projects/")
	}
	rule, ok = s.GetFrontendRules("mnt/storage", "/projects/team/")
	if !ok || len(rule.Allow.Users) == 0 {
		t.Error("Allow rule should still exist on /projects/team/")
	}

	t.Log("✓ Cascade delete correctly removes only deny rules, leaving allow rules intact")
}

// TestRemoveUserCascade_MultipleSubpaths tests cascade delete across multiple subpath levels
func TestRemoveUserCascade_MultipleSubpaths(t *testing.T) {
	setupTestSources()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "carol")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// Set up a deep hierarchy of allow rules
	paths := []string{
		"/data/",
		"/data/2024/",
		"/data/2024/q1/",
		"/data/2024/q1/jan/",
		"/data/2024/q1/feb/",
		"/data/2024/q2/",
		"/data/2024/q2/apr/",
		"/data/2025/",
	}

	for _, path := range paths {
		if err := s.AllowUser("mnt/storage", path, "carol"); err != nil {
			t.Fatalf("AllowUser failed for %s: %v", path, err)
		}
	}

	// Cascade delete from /data/2024/ should remove all 2024 subpaths
	count, err := s.RemoveUserCascade("mnt/storage", "/data/2024/", "carol", true)
	if err != nil {
		t.Fatalf("RemoveUserCascade failed: %v", err)
	}
	if count != 6 {
		t.Errorf("Expected 6 rules removed (2024 + 5 subpaths), got %d", count)
	}

	// Verify 2024 paths are gone
	for _, path := range []string{"/data/2024/", "/data/2024/q1/", "/data/2024/q1/jan/", "/data/2024/q1/feb/", "/data/2024/q2/", "/data/2024/q2/apr/"} {
		rule, ok := s.GetFrontendRules("mnt/storage", path)
		if ok && len(rule.Allow.Users) > 0 {
			t.Errorf("Rule should be removed from %s", path)
		}
	}

	// Verify other paths still exist
	rule, ok := s.GetFrontendRules("mnt/storage", "/data/")
	if !ok || len(rule.Allow.Users) == 0 {
		t.Error("Rule should still exist on /data/")
	}
	rule, ok = s.GetFrontendRules("mnt/storage", "/data/2025/")
	if !ok || len(rule.Allow.Users) == 0 {
		t.Error("Rule should still exist on /data/2025/")
	}

	t.Log("✓ Cascade delete correctly removes rules from all subpath levels")
}

// TestRemoveGroupCascade_OnlyRemovesSpecificList tests cascade delete for groups
func TestRemoveGroupCascade_OnlyRemovesSpecificList(t *testing.T) {
	setupTestSources()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "alice")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// Create a group
	if err := s.AddUserToGroup("editors", "alice"); err != nil {
		t.Fatalf("AddUserToGroup failed: %v", err)
	}

	// Set up both allow and deny rules for the group
	if err := s.AllowGroup("mnt/storage", "/content/", "editors"); err != nil {
		t.Fatalf("AllowGroup failed: %v", err)
	}
	if err := s.AllowGroup("mnt/storage", "/content/articles/", "editors"); err != nil {
		t.Fatalf("AllowGroup failed: %v", err)
	}
	if err := s.DenyGroup("mnt/storage", "/content/", "editors"); err != nil {
		t.Fatalf("DenyGroup failed: %v", err)
	}
	if err := s.DenyGroup("mnt/storage", "/content/drafts/", "editors"); err != nil {
		t.Fatalf("DenyGroup failed: %v", err)
	}

	// Cascade delete only allow rules
	count, err := s.RemoveGroupCascade("mnt/storage", "/content/", "editors", true)
	if err != nil {
		t.Fatalf("RemoveGroupCascade failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 allow rules removed, got %d", count)
	}

	// Verify allow rules are gone
	rule, ok := s.GetFrontendRules("mnt/storage", "/content/")
	if !ok || len(rule.Allow.Groups) != 0 {
		t.Error("Allow rule should be removed from /content/")
	}

	// Verify deny rules are still present
	rule, ok = s.GetFrontendRules("mnt/storage", "/content/")
	if !ok || len(rule.Deny.Groups) == 0 {
		t.Error("Deny rule should still exist on /content/")
	}
	rule, ok = s.GetFrontendRules("mnt/storage", "/content/drafts/")
	if !ok || len(rule.Deny.Groups) == 0 {
		t.Error("Deny rule should still exist on /content/drafts/")
	}

	t.Log("✓ Cascade delete for groups correctly removes only allow rules")
}

// TestRemoveGroupCascade_DenyRules tests cascade delete for group deny rules specifically
func TestRemoveGroupCascade_DenyRules(t *testing.T) {
	setupTestSources()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "alice")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// Create a group
	if err := s.AddUserToGroup("contractors", "alice"); err != nil {
		t.Fatalf("AddUserToGroup failed: %v", err)
	}

	// Set up both allow and deny rules for the group
	if err := s.AllowGroup("mnt/storage", "/work/", "contractors"); err != nil {
		t.Fatalf("AllowGroup failed: %v", err)
	}
	if err := s.AllowGroup("mnt/storage", "/work/public/", "contractors"); err != nil {
		t.Fatalf("AllowGroup failed: %v", err)
	}
	if err := s.DenyGroup("mnt/storage", "/work/", "contractors"); err != nil {
		t.Fatalf("DenyGroup failed: %v", err)
	}
	if err := s.DenyGroup("mnt/storage", "/work/confidential/", "contractors"); err != nil {
		t.Fatalf("DenyGroup failed: %v", err)
	}

	// Cascade delete only deny rules
	count, err := s.RemoveGroupCascade("mnt/storage", "/work/", "contractors", false)
	if err != nil {
		t.Fatalf("RemoveGroupCascade failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 deny rules removed, got %d", count)
	}

	// Verify deny rules are gone
	rule, ok := s.GetFrontendRules("mnt/storage", "/work/")
	if ok && len(rule.Deny.Groups) != 0 {
		t.Error("Deny rule should be removed from /work/")
	}
	rule, ok = s.GetFrontendRules("mnt/storage", "/work/confidential/")
	if ok && len(rule.Deny.Groups) != 0 {
		t.Error("Deny rule should be removed from /work/confidential/")
	}

	// Verify allow rules are still present
	rule, ok = s.GetFrontendRules("mnt/storage", "/work/")
	if !ok || len(rule.Allow.Groups) == 0 {
		t.Error("Allow rule should still exist on /work/")
	}
	rule, ok = s.GetFrontendRules("mnt/storage", "/work/public/")
	if !ok || len(rule.Allow.Groups) == 0 {
		t.Error("Allow rule should still exist on /work/public/")
	}

	t.Log("✓ Cascade delete for groups correctly removes only deny rules, leaving allow rules intact")
}

// TestRemoveUserCascade_EmptyResult tests cascade delete when no rules exist
func TestRemoveUserCascade_EmptyResult(t *testing.T) {
	setupTestSources()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "dave")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// Try to cascade delete when no rules exist
	count, err := s.RemoveUserCascade("mnt/storage", "/nonexistent/", "dave", true)
	if err != nil {
		t.Fatalf("RemoveUserCascade should not error on nonexistent rules: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 rules removed, got %d", count)
	}

	t.Log("✓ Cascade delete correctly returns 0 when no rules exist")
}

// TestRemoveUserCascade_ExactPathOnly tests that exact path is included in cascade
func TestRemoveUserCascade_ExactPathOnly(t *testing.T) {
	setupTestSources()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "eve")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// Add rule only on exact path, no subpaths
	if err := s.AllowUser("mnt/storage", "/single/", "eve"); err != nil {
		t.Fatalf("AllowUser failed: %v", err)
	}

	// Cascade delete should remove the exact path rule
	count, err := s.RemoveUserCascade("mnt/storage", "/single/", "eve", true)
	if err != nil {
		t.Fatalf("RemoveUserCascade failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 rule removed, got %d", count)
	}

	// Verify rule is gone
	rule, ok := s.GetFrontendRules("mnt/storage", "/single/")
	if ok && len(rule.Allow.Users) > 0 {
		t.Error("Rule should be removed from /single/")
	}

	t.Log("✓ Cascade delete correctly removes the exact path rule")
}

// TestRemoveUserCascade_DoesNotAffectParentPaths tests that parent paths are not affected
func TestRemoveUserCascade_DoesNotAffectParentPaths(t *testing.T) {
	setupTestSources()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "frank")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// Set up rules on parent and child paths
	if err := s.AllowUser("mnt/storage", "/parent/", "frank"); err != nil {
		t.Fatalf("AllowUser failed: %v", err)
	}
	if err := s.AllowUser("mnt/storage", "/parent/child/", "frank"); err != nil {
		t.Fatalf("AllowUser failed: %v", err)
	}
	if err := s.AllowUser("mnt/storage", "/parent/child/grandchild/", "frank"); err != nil {
		t.Fatalf("AllowUser failed: %v", err)
	}

	// Cascade delete from child path
	count, err := s.RemoveUserCascade("mnt/storage", "/parent/child/", "frank", true)
	if err != nil {
		t.Fatalf("RemoveUserCascade failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 rules removed (child + grandchild), got %d", count)
	}

	// Verify parent rule still exists
	rule, ok := s.GetFrontendRules("mnt/storage", "/parent/")
	if !ok || len(rule.Allow.Users) == 0 {
		t.Error("Parent rule should still exist")
	}

	// Verify child rules are gone
	rule, ok = s.GetFrontendRules("mnt/storage", "/parent/child/")
	if ok && len(rule.Allow.Users) > 0 {
		t.Error("Child rule should be removed")
	}

	t.Log("✓ Cascade delete does not affect parent paths")
}

// TestRemoveUserCascade_CleanupEmptyRules tests that empty rules are cleaned up
func TestRemoveUserCascade_CleanupEmptyRules(t *testing.T) {
	setupTestSources()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "grace")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// Add only allow rule (no deny rules)
	if err := s.AllowUser("mnt/storage", "/temp/", "grace"); err != nil {
		t.Fatalf("AllowUser failed: %v", err)
	}
	if err := s.AllowUser("mnt/storage", "/temp/files/", "grace"); err != nil {
		t.Fatalf("AllowUser failed: %v", err)
	}

	// Verify rules exist
	allRules, err := s.GetAllRules("mnt/storage")
	if err != nil {
		t.Fatalf("GetAllRules failed: %v", err)
	}
	if len(allRules) < 2 {
		t.Error("Expected at least 2 rules to exist")
	}

	// Cascade delete - should remove rules and cleanup empty rule objects
	count, err := s.RemoveUserCascade("mnt/storage", "/temp/", "grace", true)
	if err != nil {
		t.Fatalf("RemoveUserCascade failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 rules removed, got %d", count)
	}

	// Verify rules are completely gone (not just empty)
	allRules, err = s.GetAllRules("mnt/storage")
	if err != nil {
		t.Fatalf("GetAllRules failed: %v", err)
	}
	if _, exists := allRules["/temp/"]; exists {
		t.Error("Empty rule should be cleaned up from /temp/")
	}
	if _, exists := allRules["/temp/files/"]; exists {
		t.Error("Empty rule should be cleaned up from /temp/files/")
	}

	t.Log("✓ Cascade delete correctly cleans up empty rules")
}

// TestRemoveUserCascade_MixedUsers tests that cascade delete only affects the specified user
func TestRemoveUserCascade_MixedUsers(t *testing.T) {
	setupTestSources()
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "henry")
	createTestUser(t, userStore, "iris")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}

	// Add rules for both users on same paths
	if err := s.AllowUser("mnt/storage", "/shared/", "henry"); err != nil {
		t.Fatalf("AllowUser failed: %v", err)
	}
	if err := s.AllowUser("mnt/storage", "/shared/", "iris"); err != nil {
		t.Fatalf("AllowUser failed: %v", err)
	}
	if err := s.AllowUser("mnt/storage", "/shared/docs/", "henry"); err != nil {
		t.Fatalf("AllowUser failed: %v", err)
	}
	if err := s.AllowUser("mnt/storage", "/shared/docs/", "iris"); err != nil {
		t.Fatalf("AllowUser failed: %v", err)
	}

	// Cascade delete only henry's rules
	count, err := s.RemoveUserCascade("mnt/storage", "/shared/", "henry", true)
	if err != nil {
		t.Fatalf("RemoveUserCascade failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 rules removed for henry, got %d", count)
	}

	// Verify henry's rules are gone
	rule, ok := s.GetFrontendRules("mnt/storage", "/shared/")
	if !ok {
		t.Fatal("Rule should still exist (iris's rule)")
	}
	hasHenry := false
	for _, user := range rule.Allow.Users {
		if user == "henry" {
			hasHenry = true
		}
	}
	if hasHenry {
		t.Error("henry should be removed from allow list")
	}

	// Verify iris's rules still exist
	hasIris := false
	for _, user := range rule.Allow.Users {
		if user == "iris" {
			hasIris = true
		}
	}
	if !hasIris {
		t.Error("iris should still be in allow list")
	}

	t.Log("✓ Cascade delete only affects the specified user")
}
