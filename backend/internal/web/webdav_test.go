package web

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/app"
	_ "github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb" // Import to register SQL driver
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	commonerrors "github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// setupWebDAVTestEnv sets up the test environment with multiple sources and users
func setupWebDAVTestEnv(t *testing.T) (string, string) {
	// Create temp directories for test sources
	tempDir := t.TempDir()
	source1Path := filepath.Join(tempDir, "source1")
	source2Path := filepath.Join(tempDir, "source2")
	dockerPath := filepath.Join(source1Path, "_docker")

	// Create directory structure
	dirs := []string{
		filepath.Join(source1Path, "public"),
		filepath.Join(source1Path, "private"),
		filepath.Join(source1Path, "viewable-only"),
		filepath.Join(source1Path, "not-viewable"),
		dockerPath,
		filepath.Join(dockerPath, "data"),
		filepath.Join(source2Path, "shared"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Create test files
	testFiles := map[string]string{
		filepath.Join(source1Path, "public", "readme.txt"):       "public content",
		filepath.Join(source1Path, "private", "secret.txt"):      "private content",
		filepath.Join(source1Path, "viewable-only", "data.txt"):  "viewable content",
		filepath.Join(source1Path, "not-viewable", "hidden.txt"): "hidden content",
		filepath.Join(dockerPath, "data", "config.yml"):          "docker config",
		filepath.Join(source2Path, "shared", "document.txt"):     "shared content",
	}

	for path, content := range testFiles {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Setup database with state
	dbPath := filepath.Join(tempDir, "test.sqlite")
	_, err := state.Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	app.MustWireServices(state.Default())
	t.Cleanup(func() {
		state.Close()
	})

	// Set cache directory for index database
	settings.Config.Server.CacheDir = tempDir
	settings.Config.Server.BaseURL = "/"
	settings.Config.Server.SourceMap = map[string]*settings.Source{
		source1Path: {
			Path: source1Path,
			Name: "source1",
		},
		source2Path: {
			Path: source2Path,
			Name: "source2",
		},
	}
	settings.Config.Server.NameToSource = map[string]*settings.Source{
		"source1": {
			Path: source1Path,
			Name: "source1",
		},
		"source2": {
			Path: source2Path,
			Name: "source2",
		},
	}

	// Initialize user resolvers
	settings.InitializeUserResolvers()

	// Create minimal mock indices for webdav handler
	indexing.SetTestIndex("source1", source1Path)
	indexing.SetTestIndex("source2", source2Path)
	t.Cleanup(func() {
		indexing.ClearTestIndices()
	})

	// Mock CheckPermissions to bypass index requirement
	mockCheckPermissions(t, source1Path, source2Path)

	// Initialize mock indexing - this mocks FileInfoFaster
	mockWebDAVIndexing(t, source1Path, source2Path)

	return source1Path, source2Path
}

// mockCheckPermissions mocks the CheckPermissions function to bypass index lookups
func mockCheckPermissions(t *testing.T, source1Path, source2Path string) {
	t.Helper()

	// Store original function
	originalCheckPermissions := files.CheckPermissionsFunc
	t.Cleanup(func() { files.CheckPermissionsFunc = originalCheckPermissions })

	// Mock the function
	files.CheckPermissionsFunc = func(opts utils.FileOptions, user *users.User) (string, string, error) {
		// Get the source path
		var sourcePath string
		if opts.Source == "source1" {
			sourcePath = source1Path
		} else if opts.Source == "source2" {
			sourcePath = source2Path
		} else {
			return "", "", fmt.Errorf("unknown source: %s", opts.Source)
		}

		// Get user scope for this source - match by source PATH, not name
		userScope := "/"
		hasScope := false
		for _, scope := range user.BackendScopes {
			if scope.Path == sourcePath {
				userScope = scope.Scope
				hasScope = true
				break
			}
		}

		// If user has no scope for this source, deny access
		if !hasScope && len(user.BackendScopes) > 0 {
			return "", "", fmt.Errorf("user has no access to source: %s", opts.Source)
		}

		// Sanitize path
		safePath, err := utils.SanitizePath(opts.Path)
		if err != nil {
			return "", "", commonerrors.ErrAccessDenied
		}

		// Combine scope + sanitized path
		indexPath := utils.JoinPathAsUnix(userScope, safePath)

		// Check access control
		if !state.AccessPermitted(sourcePath, utils.IndexPathFromNormalized(indexPath, true), user.Username) {
			return "", "", commonerrors.ErrAccessDenied
		}

		return indexPath, userScope, nil
	}
}

// mockWebDAVIndexing mocks the indexing system for WebDAV tests
func mockWebDAVIndexing(t *testing.T, source1Path, source2Path string) {
	// Mock FileInfoFaster to simulate indexing behavior
	originalFileInfoFaster := files.FileInfoFasterFunc
	t.Cleanup(func() { files.FileInfoFasterFunc = originalFileInfoFaster })

	files.FileInfoFasterFunc = func(opts utils.FileOptions, user *users.User) (*iteminfo.ExtendedFileInfo, error) {
		// Resolve source name to source path first
		sourcePath := ""
		if opts.Source == "source1" {
			sourcePath = source1Path
		} else if opts.Source == "source2" {
			sourcePath = source2Path
		} else {
			return nil, fmt.Errorf("unknown source: %s", opts.Source)
		}

		// Simulate access control
		if user != nil {
			hasAccess := false
			for _, scope := range user.BackendScopes {
				if scope.Path == sourcePath {
					hasAccess = true
					userScope := scope.Scope
					fullPath := utils.JoinPathAsUnix(userScope, opts.Path)
					if !state.AccessPermitted(sourcePath, utils.IndexPathFromNormalized(fullPath, true), user.Username) {
						return nil, commonerrors.ErrAccessDenied
					}
					break
				}
			}
			if !hasAccess && len(user.BackendScopes) > 0 {
				return nil, commonerrors.ErrAccessDenied
			}
		}

		// Simulate indexing rules
		path := opts.Path

		// Paths containing "not-viewable" are not indexed and not viewable
		if strings.Contains(path, "not-viewable") {
			return nil, commonerrors.ErrNotViewable
		}

		// Paths containing "viewable-only" are not indexed but viewable
		if strings.Contains(path, "viewable-only") {
			return nil, commonerrors.ErrNotIndexed
		}

		// Simulate directory expansion
		var files []iteminfo.ExtendedItemInfo
		var folders []iteminfo.ItemInfo

		if opts.Expand && (path == "/" || strings.HasSuffix(path, "/")) {
			// Return mock directory contents based on path
			if path == "/" || path == "" {
				folders = []iteminfo.ItemInfo{
					{Name: "public", Type: "directory"},
					{Name: "private", Type: "directory"},
					{Name: "viewable-only", Type: "directory"},
				}
			} else if strings.Contains(path, "public") {
				files = []iteminfo.ExtendedItemInfo{
					{ItemInfo: iteminfo.ItemInfo{Name: "readme.txt", Size: 14, Type: "text/plain"}},
				}
			} else if strings.Contains(path, "private") {
				files = []iteminfo.ExtendedItemInfo{
					{ItemInfo: iteminfo.ItemInfo{Name: "secret.txt", Size: 15, Type: "text/plain"}},
				}
			}
		}

		return &iteminfo.ExtendedFileInfo{
			FileInfo: iteminfo.FileInfo{
				Path:    path,
				Files:   files,
				Folders: folders,
				ItemInfo: iteminfo.ItemInfo{
					Name: filepath.Base(path),
					Type: "directory",
				},
			},
		}, nil
	}
}

// Test PROPFIND with different user scopes
func TestWebDAV_PROPFIND_UserScopes(t *testing.T) {
	source1Path, source2Path := setupWebDAVTestEnv(t)

	// Create users with different scopes
	// IMPORTANT: SourceScope.Name should be the source PATH, not the source name
	adminUser := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Username: "admin",
			Permissions: users.Permissions{
				Admin:    true,
				Download: true,
				Create:   true,
				Delete:   true,
				Modify:   true,
			},
		},
		BackendScopes: []users.BackendScope{
			{Path: source1Path, Scope: "/"},
			{Path: source2Path, Scope: "/"},
		},
	}

	scopedUser := &users.User{
		ID: 2,
		FrontendUser: users.FrontendUser{
			Username: "scoped",
			Permissions: users.Permissions{
				Download: true,
			},
		},
		BackendScopes: []users.BackendScope{
			{Path: source1Path, Scope: "/_docker"},
		},
	}

	restrictedUser := &users.User{
		ID: 3,
		FrontendUser: users.FrontendUser{
			Username: "restricted",
			Permissions: users.Permissions{
				Download: true,
			},
		},
		BackendScopes: []users.BackendScope{
			{Path: source1Path, Scope: "/public"},
		},
	}

	// Users are passed in requestContext only; do not call state.CreateUser here.
	// CreateUser runs MakeUserDirs which rewrites scope "/" to "/<username>", breaking
	// WebDAV chroot (expects index scope "/" to map to the source root).

	testCases := []struct {
		name              string
		user              *users.User
		source            string
		path              string
		expectedStatus    int
		shouldHaveContent bool
	}{
		{
			name:              "Admin can access root of source1",
			user:              adminUser,
			source:            "source1",
			path:              "/",
			expectedStatus:    207, // PROPFIND returns 207 Multi-Status
			shouldHaveContent: true,
		},
		{
			name:              "Scoped user can access their scope root",
			user:              scopedUser,
			source:            "source1",
			path:              "/",
			expectedStatus:    207,   // PROPFIND returns 207 Multi-Status
			shouldHaveContent: false, // Empty because _docker is not viewable
		},
		{
			name:              "Restricted user can only see public folder",
			user:              restrictedUser,
			source:            "source1",
			path:              "/",
			expectedStatus:    207, // PROPFIND returns 207 Multi-Status
			shouldHaveContent: true,
		},
		{
			name:              "Admin can access source2",
			user:              adminUser,
			source:            "source2",
			path:              "/",
			expectedStatus:    207, // PROPFIND returns 207 Multi-Status
			shouldHaveContent: true,
		},
		{
			name:           "Scoped user cannot access source2",
			user:           scopedUser,
			source:         "source2",
			path:           "/",
			expectedStatus: http.StatusForbidden,
		},
	}

	// Initialize indexing for sources
	initTestIndex(t, "source1", source1Path)
	initTestIndex(t, "source2", source2Path)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("PROPFIND", "/dav/"+tc.source+tc.path, nil)
			req.SetPathValue("source", tc.source)
			req.SetPathValue("path", tc.path)

			w := httptest.NewRecorder()
			ctx := &requestContext{User: tc.user}

			status, err := webDAVHandler(w, req, ctx)

			// If handler returned an error status, use that instead of response code
			if status != 0 && status != http.StatusOK {
				if w.Code == 0 || w.Code == http.StatusOK {
					w.Code = status
				}
			}

			// Check the actual HTTP response status code from ResponseWriter
			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d (err: %v)", tc.expectedStatus, w.Code, err)
			}

			if tc.shouldHaveContent && w.Code == 207 {
				body := w.Body.String()
				if body == "" {
					t.Error("expected response body to have content")
				}
			}
		})
	}
}

// Test WebDAV write operations with access control
func TestWebDAV_WriteOperations(t *testing.T) {
	source1Path, _ := setupWebDAVTestEnv(t)

	// Create users with different permissions
	// IMPORTANT: SourceScope.Name should be the source PATH, not the source name
	fullAccessUser := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Username: "fullaccess",
			Permissions: users.Permissions{
				Download: true,
				Create:   true,
				Delete:   true,
				Modify:   true,
			},
		},
		BackendScopes: []users.BackendScope{
			{Path: source1Path, Scope: "/"},
		},
	}

	readOnlyUser := &users.User{
		ID: 2,
		FrontendUser: users.FrontendUser{
			Username: "readonly",
			Permissions: users.Permissions{
				Download: true,
			},
		},
		BackendScopes: []users.BackendScope{
			{Path: source1Path, Scope: "/"},
		},
	}

	scopedUser := &users.User{
		ID: 3,
		FrontendUser: users.FrontendUser{
			Username: "scoped",
			Permissions: users.Permissions{
				Download: true,
				Create:   true,
				Modify:   true,
				Delete:   true,
			},
		},
		BackendScopes: []users.BackendScope{
			{Path: source1Path, Scope: "/public"},
		},
	}

	initTestIndex(t, "source1", source1Path)

	testCases := []struct {
		name           string
		user           *users.User
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "Full access user can PUT file",
			user:           fullAccessUser,
			method:         http.MethodPut,
			path:           "/public/newfile.txt",
			expectedStatus: http.StatusCreated, // 201 for new file creation
		},
		{
			name:           "Read-only user cannot PUT file",
			user:           readOnlyUser,
			method:         http.MethodPut,
			path:           "/public/newfile.txt",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Full access user can MKCOL",
			user:           fullAccessUser,
			method:         "MKCOL",
			path:           "/newdir",
			expectedStatus: http.StatusCreated, // 201 for new directory
		},
		{
			name:           "Read-only user cannot MKCOL",
			user:           readOnlyUser,
			method:         "MKCOL",
			path:           "/newdir",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Full access user can DELETE",
			user:           fullAccessUser,
			method:         http.MethodDelete,
			path:           "/public/readme.txt",
			expectedStatus: http.StatusNoContent, // 204 for successful deletion
		},
		{
			name:           "Read-only user cannot DELETE",
			user:           readOnlyUser,
			method:         http.MethodDelete,
			path:           "/public/readme.txt",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Scoped user can write in their scope",
			user:           scopedUser,
			method:         http.MethodPut,
			path:           "/scopedfile.txt",  // Path relative to scope (/public)
			expectedStatus: http.StatusCreated, // 201 for new file
		},
		{
			name:           "User cannot write to not-viewable directory",
			user:           fullAccessUser,
			method:         http.MethodPut,
			path:           "/not-viewable/file.txt",
			expectedStatus: 404, // Not viewable = acts like it doesn't exist
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.method == http.MethodPut {
				body = strings.NewReader("test content")
			}

			req := httptest.NewRequest(tc.method, "/dav/source1"+tc.path, body)
			req.SetPathValue("source", "source1")
			req.SetPathValue("path", tc.path)

			w := httptest.NewRecorder()
			ctx := &requestContext{User: tc.user}

			status, err := webDAVHandler(w, req, ctx)

			// If handler returned an error status, use that instead of response code
			if status != 0 && status != http.StatusOK {
				if w.Code == 0 || w.Code == http.StatusOK {
					w.Code = status
				}
			}

			// Check the actual HTTP response status code from ResponseWriter
			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d (err: %v)", tc.expectedStatus, w.Code, err)
			}
		})
	}
}

// Test access control rules
func TestWebDAV_AccessControl(t *testing.T) {
	source1Path, _ := setupWebDAVTestEnv(t)

	// Create users
	// IMPORTANT: SourceScope.Name should be the source PATH, not the source name
	user1 := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Username: "user1",
			Permissions: users.Permissions{
				Download: true,
				Create:   true,
			},
		},
		BackendScopes: []users.BackendScope{
			{Path: source1Path, Scope: "/"},
		},
	}

	user2 := &users.User{
		ID: 2,
		FrontendUser: users.FrontendUser{
			Username: "user2",
			Permissions: users.Permissions{
				Download: true,
			},
		},
		BackendScopes: []users.BackendScope{
			{Path: source1Path, Scope: "/"},
		},
	}

	// Initialize index
	initTestIndex(t, "source1", source1Path)

	err := state.DenyUser(source1Path, utils.IndexPathFromNormalized("/private/", true), "user2")
	if err != nil {
		t.Fatalf("failed to set up access rule: %v", err)
	}

	testCases := []struct {
		name           string
		user           *users.User
		path           string
		expectedStatus int
	}{
		{
			name:           "User1 can access private folder",
			user:           user1,
			path:           "/private",
			expectedStatus: 207, // PROPFIND returns 207 Multi-Status
		},
		{
			name:           "User2 cannot access private folder",
			user:           user2,
			path:           "/private",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Both users can access public folder",
			user:           user1,
			path:           "/public",
			expectedStatus: 207, // PROPFIND returns 207 Multi-Status
		},
		{
			name:           "User2 can also access public folder",
			user:           user2,
			path:           "/public",
			expectedStatus: 207, // PROPFIND returns 207 Multi-Status
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("PROPFIND", "/dav/source1"+tc.path, nil)
			req.SetPathValue("source", "source1")
			req.SetPathValue("path", tc.path)

			w := httptest.NewRecorder()
			ctx := &requestContext{User: tc.user}

			status, err := webDAVHandler(w, req, ctx)

			// If handler returned an error status, use that instead of response code
			if status != 0 && status != http.StatusOK {
				if w.Code == 0 || w.Code == http.StatusOK {
					w.Code = status
				}
			}

			// Check the actual HTTP response status code from ResponseWriter
			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d (err: %v)", tc.expectedStatus, w.Code, err)
			}
		})
	}
}

// Test indexing states (indexed, viewable, not-viewable)
func TestWebDAV_IndexingStates(t *testing.T) {
	source1Path, _ := setupWebDAVTestEnv(t)

	// IMPORTANT: SourceScope.Name should be the source PATH, not the source name
	user := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Username: "testuser",
			Permissions: users.Permissions{
				Download: true,
				Create:   true,
				Modify:   true,
				Delete:   true,
			},
		},
		BackendScopes: []users.BackendScope{
			{Path: source1Path, Scope: "/"},
		},
	}

	initTestIndex(t, "source1", source1Path)

	testCases := []struct {
		name           string
		path           string
		method         string
		expectedStatus int
		description    string
	}{
		{
			name:           "Can read indexed directory",
			path:           "/public",
			method:         "PROPFIND",
			expectedStatus: 207, // PROPFIND returns 207 Multi-Status
			description:    "Indexed directories should be accessible",
		},
		{
			name:           "Can read viewable-only directory",
			path:           "/viewable-only",
			method:         "PROPFIND",
			expectedStatus: 207, // PROPFIND returns 207 Multi-Status
			description:    "Not indexed but viewable directories should be readable",
		},
		{
			name:           "Cannot read not-viewable directory",
			path:           "/not-viewable",
			method:         "PROPFIND",
			expectedStatus: 405, // WebDAV returns 405 Method Not Allowed when resource denied
			description:    "Not viewable directories should be denied",
		},
		{
			name:           "Cannot write to viewable-only directory",
			path:           "/viewable-only/newfile.txt",
			method:         http.MethodPut,
			expectedStatus: 404, // Returns 404 when trying to write to non-existent file in denied location
			description:    "Viewable-only directories should deny write",
		},
		{
			name:           "Cannot write to not-viewable directory",
			path:           "/not-viewable/newfile.txt",
			method:         http.MethodPut,
			expectedStatus: 404, // Not viewable = acts like it doesn't exist
			description:    "Not viewable directories should deny all access",
		},
		{
			name:           "Can write to indexed directory",
			path:           "/public/newfile.txt",
			method:         http.MethodPut,
			expectedStatus: http.StatusCreated, // 201 for new file creation
			description:    "Indexed directories should allow write with permissions",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.method == http.MethodPut {
				body = strings.NewReader("test content")
			}

			req := httptest.NewRequest(tc.method, "/dav/source1"+tc.path, body)
			req.SetPathValue("source", "source1")
			req.SetPathValue("path", tc.path)

			w := httptest.NewRecorder()
			ctx := &requestContext{User: user}

			status, err := webDAVHandler(w, req, ctx)

			// If handler returned an error status, use that instead of response code
			if status != 0 && status != http.StatusOK {
				if w.Code == 0 || w.Code == http.StatusOK {
					w.Code = status
				}
			}

			// Check the actual HTTP response status code from ResponseWriter
			if w.Code != tc.expectedStatus {
				t.Errorf("%s: expected status %d, got %d (err: %v)", tc.description, tc.expectedStatus, w.Code, err)
			}
		})
	}
}

// Test that PUT with X-OC-Mtime header applies the mod time to the file on initial creation and on overwrite
func TestWebDAV_PutSetsMtimeFromOCHeader(t *testing.T) {
	source1Path, _ := setupWebDAVTestEnv(t)

	user := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Username:    "mtimeuser",
			Permissions: users.Permissions{Download: true, Create: true, Modify: true, Delete: true},
		},
		BackendScopes: []users.BackendScope{{Path: source1Path, Scope: "/"}},
	}
	filePath := filepath.Join(source1Path, "public", "file.txt")

	put := func(body string, mtime time.Time, wantStatus int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPut, "/dav/source1/public/file.txt", strings.NewReader(body))
		req.SetPathValue("source", "source1")
		req.SetPathValue("path", "/public/file.txt")
		req.Header.Set("X-OC-Mtime", strconv.FormatInt(mtime.Unix(), 10))

		w := httptest.NewRecorder()
		if _, err := webDAVHandler(w, req, &requestContext{User: user}); err != nil {
			t.Fatalf("webDAVHandler: %v", err)
		}
		if w.Code != wantStatus {
			t.Fatalf("expected %d, got %d", wantStatus, w.Code)
		}

		got, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("stat file: %v", err)
		}
		if !got.ModTime().Equal(mtime) {
			t.Errorf("mtime = %v, want %v", got.ModTime(), mtime)
		}
	}
	put("hi", time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC), http.StatusCreated)
	put("updated!", time.Date(2021, 6, 7, 8, 9, 10, 0, time.UTC), http.StatusCreated)
}

func TestWebDAV_PutIgnoresInvalidOCMtimeHeader(t *testing.T) {
	source1Path, _ := setupWebDAVTestEnv(t)
	user := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Username:    "mtimeuser2",
			Permissions: users.Permissions{Download: true, Create: true, Modify: true, Delete: true},
		},
		BackendScopes: []users.BackendScope{{Path: source1Path, Scope: "/"}},
	}
	filePath := filepath.Join(source1Path, "public", "file2.txt")

	before := time.Now()
	req := httptest.NewRequest(http.MethodPut, "/dav/source1/public/file2.txt", strings.NewReader("hi"))
	req.SetPathValue("source", "source1")
	req.SetPathValue("path", "/public/file2.txt")
	req.Header.Set("X-OC-Mtime", "not-a-number")

	w := httptest.NewRecorder()
	if _, err := webDAVHandler(w, req, &requestContext{User: user}); err != nil {
		t.Fatalf("webDAVHandler: %v", err)
	}
	after := time.Now()
	if w.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, w.Code)
	}

	got, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if got.ModTime().Before(before.Add(-time.Second)) || got.ModTime().After(after.Add(time.Second)) {
		t.Errorf("mtime = %v, want between %v and %v", got.ModTime(), before, after)
	}
}

func TestWebDAV_CopyPreservesModTime(t *testing.T) {
	source1Path, _ := setupWebDAVTestEnv(t)
	user := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Username:    "testcopy",
			Permissions: users.Permissions{Download: true, Create: true, Modify: true, Delete: true},
		},
		BackendScopes: []users.BackendScope{{Path: source1Path, Scope: "/"}},
	}
	fileTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	readme := filepath.Join(source1Path, "public", "readme.txt")
	if err := os.Chtimes(readme, fileTime, fileTime); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("COPY", "/dav/source1/public/", nil)
	req.SetPathValue("source", "source1")
	req.SetPathValue("path", "/public/")
	req.Header.Set("Destination", "/dav/source1/copy")
	req.Header.Set("Depth", "infinity")

	w := httptest.NewRecorder()
	if _, err := webDAVHandler(w, req, &requestContext{User: user}); err != nil {
		t.Fatalf("webDAVHandler: %v", err)
	}
	if w.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, w.Code)
	}
	got, err := os.Stat(filepath.Join(source1Path, "copy", "readme.txt"))
	if err != nil {
		t.Fatalf("stat copy: %v", err)
	}
	if !got.ModTime().Equal(fileTime) {
		t.Errorf("mtime = %v, want %v", got.ModTime(), fileTime)
	}
}


// Helper function to initialize a test index - simplified for WebDAV tests
func initTestIndex(t *testing.T, name, path string) {
	// For WebDAV tests, indices are already initialized in setupWebDAVTestEnv
	t.Helper()
}
