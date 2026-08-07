package indexing

import (
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

// setupConditionalTestIndex creates a test index with source rules configured.
func setupConditionalTestIndex(rules []settings.ConditionalRule) *Index {
	source := &settings.Source{
		Name: "test",
		Path: "/test/path",
		Config: settings.SourceConfig{
			Rules: rules,
		},
	}

	resolved := settings.ResolvedRulesConfig{
		FileNames:                make(map[string]settings.ConditionalRule),
		FolderNames:              make(map[string]settings.ConditionalRule),
		FilePaths:                make(map[string]settings.ConditionalRule),
		FolderPaths:              make(map[string]settings.ConditionalRule),
		NeverWatchPaths:          make(map[string]struct{}),
		IncludeRootItems:         make(map[string]struct{}),
		IgnoreAllHidden:          false,
		IgnoreAllZeroSizeFolders: false,
		IgnoreAllSymlinks:        false,
		IndexingDisabled:         false,
		NoRules:                  len(rules) == 0,
	}

	for _, rule := range rules {
		isRootLevelRule := rule.FolderPath == "/"
		if isRootLevelRule {
			if rule.IgnoreHidden {
				resolved.IgnoreAllHidden = true
			}
			if rule.IgnoreSymlinks {
				resolved.IgnoreAllSymlinks = true
			}
			if rule.IgnoreZeroSizeFolders {
				resolved.IgnoreAllZeroSizeFolders = true
			}
			if rule.Viewable {
				resolved.IndexingDisabled = true
			}
		}

		if rule.FilePath != "" {
			resolved.FilePaths[rule.FilePath] = rule
		}
		if rule.FolderPath != "" {
			resolved.FolderPaths[rule.FolderPath] = rule
		}
		if rule.FileEndsWith != "" {
			resolved.FileEndsWith = append(resolved.FileEndsWith, rule)
		}
		if rule.FolderEndsWith != "" {
			resolved.FolderEndsWith = append(resolved.FolderEndsWith, rule)
		}
		if rule.FileStartsWith != "" {
			resolved.FileStartsWith = append(resolved.FileStartsWith, rule)
		}
		if rule.FolderStartsWith != "" {
			resolved.FolderStartsWith = append(resolved.FolderStartsWith, rule)
		}
		if rule.NeverWatchPath != "" {
			resolved.NeverWatchPaths[rule.NeverWatchPath] = struct{}{}
		}
		if rule.IncludeRootItem != "" {
			resolved.IncludeRootItems[rule.IncludeRootItem] = struct{}{}
		}
		if rule.FileName != "" {
			resolved.FileNames[rule.FileName] = rule
		}
		if rule.FolderName != "" {
			resolved.FolderNames[rule.FolderName] = rule
		}
	}

	source.Config.ResolvedRules = resolved

	return &Index{
		Source: *source,
		mock:   true,
	}
}

func TestShouldSkip_FolderStartsWith(t *testing.T) {
	tests := []struct {
		name        string
		ruleValue   string
		fullPath    string
		baseName    string
		shouldSkip  bool
		description string
	}{
		{
			name:        "Skip folder starting with 'graham'",
			ruleValue:   "graham",   // After normalization (no leading slash)
			fullPath:    "/graham/", // Full index path
			baseName:    "graham",   // Base name
			shouldSkip:  true,
			description: "Folder 'graham' should be skipped",
		},
		{
			name:        "Skip folder starting with 'temp'",
			ruleValue:   "temp",
			fullPath:    "/temp/",
			baseName:    "temp",
			shouldSkip:  true,
			description: "Folder 'temp' should be skipped",
		},
		{
			name:        "Skip folder starting with 'tmp-'",
			ruleValue:   "tmp-",
			fullPath:    "/tmp-backup/",
			baseName:    "tmp-backup",
			shouldSkip:  true,
			description: "Folder 'tmp-backup' starts with 'tmp-' and should be skipped",
		},
		{
			name:        "Don't skip folder not starting with rule",
			ruleValue:   "graham",
			fullPath:    "/other/",
			baseName:    "other",
			shouldSkip:  false,
			description: "Folder 'other' doesn't start with 'graham'",
		},
		{
			name:        "Don't skip folder that contains but doesn't start",
			ruleValue:   "graham",
			fullPath:    "/mygraham/",
			baseName:    "mygraham",
			shouldSkip:  false,
			description: "Folder 'mygraham' contains but doesn't start with 'graham'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := setupConditionalTestIndex([]settings.ConditionalRule{
					{FolderStartsWith: tt.ruleValue},
			})

			result := idx.ShouldSkip(true, tt.fullPath, false, false, false)
			if result != tt.shouldSkip {
				t.Errorf("%s: expected shouldSkip=%v, got %v (fullPath=%s, baseName=%s, rule=%s)",
					tt.description, tt.shouldSkip, result, tt.fullPath, tt.baseName, tt.ruleValue)
			}
		})
	}
}

func TestShouldSkip_FolderNames(t *testing.T) {
	tests := []struct {
		name        string
		ruleValue   string
		baseName    string
		shouldSkip  bool
		description string
	}{
		{
			name:        "Skip exact folder name match",
			ruleValue:   "node_modules",
			baseName:    "node_modules",
			shouldSkip:  true,
			description: "Exact match should be skipped",
		},
		{
			name:        "Skip folder name '@eaDir'",
			ruleValue:   "@eaDir",
			baseName:    "@eaDir",
			shouldSkip:  true,
			description: "Synology thumbnail folder should be skipped",
		},
		{
			name:        "Don't skip partial match",
			ruleValue:   "node_modules",
			baseName:    "node_modules_backup",
			shouldSkip:  false,
			description: "Partial match should not be skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := setupConditionalTestIndex([]settings.ConditionalRule{
				{FolderName: tt.ruleValue},
			})

			result := idx.ShouldSkip(true, "/"+tt.baseName+"/", false, false, false)
			if result != tt.shouldSkip {
				t.Errorf("%s: expected shouldSkip=%v, got %v (baseName=%s, rule=%s)",
					tt.description, tt.shouldSkip, result, tt.baseName, tt.ruleValue)
			}
		})
	}
}

func TestShouldSkip_FolderPaths(t *testing.T) {
	tests := []struct {
		name        string
		ruleValue   string
		fullPath    string
		baseName    string
		shouldSkip  bool
		description string
	}{
		{
			name:        "Skip exact folder path",
			ruleValue:   "/graham/", // Exact path with trailing slash
			fullPath:    "/graham/", // Full index path
			baseName:    "graham",   // Base name
			shouldSkip:  true,
			description: "Exact path match should be skipped (O(1) lookup)",
		},
		{
			name:        "Skip subfolder with prefix matching",
			ruleValue:   "/graham/",
			fullPath:    "/graham/subfolder/",
			baseName:    "subfolder",
			shouldSkip:  true,
			description: "FolderPaths now does prefix matching for child folders",
		},
		{
			name:        "Don't skip unrelated path",
			ruleValue:   "/graham/",
			fullPath:    "/other/",
			baseName:    "other",
			shouldSkip:  false,
			description: "Unrelated path should not be skipped",
		},
		{
			name:        "Skip nested path and its children",
			ruleValue:   "/projects/old/",
			fullPath:    "/projects/old/backup/",
			baseName:    "backup",
			shouldSkip:  true,
			description: "Nested path under rule should be skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := setupConditionalTestIndex([]settings.ConditionalRule{
					{FolderPath: tt.ruleValue},
			})

			result := idx.ShouldSkip(true, tt.fullPath, false, false, false)
			if result != tt.shouldSkip {
				t.Errorf("%s: expected shouldSkip=%v, got %v (fullPath=%s, baseName=%s, rule=%s)",
					tt.description, tt.shouldSkip, result, tt.fullPath, tt.baseName, tt.ruleValue)
			}
		})
	}
}

func TestShouldSkip_FileStartsWith(t *testing.T) {
	tests := []struct {
		name        string
		ruleValue   string
		baseName    string
		shouldSkip  bool
		description string
	}{
		{
			name:        "Skip file starting with 'Docker'",
			ruleValue:   "Docker", // After normalization (no leading slash)
			baseName:    "Docker.dmg",
			shouldSkip:  true,
			description: "File 'Docker.dmg' should be skipped",
		},
		{
			name:        "Skip file starting with '.'",
			ruleValue:   ".",
			baseName:    ".DS_Store",
			shouldSkip:  true,
			description: "Hidden file should be skipped",
		},
		{
			name:        "Don't skip file not starting with rule",
			ruleValue:   "tmp",
			baseName:    "document.txt",
			shouldSkip:  false,
			description: "File not starting with 'tmp' should not be skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := setupConditionalTestIndex([]settings.ConditionalRule{
					{FileStartsWith: tt.ruleValue},
			})

			result := idx.ShouldSkip(false, "/"+tt.baseName, false, false, false)
			if result != tt.shouldSkip {
				t.Errorf("%s: expected shouldSkip=%v, got %v (baseName=%s, rule=%s)",
					tt.description, tt.shouldSkip, result, tt.baseName, tt.ruleValue)
			}
		})
	}
}

func TestShouldSkip_FileEndsWith(t *testing.T) {
	tests := []struct {
		name        string
		ruleValue   string
		baseName    string
		shouldSkip  bool
		description string
	}{
		{
			name:        "Skip file ending with '.tmp'",
			ruleValue:   ".tmp",
			baseName:    "document.tmp",
			shouldSkip:  true,
			description: "Temporary file should be skipped",
		},
		{
			name:        "Skip file ending with '~'",
			ruleValue:   "~",
			baseName:    "document.txt~",
			shouldSkip:  true,
			description: "Backup file should be skipped",
		},
		{
			name:        "Don't skip file with different extension",
			ruleValue:   ".tmp",
			baseName:    "document.txt",
			shouldSkip:  false,
			description: "Regular file should not be skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := setupConditionalTestIndex([]settings.ConditionalRule{
					{FileEndsWith: tt.ruleValue},
			})

			result := idx.ShouldSkip(false, "/"+tt.baseName, false, false, false)
			if result != tt.shouldSkip {
				t.Errorf("%s: expected shouldSkip=%v, got %v (baseName=%s, rule=%s)",
					tt.description, tt.shouldSkip, result, tt.baseName, tt.ruleValue)
			}
		})
	}
}

func TestShouldSkip_FilePaths(t *testing.T) {
	tests := []struct {
		name        string
		ruleValue   string
		fullPath    string
		baseName    string
		shouldSkip  bool
		description string
	}{
		{
			name:        "Skip exact file path",
			ruleValue:   "/config.txt",
			fullPath:    "/config.txt",
			baseName:    "config.txt",
			shouldSkip:  true,
			description: "Exact file path match should be skipped (O(1) lookup)",
		},
		{
			name:        "Skip file in excluded folder with prefix matching",
			ruleValue:   "/logs",
			fullPath:    "/logs/app.log",
			baseName:    "app.log",
			shouldSkip:  true,
			description: "FilePaths now does prefix matching for files in excluded folders",
		},
		{
			name:        "Don't skip file in different folder",
			ruleValue:   "/logs",
			fullPath:    "/data/file.txt",
			baseName:    "file.txt",
			shouldSkip:  false,
			description: "File in different folder should not be skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := setupConditionalTestIndex([]settings.ConditionalRule{
					{FilePath: tt.ruleValue},
			})

			result := idx.ShouldSkip(false, tt.fullPath, false, false, false)
			if result != tt.shouldSkip {
				t.Errorf("%s: expected shouldSkip=%v, got %v (fullPath=%s, rule=%s)",
					tt.description, tt.shouldSkip, result, tt.fullPath, tt.ruleValue)
			}
		})
	}
}

func TestShouldSkip_FileInExcludedFolderPath(t *testing.T) {
	// Test that files in excluded folder paths are also skipped
	// Uses FilePaths rule which applies to both the folder and its files via prefix matching
	idx := setupConditionalTestIndex([]settings.ConditionalRule{
			{FilePath: "/graham/"			},
			})

	tests := []struct {
		fullPath   string
		baseName   string
		shouldSkip bool
		reason     string
	}{
		{"/graham/file.txt", "file.txt", true, "File in excluded folder should be skipped"},
		{"/graham/subfolder/file.txt", "file.txt", true, "File in subfolder of excluded folder should be skipped"},
		{"/other/file.txt", "file.txt", false, "File in different folder should not be skipped"},
	}

	for _, tt := range tests {
		result := idx.ShouldSkip(false, tt.fullPath, false, false, false)
		if result != tt.shouldSkip {
			t.Errorf("File %s (%s): expected shouldSkip=%v, got %v",
				tt.fullPath, tt.reason, tt.shouldSkip, result)
		}
	}
}

func TestShouldSkip_MultipleRules(t *testing.T) {
	// Test that multiple rules work together
	idx := setupConditionalTestIndex([]settings.ConditionalRule{
		{FolderStartsWith: "temp"},
		{FolderStartsWith: "tmp"},
		{FolderName: "node_modules"},
		{FolderName: "@eaDir"},
		{FileStartsWith: "."},
		{FileEndsWith: ".tmp"},
		{FileEndsWith: "~"},
	})

	tests := []struct {
		isDir      bool
		fullPath   string
		baseName   string
		shouldSkip bool
		reason     string
	}{
		// Folders
		{true, "/temp/", "temp", true, "starts with temp"},
		{true, "/tmp-backup/", "tmp-backup", true, "starts with tmp"},
		{true, "/node_modules/", "node_modules", true, "exact name match"},
		{true, "/@eaDir/", "@eaDir", true, "exact name match"},
		{true, "/regular/", "regular", false, "no rule matches"},

		// Files
		{false, "/.DS_Store", ".DS_Store", true, "starts with ."},
		{false, "/file.tmp", "file.tmp", true, "ends with .tmp"},
		{false, "/file.txt~", "file.txt~", true, "ends with ~"},
		{false, "/document.txt", "document.txt", false, "no rule matches"},
	}

	for _, tt := range tests {
		result := idx.ShouldSkip(tt.isDir, tt.fullPath, false, false, false)
		if result != tt.shouldSkip {
			t.Errorf("%s (%s): expected shouldSkip=%v, got %v",
				tt.baseName, tt.reason, tt.shouldSkip, result)
		}
	}
}

func TestShouldSkip_ViewableStillSkips(t *testing.T) {
	// Test that Viewable:true doesn't affect shouldSkip behavior
	// The Viewable field is checked separately in IsViewable(), not in shouldSkip()
	idx := setupConditionalTestIndex([]settings.ConditionalRule{
		{FolderStartsWith: "important", Viewable: true}, // Viewable but still skipped from indexing
	})

	// ShouldSkip still returns true even with Viewable:true (it's checked separately)
	result := idx.ShouldSkip(true, "/important/", false, false, false)
	if result != true {
		t.Errorf("Folder should be skipped (Viewable is checked separately), got shouldSkip=%v", result)
	}
}

func TestShouldSkip_HiddenFiles(t *testing.T) {
	idx := setupConditionalTestIndex(nil)
	idx.Config.ResolvedRules.NoRules = false
	idx.Config.ResolvedRules.IgnoreAllHidden = true

	tests := []struct {
		path       string
		shouldSkip bool
	}{
		{"/.hiddenfile", true},       // Hidden file (starts with .)
		{"/file.txt", false},         // Regular file
		{"/.hidden/file.txt", false}, // File in hidden directory (but file itself isn't hidden)
	}

	for _, tt := range tests {
		// For hidden file test, extract if the path starts with "/."
		isHiddenPath := IsHidden(tt.path)
		result := idx.ShouldSkip(false, tt.path, isHiddenPath, false, false)
		if result != tt.shouldSkip {
			t.Errorf("path=%s: expected shouldSkip=%v, got %v",
				tt.path, tt.shouldSkip, result)
		}
	}
}

func TestIsNeverWatchPath(t *testing.T) {
	// Test NeverWatchPath functionality - paths indexed once, then never re-indexed
	idx := setupConditionalTestIndex([]settings.ConditionalRule{
		{NeverWatchPath: "/temp/"},
		{NeverWatchPath: "/cache/"},
	})

	tests := []struct {
		name            string
		path            string
		hasBeenIndexed  bool
		expectedSkipped bool
		description     string
	}{
		{
			name:            "Initial scan - don't skip NeverWatch",
			path:            "/temp/",
			hasBeenIndexed:  false,
			expectedSkipped: false,
			description:     "NeverWatch paths should NOT be skipped during initial scan",
		},
		{
			name:            "Routine scan - skip NeverWatch",
			path:            "/temp/",
			hasBeenIndexed:  true,
			expectedSkipped: true,
			description:     "NeverWatch paths SHOULD be skipped after initial scan",
		},
		{
			name:            "Initial scan - regular path",
			path:            "/regular/",
			hasBeenIndexed:  false,
			expectedSkipped: false,
			description:     "Regular paths should not be skipped during initial scan",
		},
		{
			name:            "Routine scan - regular path",
			path:            "/regular/",
			hasBeenIndexed:  true,
			expectedSkipped: false,
			description:     "Regular paths should not be skipped during routine scans",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate whether index has been scanned before
			if tt.hasBeenIndexed {
				// Set a fake last indexed time to simulate completed scan
				idx.mu.Lock()
				if idx.scanners == nil {
					idx.scanners = make(map[string]*Scanner)
				}
				scanner := &Scanner{
					lastScanned: time.Now(),
				}
				idx.scanners["/"] = scanner
				idx.mu.Unlock()
			} else {
				// Clear scanners to simulate initial state
				idx.mu.Lock()
				idx.scanners = make(map[string]*Scanner)
				idx.mu.Unlock()
			}

			result := idx.IsNeverWatchPath(tt.path)
			if result != tt.expectedSkipped {
				t.Errorf("%s: expected IsNeverWatchPath=%v, got %v (path=%s, hasBeenIndexed=%v)",
					tt.description, tt.expectedSkipped, result, tt.path, tt.hasBeenIndexed)
			}
		})
	}
}

// TestIsViewable_DoesNotMutateFolderPaths verifies that IsViewable doesn't modify FolderPaths
func TestIsViewable_DoesNotMutateFolderPaths(t *testing.T) {
	idx := setupConditionalTestIndex(nil)
	idx.Config.ResolvedRules.NoRules = false
	idx.Config.ResolvedRules.FolderNames["private"] = settings.ConditionalRule{
		FolderName: "private",
		Viewable:   false,
	}

	if len(idx.Config.ResolvedRules.FolderPaths) != 0 {
		t.Fatalf("expected no preloaded folder path rules, got %d", len(idx.Config.ResolvedRules.FolderPaths))
	}

	if idx.IsViewable(true, "/private/", false, false) {
		t.Fatal("expected /private/ to be non-viewable")
	}

	if len(idx.Config.ResolvedRules.FolderPaths) != 0 {
		t.Fatalf("IsViewable should not mutate FolderPaths, got %d entries", len(idx.Config.ResolvedRules.FolderPaths))
	}
}

// TestIsViewableWithParentCheck_RecursiveParentWalking tests API request parent checking
func TestIsViewableWithParentCheck_RecursiveParentWalking(t *testing.T) {
	idx := setupConditionalTestIndex(nil)
	idx.Config.ResolvedRules.NoRules = false
	idx.Config.ResolvedRules.FolderPaths["/private"] = settings.ConditionalRule{
		FolderPath: "/private",
		Viewable:   false,
	}
	// Enable global hidden folder blocking
	idx.Config.ResolvedRules.IgnoreAllHidden = true

	tests := []struct {
		name        string
		path        string
		isDir       bool
		expected    bool
		description string
	}{
		{
			name:        "Direct non-viewable folder",
			path:        "/private",
			isDir:       true,
			expected:    false,
			description: "Folder with explicit non-viewable rule should not be viewable",
		},
		{
			name:        "Child of non-viewable folder",
			path:        "/private/subfolder",
			isDir:       true,
			expected:    false,
			description: "Child should inherit parent's non-viewable status",
		},
		{
			name:        "Deep child of non-viewable folder",
			path:        "/private/subfolder/deep/nested",
			isDir:       true,
			expected:    false,
			description: "Deeply nested child should inherit parent's non-viewable status",
		},
		{
			name:        "File in non-viewable folder",
			path:        "/private/secret.txt",
			isDir:       false,
			expected:    false,
			description: "File should inherit parent folder's non-viewable status",
		},
		{
			name:        "Unrelated folder",
			path:        "/public/documents",
			isDir:       true,
			expected:    true,
			description: "Unrelated folder should be viewable",
		},
		{
			name:        "Hidden folder",
			path:        "/.hiddenDir",
			isDir:       true,
			expected:    false,
			description: "Hidden folder should not be viewable when IgnoreAllHidden is true",
		},
		{
			name:        "File in hidden folder",
			path:        "/.hiddenDir/nested.txt",
			isDir:       false,
			expected:    false,
			description: "File in hidden folder should inherit parent's non-viewable status",
		},
		{
			name:        "Nested hidden folder",
			path:        "/subfolderExclusions/.hiddenDir",
			isDir:       true,
			expected:    false,
			description: "Nested hidden folder should not be viewable",
		},
		{
			name:        "File in nested hidden folder",
			path:        "/subfolderExclusions/.hiddenDir/nested.txt",
			isDir:       false,
			expected:    false,
			description: "File in nested hidden folder should not be viewable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Determine if the path is hidden
			realPath := utils.JoinPathAsUnix(idx.Path, tt.path)
			isHidden := IsHidden(realPath)

			result := idx.IsViewableWithParentCheck(tt.isDir, tt.path, false, isHidden)
			if result != tt.expected {
				t.Errorf("%s: expected %v, got %v (path=%s, isHidden=%v)",
					tt.description, tt.expected, result, tt.path, isHidden)
			}
		})
	}
}

// TestIsViewableWithParentCheck_DoesNotMutate verifies thread safety
func TestIsViewableWithParentCheck_DoesNotMutate(t *testing.T) {
	idx := setupConditionalTestIndex(nil)
	idx.Config.ResolvedRules.NoRules = false
	idx.Config.ResolvedRules.FolderNames["restricted"] = settings.ConditionalRule{
		FolderName: "restricted",
		Viewable:    false,
	}

	initialFolderPathCount := len(idx.Config.ResolvedRules.FolderPaths)

	// Call IsViewableWithParentCheck multiple times with various paths
	paths := []string{
		"/restricted/data/file.txt",
		"/projects/restricted/subfolder/doc.pdf",
		"/restricted/deep/nested/path/file.csv",
	}

	for _, path := range paths {
		_ = idx.IsViewableWithParentCheck(false, path, false, false)
	}

	// Verify no mutation occurred
	if len(idx.Config.ResolvedRules.FolderPaths) != initialFolderPathCount {
		t.Errorf("FolderPaths was mutated: expected %d, got %d",
			initialFolderPathCount, len(idx.Config.ResolvedRules.FolderPaths))
	}
}
