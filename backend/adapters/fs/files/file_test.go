package files

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	liberrors "github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	dbsql "github.com/gtsteffaniak/filebrowser/backend/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

func Test_GetRealPath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	trimPrefix := filepath.Dir(filepath.Dir(cwd))
	tests := []struct {
		name  string
		paths []string
		want  struct {
			path  string
			isDir bool
		}
	}{
		{
			name: "current directory",
			paths: []string{
				"./",
			},
			want: struct {
				path  string
				isDir bool
			}{
				path:  "",
				isDir: true,
			},
		},
		{
			name: "current directory",
			paths: []string{
				"./files/file.go",
			},
			want: struct {
				path  string
				isDir bool
			}{
				path:  "/files/file.go",
				isDir: false,
			},
		},
		{
			name: "other test case",
			paths: []string{
				"/mnt/doesnt/exist",
			},
			want: struct {
				path  string
				isDir bool
			}{
				path:  "/mnt/doesnt/exist",
				isDir: false,
			},
		},
	}
	idx := indexing.Index{
		Source: settings.Source{
			Path: trimPrefix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			realPath, isDir, _ := idx.GetRealPath(tt.paths...)
			adjustedRealPath := strings.TrimPrefix(realPath, trimPrefix)
			if tt.want.path != adjustedRealPath || tt.want.isDir != isDir {
				t.Errorf("expected %v:%v but got: %v:%v", tt.want.path, tt.want.isDir, adjustedRealPath, isDir)
			}
		})
	}
}

// Regression: PUT / save must not os.Chmod to PermFile for existing files, or +x (and other mode bits) are lost (#2309).
func TestWriteFilePreservesExecutableBit(t *testing.T) {
	cacheDir := t.TempDir()
	originalCache := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = cacheDir
	t.Cleanup(func() { settings.Config.Server.CacheDir = originalCache })

	fileutils.SetFsPermissions(0o644, 0o755)

	if indexing.GetIndexDB() == nil {
		db, _, err := dbsql.NewIndexDB("test_writefile_exec", "OFF", 1000, 32, false)
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
		indexing.SetIndexDBForTesting(db)
	}

	root := t.TempDir()
	scriptName := "backupscript.sh"
	scriptAbs := filepath.Join(root, scriptName)
	if err := os.WriteFile(scriptAbs, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	sourceName := "test_writefile_exec"
	indexing.Initialize(&settings.Source{
		Name: sourceName,
		Path: root,
	}, false, false)

	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		t.Fatal("Failed to get test index")
	}
	stopPoll := startScannerStatusPoll(t, idx, 1)
	defer stopPoll()
	waitForScannerReady(t, idx)

	indexPath := "/" + scriptName
	if err := WriteFile(sourceName, indexPath, strings.NewReader("#!/bin/sh\necho bye\n")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	st, err := os.Stat(scriptAbs)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode()&0o111 == 0 {
		t.Fatalf("executable bits cleared after save: mode=%v", st.Mode())
	}

	indexing.StopAllScanners()
	time.Sleep(50 * time.Millisecond)
	indexing.ClearTestIndices()
}

func TestWriteFileRejectsExistingDirectory(t *testing.T) {
	cacheDir := t.TempDir()
	originalCache := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = cacheDir
	t.Cleanup(func() { settings.Config.Server.CacheDir = originalCache })

	fileutils.SetFsPermissions(0o644, 0o755)

	if indexing.GetIndexDB() == nil {
		db, _, err := dbsql.NewIndexDB("test_writefile_isdir", "OFF", 1000, 32, false)
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
		indexing.SetIndexDBForTesting(db)
	}

	root := t.TempDir()
	dirName := "folder"
	if err := os.Mkdir(filepath.Join(root, dirName), 0o755); err != nil {
		t.Fatal(err)
	}

	sourceName := "test_writefile_isdir"
	indexing.Initialize(&settings.Source{
		Name: sourceName,
		Path: root,
	}, false, false)

	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		t.Fatal("Failed to get test index")
	}
	stopPoll := startScannerStatusPoll(t, idx, 1)
	defer stopPoll()
	waitForScannerReady(t, idx)

	err := WriteFile(sourceName, "/"+dirName, strings.NewReader("file content"))
	if err == nil {
		t.Fatal("expected error when path is an existing directory")
	}
	if !errors.Is(err, liberrors.ErrIsDirectory) {
		t.Fatalf("expected ErrIsDirectory, got: %v", err)
	}

	indexing.StopAllScanners()
	time.Sleep(50 * time.Millisecond)
	indexing.ClearTestIndices()
}

func TestSortItems(t *testing.T) {
	tests := []struct {
		name     string
		input    iteminfo.FileInfo
		expected iteminfo.FileInfo
	}{
		{
			name: "Numeric and Lexicographical Sorting",
			input: iteminfo.FileInfo{
				Folders: []iteminfo.ItemInfo{
					{Name: "10.txt"},
					{Name: "2.txt"},
					{Name: "apple"},
					{Name: "Banana"},
				},
				Files: []iteminfo.ExtendedItemInfo{
					{ItemInfo: iteminfo.ItemInfo{Name: "File2.txt"}},
					{ItemInfo: iteminfo.ItemInfo{Name: "File10.txt"}},
					{ItemInfo: iteminfo.ItemInfo{Name: "File1"}},
					{ItemInfo: iteminfo.ItemInfo{Name: "banana"}},
				},
			},
			expected: iteminfo.FileInfo{
				Folders: []iteminfo.ItemInfo{
					{Name: "2.txt"},
					{Name: "10.txt"},
					{Name: "apple"},
					{Name: "Banana"},
				},
				Files: []iteminfo.ExtendedItemInfo{
					{ItemInfo: iteminfo.ItemInfo{Name: "banana"}},
					{ItemInfo: iteminfo.ItemInfo{Name: "File1"}},
					{ItemInfo: iteminfo.ItemInfo{Name: "File10.txt"}},
					{ItemInfo: iteminfo.ItemInfo{Name: "File2.txt"}},
				},
			},
		},
		{
			name: "Only Lexicographical Sorting",
			input: iteminfo.FileInfo{
				Folders: []iteminfo.ItemInfo{
					{Name: "dog.txt"},
					{Name: "Cat.txt"},
					{Name: "apple"},
				},
				Files: []iteminfo.ExtendedItemInfo{
					{ItemInfo: iteminfo.ItemInfo{Name: "Zebra"}},
					{ItemInfo: iteminfo.ItemInfo{Name: "apple"}},
					{ItemInfo: iteminfo.ItemInfo{Name: "cat"}},
				},
			},
			expected: iteminfo.FileInfo{
				Folders: []iteminfo.ItemInfo{
					{Name: "apple"},
					{Name: "Cat.txt"},
					{Name: "dog.txt"},
				},
				Files: []iteminfo.ExtendedItemInfo{
					{ItemInfo: iteminfo.ItemInfo{Name: "apple"}},
					{ItemInfo: iteminfo.ItemInfo{Name: "cat"}},
					{ItemInfo: iteminfo.ItemInfo{Name: "Zebra"}},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.input.SortItems()

			getFolderNames := func(items []iteminfo.ItemInfo) []string {
				names := []string{}
				for _, folder := range items {
					names = append(names, folder.Name)
				}
				return names
			}

			getFileNames := func(items []iteminfo.ExtendedItemInfo) []string {
				names := []string{}
				for _, file := range items {
					names = append(names, file.Name)
				}
				return names
			}

			actualFolderNames := getFolderNames(test.input.Folders)
			expectedFolderNames := getFolderNames(test.expected.Folders)

			if !reflect.DeepEqual(actualFolderNames, expectedFolderNames) {
				t.Errorf("Folders not sorted correctly.\nGot: %v\nExpected: %v", actualFolderNames, expectedFolderNames)
			}

			actualFileNames := getFileNames(test.input.Files)
			expectedFileNames := getFileNames(test.expected.Files)

			if !reflect.DeepEqual(actualFileNames, expectedFileNames) {
				t.Errorf("Files not sorted correctly.\nGot: %v\nExpected: %v", actualFileNames, expectedFileNames)
			}
		})
	}
}

// TestDeleteFilesCacheClearing was removed because cache clearing is not performed
// per design decision - RealPathCache is auxiliary and doesn't need clearing

func TestOverrideDirectoryToFile(t *testing.T) {
	// Use a temporary directory for cache to avoid creating directories in the source tree
	tmpDir := t.TempDir()
	originalCacheDir := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = tmpDir
	defer func() {
		settings.Config.Server.CacheDir = originalCacheDir
	}()

	// Ensure fileutils permissions are set (needed by NewTempDB)
	if fileutils.PermDir == 0 {
		fileutils.SetFsPermissions(0644, 0755)
	}

	// Initialize the database first (use test helper to avoid permission issues)
	if indexing.GetIndexDB() == nil {
		db, _, err := dbsql.NewIndexDB("test_file", "OFF", 1000, 32, false)
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
		indexing.SetIndexDBForTesting(db)
	}

	// Initialize the index with scanning disabled to prevent background scanner interference
	indexing.Initialize(&settings.Source{
		Name: "test",
		Path: "/mock/path",
	}, true, false) // true for mock mode, false for isNewDb

	// Get the index and set up mock data
	idx := indexing.GetIndex("test")
	if idx == nil { //nolint:staticcheck // t.Fatal terminates execution
		t.Fatal("Failed to get test index")
	}

	// Wait for initial scanner to complete (it will fail on /mock/path but needs to finish cleanup)
	// This ensures batchItems is nil before we start testing
	for i := 0; i < 50; i++ { // Wait up to 500ms
		time.Sleep(10 * time.Millisecond)
		status := idx.GetScannerStatus()
		if status["status"] == "ready" || status["status"] == "unavailable" {
			break
		}
	}

	// Create mock directory structure using UpdateMetadata
	rootInfo := &iteminfo.FileInfo{
		Path: "/",
		ItemInfo: iteminfo.ItemInfo{
			Name: "/",
			Type: "directory",
		},
		Folders: []iteminfo.ItemInfo{
			{Name: "Test Object", Type: "directory"},
		},
	}
	idx.UpdateMetadata(rootInfo, nil) // nil scanner for test

	// Delete the old directory item from the database before replacing it with a file
	idx.DeleteMetadata("/Test Object", true, false)

	// Simulate the directory-to-file override by updating the mock data
	// Remove the directory from the parent's Folders slice
	rootInfo.Folders = []iteminfo.ItemInfo{} // Clear folders

	// Add the file to the parent's Files slice
	rootInfo.Files = []iteminfo.ExtendedItemInfo{
		{
			ItemInfo: iteminfo.ItemInfo{
				Name: "Test Object",
				Size: 25, // Length of "This is test file content"
			},
		},
	}
	idx.UpdateMetadata(rootInfo, nil) // nil scanner for test

	// Verify the directory was replaced with a file in the mock data
	rootInfo, exists := idx.GetMetadataInfo("/", true, false)
	if !exists {
		t.Fatal("Root metadata not found")
	}

	// Check that the directory was removed from Folders
	foundDir := false
	for _, folder := range rootInfo.Folders {
		if folder.Name == "Test Object" {
			foundDir = true
			break
		}
	}
	if foundDir {
		t.Error("Directory 'Test Object' should have been removed from Folders")
	}

	// Check that the file was added to Files
	foundFile := false
	for _, file := range rootInfo.Files {
		if file.Name == "Test Object" {
			foundFile = true
			if file.Size != 25 {
				t.Errorf("Expected file size 25, got %d", file.Size)
			}
			break
		}
	}
	if !foundFile {
		t.Error("File 'Test Object' should have been added to Files")
	}
}

func TestOverrideFileToDirectory(t *testing.T) {
	// Use a temporary directory for cache to avoid creating directories in the source tree
	tmpDir := t.TempDir()
	originalCacheDir := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = tmpDir
	defer func() {
		settings.Config.Server.CacheDir = originalCacheDir
	}()

	// Ensure fileutils permissions are set (needed by NewTempDB)
	if fileutils.PermDir == 0 {
		fileutils.SetFsPermissions(0644, 0755)
	}

	// Initialize the database first (use test helper to avoid permission issues)
	if indexing.GetIndexDB() == nil {
		db, _, err := dbsql.NewIndexDB("test_file", "OFF", 1000, 32, false)
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
		indexing.SetIndexDBForTesting(db)
	}

	// Initialize the index in mock mode (no filesystem operations)
	indexing.Initialize(&settings.Source{
		Name: "test",
		Path: "/mock/path",
	}, true, false) // true for mock mode, false for isNewDb

	// Get the index and set up mock data
	idx := indexing.GetIndex("test")
	if idx == nil { //nolint:staticcheck // t.Fatal terminates execution
		t.Fatal("Failed to get test index")
	}

	// Wait for initial scanner to complete (it will fail on /mock/path but needs to finish cleanup)
	// This ensures batchItems is nil before we start testing
	for i := 0; i < 50; i++ { // Wait up to 500ms
		time.Sleep(10 * time.Millisecond)
		status := idx.GetScannerStatus()
		if status["status"] == "ready" || status["status"] == "unavailable" {
			break
		}
	}

	// Create mock directory structure with a file using UpdateMetadata
	rootInfo := &iteminfo.FileInfo{
		Path: "/",
		ItemInfo: iteminfo.ItemInfo{
			Name: "/",
			Type: "directory",
		},
		Files: []iteminfo.ExtendedItemInfo{
			{ItemInfo: iteminfo.ItemInfo{Name: "Test Object", Size: 12}}, // Length of "test content"
		},
	}
	idx.UpdateMetadata(rootInfo, nil) // nil scanner for test

	// Delete the old file item from the database before replacing it with a directory
	idx.DeleteMetadata("/Test Object", false, false)

	// Simulate the file-to-directory override by updating the mock data
	// Remove the file from the parent's Files slice
	rootInfo.Files = []iteminfo.ExtendedItemInfo{} // Clear files

	// Add the directory to the parent's Folders slice
	rootInfo.Folders = []iteminfo.ItemInfo{
		{
			Name: "Test Object",
			Type: "directory",
		},
	}
	idx.UpdateMetadata(rootInfo, nil) // nil scanner for test

	// Verify the file was replaced with a directory in the mock data
	rootInfo, exists := idx.GetMetadataInfo("/", true, false)
	if !exists {
		t.Fatal("Root metadata not found")
	}

	// Check that the file was removed from Files
	foundFile := false
	for _, file := range rootInfo.Files {
		if file.Name == "Test Object" {
			foundFile = true
			break
		}
	}
	if foundFile {
		t.Error("File 'Test Object' should have been removed from Files")
	}

	// Check that the directory was added to Folders
	foundDir := false
	for _, folder := range rootInfo.Folders {
		if folder.Name == "Test Object" {
			foundDir = true
			if folder.Type != "directory" {
				t.Errorf("Expected directory type, got %s", folder.Type)
			}
			break
		}
	}
	if !foundDir {
		t.Error("Directory 'Test Object' should have been added to Folders")
	}
}

// TestDeleteFilesRootProtection tests that DeleteFiles refuses to delete
// the source root directory itself, preventing catastrophic data loss.
// This is a regression test for the safety guard added to prevent accidental
// deletion of the entire source directory.
func TestDeleteFilesRootProtection(t *testing.T) {
	// Use a temporary directory for cache
	tmpDir := t.TempDir()
	originalCacheDir := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = tmpDir
	defer func() {
		settings.Config.Server.CacheDir = originalCacheDir
	}()

	// Ensure fileutils permissions are set
	if fileutils.PermDir == 0 {
		fileutils.SetFsPermissions(0644, 0755)
	}

	// Initialize the database
	if indexing.GetIndexDB() == nil {
		db, _, err := dbsql.NewIndexDB("test_root_protection", "OFF", 1000, 32, false)
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
		indexing.SetIndexDBForTesting(db)
	}

	// Create a real temporary directory to use as source root
	realTmpDir := t.TempDir()

	// Initialize the index with a real path
	indexing.Initialize(&settings.Source{
		Name: "test_root_protection",
		Path: realTmpDir,
	}, false, false) // false for mock mode (real path), false for isNewDb

	// Wait for scanner to be ready
	idx := indexing.GetIndex("test_root_protection")
	if idx == nil {
		t.Fatal("Failed to get test index")
	}

	// Wait for scanner to finish
	for i := 0; i < 50; i++ {
		time.Sleep(10 * time.Millisecond)
		status := idx.GetScannerStatus()
		if status["status"] == "ready" || status["status"] == "unavailable" {
			break
		}
	}

	// Test: Attempting to delete the root directory should return an error
	err := DeleteFiles("test_root_protection", realTmpDir, true)
	if err == nil {
		t.Error("DeleteFiles should return an error when trying to delete root directory")
	}

	// Verify the error message contains expected text
	expectedErrMsg := "refusing to delete source root directory"
	if err != nil && !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("Expected error message to contain '%s', got: %v", expectedErrMsg, err.Error())
	}

	// Test with trailing slash - should still be blocked
	err = DeleteFiles("test_root_protection", realTmpDir+"/", true)
	if err == nil {
		t.Error("DeleteFiles should return an error for root directory with trailing slash")
	}

	// Test with cleaned path variations
	err = DeleteFiles("test_root_protection", realTmpDir+"/.", true)
	if err == nil {
		t.Error("DeleteFiles should return an error for root directory with /. suffix")
	}
}

// TestDeleteFilesSubfolderWithRootName tests that deleting a subfolder
// with the same name as the root directory works correctly and doesn't
// accidentally delete the root.
// Regression test for path handling bug where /srv/srv could be mistaken for /srv.
func TestDeleteFilesSubfolderWithRootName(t *testing.T) {
	for iter := 1; iter <= raceStressIterations; iter++ {
		iter := iter
		t.Run(fmt.Sprintf("stress-iter-%d", iter), func(t *testing.T) {
			runDeleteFilesSubfolderWithRaceStress(t, iter)
		})
	}
}

const (
	raceStressIterations   = 3
	raceStatusPollInterval = 5 * time.Millisecond
	waitStatusMaxTries     = 100
	waitStatusSleep        = 50 * time.Millisecond
)

func runDeleteFilesSubfolderWithRaceStress(t *testing.T, iter int) {
	t.Helper()
	start := time.Now()
	t.Logf("iteration %d: start", iter)

	cacheDir := t.TempDir()
	originalCache := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = cacheDir
	t.Cleanup(func() { settings.Config.Server.CacheDir = originalCache })

	if fileutils.PermDir == 0 {
		fileutils.SetFsPermissions(0644, 0755)
	}

	if indexing.GetIndexDB() == nil {
		db, _, err := dbsql.NewIndexDB("test_subfolder_rootname", "OFF", 1000, 32, false)
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
		indexing.SetIndexDBForTesting(db)
	}

	realTmpDir := t.TempDir()
	subfolderName := filepath.Base(realTmpDir)
	subfolderPath := filepath.Join(realTmpDir, subfolderName)

	if err := os.Mkdir(subfolderPath, 0755); err != nil {
		t.Fatalf("Failed to create subfolder: %v", err)
	}

	testFile := filepath.Join(subfolderPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	sourceName := fmt.Sprintf("test_subfolder_rootname-%d", iter)
	indexing.Initialize(&settings.Source{
		Name: sourceName,
		Path: realTmpDir,
	}, false, false)

	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		t.Fatalf("Failed to get test index for %s", sourceName)
	}

	stopPolling := startScannerStatusPoll(t, idx, iter)
	defer stopPolling()

	waitForScannerReady(t, idx)

	err := DeleteFiles(sourceName, subfolderPath, true)
	stopPolling()

	t.Logf("iteration %d: DeleteFiles finished in %s (err=%v)", iter, time.Since(start), err)

	if err != nil {
		t.Fatalf("DeleteFiles should succeed for subfolder with root-like name, got error: %v", err)
	}

	if _, err := os.Stat(subfolderPath); !os.IsNotExist(err) {
		t.Fatal("Subfolder should have been deleted")
	}

	if _, err := os.Stat(realTmpDir); os.IsNotExist(err) {
		t.Fatal("Root directory should NOT have been deleted")
	}

	indexing.StopAllScanners()
	// Give scanners time to finish their cleanup (defer blocks in tryAcquireAndScan)
	time.Sleep(100 * time.Millisecond)
	indexing.ClearTestIndices()
	t.Logf("iteration %d: cleaned scanners and indices", iter)
}

func waitForScannerReady(t *testing.T, idx *indexing.Index) {
	t.Helper()
	// Give the scanners a moment to initialize, but don't wait for full "ready" status
	// The initial scan can take longer than the test timeout, but DeleteFiles will work
	// as long as the scanners are initialized and running
	time.Sleep(500 * time.Millisecond)
	
	// Verify scanners exist
	status := idx.GetScannerStatus()
	if totalScanners, ok := status["totalScanners"].(int); !ok || totalScanners == 0 {
		t.Fatal("No scanners were created")
	}
	t.Log("Scanners initialized, proceeding with delete operation")
}

func startScannerStatusPoll(t *testing.T, idx *indexing.Index, iter int) func() {
	t.Helper()
	stop := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once

	go func() {
		ticker := time.NewTicker(raceStatusPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				close(done)
				return
			case <-ticker.C:
				idx.GetScannerStatus()
			}
		}
	}()

	return func() {
		once.Do(func() {
			close(stop)
			<-done
			t.Logf("iteration %d: poll watcher stopped", iter)
		})
	}
}
