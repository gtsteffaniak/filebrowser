package indexing

import (
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	dbsql "github.com/gtsteffaniak/filebrowser/backend/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	// Ensure fileutils permissions are set (needed by NewTempDB)
	if fileutils.PermDir == 0 {
		fileutils.SetFsPermissions(0644, 0755)
	}

	// Create a temporary directory for test database files with proper permissions
	tmpDir, err := os.MkdirTemp("", "indexing_test_*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	// Ensure the temp directory has correct permissions
	if err := os.Chmod(tmpDir, 0755); err != nil {
		panic(err)
	}

	// Set the cache directory to the temporary directory
	originalCacheDir := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = tmpDir
	defer func() {
		settings.Config.Server.CacheDir = originalCacheDir
	}()

	// Run the tests
	code := m.Run()

	// Exit with the test result code
	os.Exit(code)
}

func BenchmarkSearchAllIndexes(b *testing.B) {
	Initialize(&settings.Source{Name: "test", Path: "/srv"}, true)
	idx := GetIndex("test")

	idx.CreateMockData(50, 3) // 50 dirs, 3 files per dir

	// Generate 100 random search terms
	searchTerms := utils.GenerateRandomSearchTerms(100)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Execute the SearchAllIndexes function
		for _, term := range searchTerms {
			idx.Search(term, "/", "test", false, DefaultSearchResults)
		}
	}
}

func TestParseSearch(t *testing.T) {
	tests := []struct {
		input string
		want  iteminfo.SearchOptions
	}{
		{
			input: "my test search",
			want: iteminfo.SearchOptions{
				Conditions: map[string]bool{"exact": false},
				Terms:      []string{"my test search"},
			},
		},
		{
			input: "case:exact my|test|search",
			want: iteminfo.SearchOptions{
				Conditions: map[string]bool{"exact": true},
				Terms:      []string{"my", "test", "search"},
			},
		},
		{
			input: "type:largerThan=100 type:smallerThan=1000 test",
			want: iteminfo.SearchOptions{
				Conditions:  map[string]bool{"exact": false, "larger": true, "smaller": true},
				Terms:       []string{"test"},
				LargerThan:  100,
				SmallerThan: 1000,
			},
		},
		{
			input: "type:audio thisfile",
			want: iteminfo.SearchOptions{
				Conditions: map[string]bool{"exact": false, "audio": true},
				Terms:      []string{"thisfile"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			value := iteminfo.ParseSearch(tt.input)
			if !reflect.DeepEqual(value, tt.want) {
				t.Fatalf("\n got:  %+v\n want: %+v", value, tt.want)
			}
		})
	}
}

func TestSearchWhileIndexing(t *testing.T) {
	// Initialize the database if not already done
	if indexDB == nil {
		var err error
		indexDB, err = dbsql.NewIndexDB("test_search_while", "OFF", 1000, 32, false)
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
	}

	Initialize(&settings.Source{Name: "test", Path: "/srv"}, true)
	idx := GetIndex("test")
	if idx == nil {
		t.Fatal("Failed to get test index")
	}

	var wg sync.WaitGroup
	searchTerms := utils.GenerateRandomSearchTerms(5)

	// Reduced load: 3 iterations × 10 dirs × 10 files = ~300 items total
	// This is enough to test concurrency without overwhelming the database
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx.CreateMockData(10, 10) // Reduced from 100×100 to 10×10
		}()

		for _, term := range searchTerms {
			wg.Add(1)
			go func(searchTerm string) {
				defer wg.Done()
				idx.Search(searchTerm, "/", "test", false, DefaultSearchResults)
			}(term)
		}

		// Small delay to reduce contention spikes
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for all goroutines to complete before test ends
	wg.Wait()
}

func TestSearchIndexes(t *testing.T) {
	// Initialize the database if not already done
	if indexDB == nil {
		var err error
		indexDB, err = dbsql.NewIndexDB("test_search", "OFF", 1000, 32, false)
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
	}

	index := Index{
		Source: settings.Source{
			Name: "test_search",
			Path: "/mock/path",
		},
		db:   indexDB,
		mock: true,
	}

	// Insert test data into database
	now := time.Now()
	directories := map[string]*iteminfo.FileInfo{
		"/":           {ItemInfo: iteminfo.ItemInfo{Name: "/", Type: "directory"}, Files: []iteminfo.ExtendedItemInfo{{ItemInfo: iteminfo.ItemInfo{Name: "audio-one.wav", Type: "audio"}}}},
		"/test/":      {ItemInfo: iteminfo.ItemInfo{Name: "test", Type: "directory"}, Files: []iteminfo.ExtendedItemInfo{{ItemInfo: iteminfo.ItemInfo{Name: "audio-one.wav", Type: "audio"}}}},
		"/test/path/": {ItemInfo: iteminfo.ItemInfo{Name: "path", Type: "directory"}, Files: []iteminfo.ExtendedItemInfo{{ItemInfo: iteminfo.ItemInfo{Name: "file.txt", Type: "text"}}}},
		"/new/":       {ItemInfo: iteminfo.ItemInfo{Name: "new", Type: "directory"}},
		"/new/test/": {ItemInfo: iteminfo.ItemInfo{Name: "test", Type: "directory"}, Files: []iteminfo.ExtendedItemInfo{
			{ItemInfo: iteminfo.ItemInfo{Name: "audio.wav", Type: "audio"}},
			{ItemInfo: iteminfo.ItemInfo{Name: "video.mp4", Type: "video"}},
			{ItemInfo: iteminfo.ItemInfo{Name: "video.MP4", Type: "video"}},
		}},
		"/first Dir/": {
			ItemInfo: iteminfo.ItemInfo{Name: "first Dir", Type: "directory"},
			Files: []iteminfo.ExtendedItemInfo{
				{ItemInfo: iteminfo.ItemInfo{Name: "space jam.zip", Size: 100, Type: "archive"}},
			},
		},
		"/new/test/path/": {ItemInfo: iteminfo.ItemInfo{Name: "path", Type: "directory"}, Files: []iteminfo.ExtendedItemInfo{{ItemInfo: iteminfo.ItemInfo{Name: "archive.zip", Type: "archive"}}}},
		"/firstDir/": {
			ItemInfo: iteminfo.ItemInfo{Name: "firstDir", Type: "directory"},
			Files: []iteminfo.ExtendedItemInfo{
				{ItemInfo: iteminfo.ItemInfo{Name: "archive.zip", Size: 100, Type: "archive"}},
			},
			Folders: []iteminfo.ItemInfo{
				{Name: "thisIsDir", Type: "directory", Size: 2 * 1024 * 1024},
			},
		},
		"/firstDir/thisIsDir/": {
			ItemInfo: iteminfo.ItemInfo{
				Name: "thisIsDir",
				Type: "directory",
				Size: 2 * 1024 * 1024,
			},
			Files: []iteminfo.ExtendedItemInfo{
				{ItemInfo: iteminfo.ItemInfo{Name: "hi.txt", Type: "text"}},
			},
		},
		"/new+folder/": {ItemInfo: iteminfo.ItemInfo{Name: "new+folder", Type: "directory"}},
		"/new+folder/Pictures/": {
			ItemInfo: iteminfo.ItemInfo{Name: "Pictures", Type: "directory"},
			Files: []iteminfo.ExtendedItemInfo{
				{ItemInfo: iteminfo.ItemInfo{Name: "consoletest.mp4", Size: 196091904, Type: "video/mp4"}},
				{ItemInfo: iteminfo.ItemInfo{Name: "playwright.gif", Size: 2416640, Type: "image/gif"}},
				{ItemInfo: iteminfo.ItemInfo{Name: "toggle.gif", Size: 65536, Type: "image/gif"}},
			},
		},
	}

	// Insert all directories and files into database
	for path, dirInfo := range directories {
		dirInfo.Path = path
		if dirInfo.ModTime.IsZero() {
			dirInfo.ModTime = now
		}
		_ = index.db.InsertItem("test_search", path, dirInfo)

		// Insert folders
		for _, folder := range dirInfo.Folders {
			folderPath := strings.TrimSuffix(path, "/") + "/" + folder.Name + "/"
			folderInfo := &iteminfo.FileInfo{
				Path:     folderPath,
				ItemInfo: folder,
			}
			if folderInfo.ModTime.IsZero() {
				folderInfo.ModTime = now
			}
			// Only insert if not already in the directories map (to avoid duplicates)
			if _, exists := directories[folderPath]; !exists {
				_ = index.db.InsertItem("test_search", folderPath, folderInfo)
			}
		}

		// Insert files
		for _, file := range dirInfo.Files {
			filePath := strings.TrimSuffix(path, "/") + "/" + file.Name
			fileInfo := &iteminfo.FileInfo{
				Path:     filePath,
				ItemInfo: file.ItemInfo,
			}
			if fileInfo.ModTime.IsZero() {
				fileInfo.ModTime = now
			}
			_ = index.db.InsertItem("test_search", filePath, fileInfo)
		}
	}

	tests := []struct {
		search         string
		scope          string
		expectedResult []*SearchResult
	}{
		{
			search: "audio",
			scope:  "/new/",
			expectedResult: []*SearchResult{
				{
					Path: "/new/test/audio.wav",
					Type: "audio",
					Size: 0,
				},
			},
		},
		{
			search: "test",
			scope:  "/",
			expectedResult: []*SearchResult{
				{
					Path: "/test/",
					Type: "directory",
					Size: 0,
				},
				{
					Path: "/new/test/",
					Type: "directory",
					Size: 0,
				},
				{
					Path: "/new+folder/Pictures/consoletest.mp4",
					Type: "video/mp4",
					Size: 196091904,
				},
			},
		},
		{
			search: "archive",
			scope:  "/",
			expectedResult: []*SearchResult{
				{
					Path: "/firstDir/archive.zip",
					Type: "archive",
					Size: 100,
				},
				{
					Path: "/new/test/path/archive.zip",
					Type: "archive",
					Size: 0,
				},
			},
		},
		{
			search: "arch",
			scope:  "/firstDir/",
			expectedResult: []*SearchResult{
				{
					Path: "/firstDir/archive.zip",
					Type: "archive",
					Size: 100,
				},
			},
		},
		{
			search: "space jam",
			scope:  "/first Dir/",
			expectedResult: []*SearchResult{
				{
					Path: "/first Dir/space jam.zip",
					Type: "archive",
					Size: 100,
				},
			},
		},
		{
			search: "isdir",
			scope:  "/",
			expectedResult: []*SearchResult{
				{
					Path: "/firstDir/thisIsDir/",
					Type: "directory",
					Size: 2097152,
				},
			},
		},
		{
			search: "IsDir type:largerThan=1",
			scope:  "/",
			expectedResult: []*SearchResult{
				{
					Path: "/firstDir/thisIsDir/",
					Type: "directory",
					Size: 2097152,
				},
			},
		},
		{
			search: "video",
			scope:  "/",
			expectedResult: []*SearchResult{
				{
					Path: "/new/test/video.MP4",
					Type: "video",
					Size: 0,
				},
				{
					Path: "/new/test/video.mp4",
					Type: "video",
					Size: 0,
				},
			},
		},
		{
			search: "audio",
			scope:  "/",
			expectedResult: []*SearchResult{
				{
					Path: "/audio-one.wav",
					Type: "audio",
					Size: 0,
				},
				{
					Path: "/test/audio-one.wav",
					Type: "audio",
					Size: 0,
				},
				{
					Path: "/new/test/audio.wav",
					Type: "audio",
					Size: 0,
				},
			},
		},
		{
			search: "cons",
			scope:  "/new+folder/Pictures/",
			expectedResult: []*SearchResult{
				{
					Path: "/new+folder/Pictures/consoletest.mp4",
					Type: "video/mp4",
					Size: 196091904,
				},
			},
		},
		{
			search: "toggle",
			scope:  "/new+folder/Pictures/",
			expectedResult: []*SearchResult{
				{
					Path: "/new+folder/Pictures/toggle.gif",
					Type: "image/gif",
					Size: 65536,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.search, func(t *testing.T) {
			result := index.Search(tt.search, tt.scope, "", false, DefaultSearchResults)
			// Convert results to comparable format (without Modified field)
			expected := make([]SearchResult, len(tt.expectedResult))
			for i, r := range tt.expectedResult {
				expected[i] = SearchResult{Path: r.Path, Type: r.Type, Size: r.Size}
			}
			actual := make([]SearchResult, len(result))
			for i, r := range result {
				actual[i] = SearchResult{Path: r.Path, Type: r.Type, Size: r.Size}
			}
			assert.ElementsMatch(t, expected, actual)
		})
	}
}

func TestSearchLargestModeExcludesRoot(t *testing.T) {
	// Initialize the database if not already done
	if indexDB == nil {
		var err error
		indexDB, err = dbsql.NewIndexDB("test_search_largest", "OFF", 1000, 32, false)
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
	}

	index := Index{
		Source: settings.Source{
			Name: "test_search_largest",
			Path: "/mock/path",
		},
		db:   indexDB,
		mock: true,
	}

	// Insert test data into database
	now := time.Now()
	directories := map[string]*iteminfo.FileInfo{
		"/": {
			Files: []iteminfo.ExtendedItemInfo{
				{ItemInfo: iteminfo.ItemInfo{Name: "root-file.txt", Type: "text", Size: 100}},
			},
			ItemInfo: iteminfo.ItemInfo{
				Size: 3209322496, // Large root directory size
			},
		},
		"/subdir/": {
			Files: []iteminfo.ExtendedItemInfo{
				{ItemInfo: iteminfo.ItemInfo{Name: "sub-file.txt", Type: "text", Size: 200}},
			},
			ItemInfo: iteminfo.ItemInfo{
				Size: 5 * 1024 * 1024, // 5MB subdirectory
			},
		},
		"/another-dir/": {
			Files: []iteminfo.ExtendedItemInfo{
				{ItemInfo: iteminfo.ItemInfo{Name: "large-file.bin", Type: "binary", Size: 10 * 1024 * 1024}}, // 10MB file
			},
			ItemInfo: iteminfo.ItemInfo{
				Size: 10 * 1024 * 1024,
			},
		},
	}

	// Insert all directories and files into database
	for path, dirInfo := range directories {
		dirInfo.Path = path
		if dirInfo.ModTime.IsZero() {
			dirInfo.ModTime = now
		}
		if dirInfo.Type == "" {
			dirInfo.Type = "directory"
		}
		_ = index.db.InsertItem("test_search_largest", path, dirInfo)

		// Insert files
		for _, file := range dirInfo.Files {
			filePath := strings.TrimSuffix(path, "/") + "/" + file.Name
			fileInfo := &iteminfo.FileInfo{
				Path:     filePath,
				ItemInfo: file.ItemInfo,
			}
			if fileInfo.ModTime.IsZero() {
				fileInfo.ModTime = now
			}
			if fileInfo.Type == "" {
				fileInfo.Type = "file"
			}
			_ = index.db.InsertItem("test_search_largest", filePath, fileInfo)
		}
	}

	// Test that when largest=true and scope="/", the root directory "/" is NOT included
	result := index.Search("", "/", "test-session", true, DefaultSearchResults)

	// Verify that "/" is NOT in the results
	rootFound := false
	for _, r := range result {
		if r.Path == "/" {
			rootFound = true
			t.Errorf("Root directory '/' should not be included in results when largest=true, but found: %+v", r)
		}
	}

	if rootFound {
		t.Error("Root directory was incorrectly included in results")
	}

	// The test passes if root is excluded
	// (subdirectories and files may or may not be included depending on size filtering logic)
}

func TestSearchLargestModeExcludesScopeDirectory(t *testing.T) {
	// Initialize the database if not already done
	if indexDB == nil {
		var err error
		indexDB, err = dbsql.NewIndexDB("test_search_scope", "OFF", 1000, 32, false)
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
	}

	index := Index{
		Source: settings.Source{
			Name: "test_search_scope",
			Path: "/mock/path",
		},
		db:   indexDB,
		mock: true,
	}

	// Insert test data into database
	now := time.Now()
	directories := map[string]*iteminfo.FileInfo{
		"/": {
			Files: []iteminfo.ExtendedItemInfo{
				{ItemInfo: iteminfo.ItemInfo{Name: "root-file.txt", Type: "text", Size: 50}},
			},
			ItemInfo: iteminfo.ItemInfo{
				Size: 100,
			},
		},
		"/test/": {
			Files: []iteminfo.ExtendedItemInfo{
				{ItemInfo: iteminfo.ItemInfo{Name: "file1.txt", Type: "text", Size: 100}},
			},
			ItemInfo: iteminfo.ItemInfo{
				Size: 2 * 1024 * 1024, // 2MB directory
			},
		},
		"/test/subdir/": {
			Files: []iteminfo.ExtendedItemInfo{
				{ItemInfo: iteminfo.ItemInfo{Name: "file2.txt", Type: "text", Size: 200}},
			},
			ItemInfo: iteminfo.ItemInfo{
				Size: 3 * 1024 * 1024, // 3MB subdirectory
			},
		},
	}

	// Insert all directories and files into database
	for path, dirInfo := range directories {
		dirInfo.Path = path
		if dirInfo.ModTime.IsZero() {
			dirInfo.ModTime = now
		}
		if dirInfo.Type == "" {
			dirInfo.Type = "directory"
		}
		_ = index.db.InsertItem("test_search_scope", path, dirInfo)

		// Insert files
		for _, file := range dirInfo.Files {
			filePath := strings.TrimSuffix(path, "/") + "/" + file.Name
			fileInfo := &iteminfo.FileInfo{
				Path:     filePath,
				ItemInfo: file.ItemInfo,
			}
			if fileInfo.ModTime.IsZero() {
				fileInfo.ModTime = now
			}
			if fileInfo.Type == "" {
				fileInfo.Type = "file"
			}
			_ = index.db.InsertItem("test_search_scope", filePath, fileInfo)
		}
	}

	// Test that when largest=true and scope="/test/", the scope directory "/test/" is NOT included
	result := index.Search("", "/test/", "test-session", true, DefaultSearchResults)

	// Verify that "/test/" is NOT in the results
	scopeDirFound := false
	for _, r := range result {
		if r.Path == "/test/" {
			scopeDirFound = true
			t.Errorf("Scope directory '/test/' should not be included in results when largest=true, but found: %+v", r)
		}
	}

	if scopeDirFound {
		t.Error("Scope directory was incorrectly included in results")
	}

	// The main assertion: scope directory should be excluded
	// Note: When search is empty, scope gets cleared, so we search all directories
	// but still exclude the original scope directory "/test/" from results
	// Files and subdirectories may or may not be included depending on size/type filtering
}
