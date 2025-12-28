package indexing

import (
	"testing"

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
	source.Config.ResolvedConditionals = &settings.ResolvedConditionalsConfig{
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
		source.Config.ResolvedConditionals.IgnoreAllHidden = true
	}
	if conditionals.ZeroSizeFolders {
		source.Config.ResolvedConditionals.IgnoreAllZeroSizeFolders = true
	}

	// Build maps from ItemRules - match the real setConditionals implementation
	resolved := source.Config.ResolvedConditionals
	for _, rule := range conditionals.ItemRules {
		// Check if this is a root-level rule (folderPath == "/")
		// Root-level rules with ignoreHidden/ignoreZeroSizeFolders/viewable set global flags
		isRootLevelRule := rule.FolderPath == "/"

		// Infer global flags from root-level rules
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
		// Note: FileNames and FolderNames are NOT populated from rules in the real implementation
		// They remain empty maps (unused in current implementation)

		// Handle exact path matches
		if rule.FilePath != "" {
			resolved.FilePaths[rule.FilePath] = rule
		}
		if rule.FolderPath != "" {
			resolved.FolderPaths[rule.FolderPath] = rule
		}

		// Handle StartsWith/EndsWith (stored in slices)
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

		// Handle NeverWatchPath
		if rule.NeverWatchPath != "" {
			resolved.NeverWatchPaths[rule.NeverWatchPath] = struct{}{}
		}

		// Handle IncludeRootItem
		if rule.IncludeRootItem != "" {
			resolved.IncludeRootItems[rule.IncludeRootItem] = struct{}{}
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

			result := idx.shouldSkip(true, false, tt.fullPath, tt.baseName, actionConfig{})
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
			idx.Config.ResolvedConditionals.FolderNames[tt.ruleValue] = settings.ConditionalRule{FolderNames: tt.ruleValue}

			result := idx.shouldSkip(true, false, "/"+tt.baseName+"/", tt.baseName, actionConfig{})
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

			result := idx.shouldSkip(true, false, tt.fullPath, tt.baseName, actionConfig{})
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

			result := idx.shouldSkip(false, false, "/"+tt.baseName, tt.baseName, actionConfig{})
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

			result := idx.shouldSkip(false, false, "/"+tt.baseName, tt.baseName, actionConfig{})
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

			result := idx.shouldSkip(false, false, tt.fullPath, tt.baseName, actionConfig{})
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
		result := idx.shouldSkip(false, false, tt.fullPath, tt.baseName, actionConfig{})
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
	idx.Config.ResolvedConditionals.FolderNames["node_modules"] = settings.ConditionalRule{FolderNames: "node_modules"}
	idx.Config.ResolvedConditionals.FolderNames["@eaDir"] = settings.ConditionalRule{FolderNames: "@eaDir"}

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
		result := idx.shouldSkip(tt.isDir, false, tt.fullPath, tt.baseName, actionConfig{})
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

	// shouldSkip still returns true even with Viewable:true (it's checked separately)
	result := idx.shouldSkip(true, false, "/important/", "important", actionConfig{})
	if result != true {
		t.Errorf("Folder should be skipped (Viewable is checked separately), got shouldSkip=%v", result)
	}
}

func TestShouldSkip_HiddenFiles(t *testing.T) {
	idx := setupConditionalTestIndex(settings.ConditionalFilter{
		IgnoreHidden: true, // Skip hidden files
	})

	tests := []struct {
		isHidden   bool
		shouldSkip bool
	}{
		{true, true},
		{false, false},
	}

	for _, tt := range tests {
		result := idx.shouldSkip(false, tt.isHidden, "/file.txt", "file.txt", actionConfig{})
		if result != tt.shouldSkip {
			t.Errorf("isHidden=%v: expected shouldSkip=%v, got %v",
				tt.isHidden, tt.shouldSkip, result)
		}
	}
}

func TestShouldSkip_NeverWatch(t *testing.T) {
	// Test NeverWatch functionality - folders should be skipped during routine scans
	idx := setupConditionalTestIndex(settings.ConditionalFilter{
		ItemRules: []settings.ConditionalRule{
			{NeverWatchPath: "/cache/"}, // Must match the fullPath exactly
			{NeverWatchPath: "/logs/"},  // Must match the fullPath exactly
		},
	})

	// Simulate a routine scan (index has been scanned before, IsRoutineScan=true)
	// The IsRoutineScan flag is sufficient - GetLastIndexed() will check scanner.lastScanned
	config := actionConfig{IsRoutineScan: true}

	tests := []struct {
		name        string
		fullPath    string
		baseName    string
		shouldSkip  bool
		description string
	}{
		{
			name:        "Skip NeverWatch path during routine scan",
			fullPath:    "/cache/",
			baseName:    "cache",
			shouldSkip:  true,
			description: "Folder with neverWatch should be skipped in routine scan",
		},
		{
			name:        "Skip NeverWatch name during routine scan",
			fullPath:    "/logs/",
			baseName:    "logs",
			shouldSkip:  true,
			description: "Folder with neverWatch name should be skipped in routine scan",
		},
		{
			name:        "Don't skip regular folder",
			fullPath:    "/regular/",
			baseName:    "regular",
			shouldSkip:  false,
			description: "Regular folder should not be skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := idx.shouldSkip(true, false, tt.fullPath, tt.baseName, config)
			if result != tt.shouldSkip {
				t.Errorf("%s: expected shouldSkip=%v, got %v (fullPath=%s, baseName=%s)",
					tt.description, tt.shouldSkip, result, tt.fullPath, tt.baseName)
			}
		})
	}

	// Test initial scan (IsRoutineScan=false) - NeverWatch folders should NOT be skipped
	config.IsRoutineScan = false
	result := idx.shouldSkip(true, false, "/cache/", "cache", config)
	if result != false {
		t.Errorf("NeverWatch folder should NOT be skipped during initial scan, got shouldSkip=%v", result)
	}
}
