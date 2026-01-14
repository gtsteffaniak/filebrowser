package indexing

import (
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
)

// setupConditionalTestIndex creates a test index with conditional rules configured
func setupConditionalTestIndex(conditionals settings.ConditionalFilter) *Index {
	source := &settings.Source{
		Name: "test",
		Path: "/test/path",
		Config: settings.SourceConfig{
			DisableIndexing: false,
			Conditionals:    conditionals,
		},
	}

	// Build the conditional maps (this is normally done during config loading)
	source.Config.ResolvedRules = settings.ResolvedRulesConfig{
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
	}

	// Backwards compatibility: if old format fields are set, treat as global rules
	if conditionals.IgnoreHidden {
		source.Config.ResolvedRules.IgnoreAllHidden = true
	}
	if conditionals.ZeroSizeFolders {
		source.Config.ResolvedRules.IgnoreAllZeroSizeFolders = true
	}

	// Build maps from ItemRules - match the real setConditionals implementation
	for _, rule := range conditionals.ItemRules {
		// Check if this is a root-level rule (folderPath == "/")
		// Root-level rules with ignoreHidden/ignoreZeroSizeFolders/viewable set global flags
		isRootLevelRule := rule.FolderPath == "/"

		// Infer global flags from root-level rules
		if isRootLevelRule {
			if rule.IgnoreHidden {
				source.Config.ResolvedRules.IgnoreAllHidden = true
			}
			if rule.IgnoreSymlinks {
				source.Config.ResolvedRules.IgnoreAllSymlinks = true
			}
			if rule.IgnoreZeroSizeFolders {
				source.Config.ResolvedRules.IgnoreAllZeroSizeFolders = true
			}
			if rule.Viewable {
				source.Config.ResolvedRules.IndexingDisabled = true
			}
		}
		// Note: FileNames and FolderNames are NOT populated from rules in the real implementation
		// They remain empty maps (unused in current implementation)

		// Handle exact path matches
		if rule.FilePath != "" {
			source.Config.ResolvedRules.FilePaths[rule.FilePath] = rule
		}
		if rule.FolderPath != "" {
			source.Config.ResolvedRules.FolderPaths[rule.FolderPath] = rule
		}

		// Handle StartsWith/EndsWith (stored in slices)
		if rule.FileEndsWith != "" {
			source.Config.ResolvedRules.FileEndsWith = append(source.Config.ResolvedRules.FileEndsWith, rule)
		}
		if rule.FolderEndsWith != "" {
			source.Config.ResolvedRules.FolderEndsWith = append(source.Config.ResolvedRules.FolderEndsWith, rule)
		}
		if rule.FileStartsWith != "" {
			source.Config.ResolvedRules.FileStartsWith = append(source.Config.ResolvedRules.FileStartsWith, rule)
		}
		if rule.FolderStartsWith != "" {
			source.Config.ResolvedRules.FolderStartsWith = append(source.Config.ResolvedRules.FolderStartsWith, rule)
		}

		// Handle NeverWatchPath
		if rule.NeverWatchPath != "" {
			source.Config.ResolvedRules.NeverWatchPaths[rule.NeverWatchPath] = struct{}{}
		}

		// Handle IncludeRootItem
		if rule.IncludeRootItem != "" {
			source.Config.ResolvedRules.IncludeRootItems[rule.IncludeRootItem] = struct{}{}
		}
	}

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
			idx := setupConditionalTestIndex(settings.ConditionalFilter{
				ItemRules: []settings.ConditionalRule{
					{FolderStartsWith: tt.ruleValue},
				},
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
			idx := setupConditionalTestIndex(settings.ConditionalFilter{
				ItemRules: []settings.ConditionalRule{
					{FolderNames: tt.ruleValue},
				},
			})

			// Manually populate FolderNames map since setConditionals doesn't do it
			idx.Config.ResolvedRules.FolderNames[tt.ruleValue] = settings.ConditionalRule{FolderNames: tt.ruleValue}

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
			idx := setupConditionalTestIndex(settings.ConditionalFilter{
				ItemRules: []settings.ConditionalRule{
					{FolderPath: tt.ruleValue},
				},
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
			idx := setupConditionalTestIndex(settings.ConditionalFilter{
				ItemRules: []settings.ConditionalRule{
					{FileStartsWith: tt.ruleValue},
				},
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
			idx := setupConditionalTestIndex(settings.ConditionalFilter{
				ItemRules: []settings.ConditionalRule{
					{FileEndsWith: tt.ruleValue},
				},
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
			idx := setupConditionalTestIndex(settings.ConditionalFilter{
				ItemRules: []settings.ConditionalRule{
					{FilePath: tt.ruleValue},
				},
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
	idx := setupConditionalTestIndex(settings.ConditionalFilter{
		ItemRules: []settings.ConditionalRule{
			{FilePath: "/graham/"},
		},
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
	idx := setupConditionalTestIndex(settings.ConditionalFilter{
		ItemRules: []settings.ConditionalRule{
			{FolderStartsWith: "temp"},
			{FolderStartsWith: "tmp"},
			{FolderNames: "node_modules"},
			{FolderNames: "@eaDir"},
			{FileStartsWith: "."},
			{FileEndsWith: ".tmp"},
			{FileEndsWith: "~"},
		},
	})

	// Manually populate FolderNames map
	idx.Config.ResolvedRules.FolderNames["node_modules"] = settings.ConditionalRule{FolderNames: "node_modules"}
	idx.Config.ResolvedRules.FolderNames["@eaDir"] = settings.ConditionalRule{FolderNames: "@eaDir"}

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
	idx := setupConditionalTestIndex(settings.ConditionalFilter{
		ItemRules: []settings.ConditionalRule{
			{FolderStartsWith: "important", Viewable: true}, // Viewable but still skipped from indexing
		},
	})

	// ShouldSkip still returns true even with Viewable:true (it's checked separately)
	result := idx.ShouldSkip(true, "/important/", false, false, false)
	if result != true {
		t.Errorf("Folder should be skipped (Viewable is checked separately), got shouldSkip=%v", result)
	}
}

func TestShouldSkip_HiddenFiles(t *testing.T) {
	idx := setupConditionalTestIndex(settings.ConditionalFilter{
		IgnoreHidden: true, // Skip hidden files
	})

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
	idx := setupConditionalTestIndex(settings.ConditionalFilter{
		ItemRules: []settings.ConditionalRule{
			{NeverWatchPath: "/temp/"},
			{NeverWatchPath: "/cache/"},
		},
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
