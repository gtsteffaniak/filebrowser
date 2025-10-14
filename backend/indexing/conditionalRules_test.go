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
	source.Config.ConditionalsMap = &settings.ConditionalMaps{
		FileNamesMap:        make(map[string]settings.ConditionalIndexConfig),
		FolderNamesMap:      make(map[string]settings.ConditionalIndexConfig),
		FilePathsMap:        make(map[string]settings.ConditionalIndexConfig),
		FolderPathsMap:      make(map[string]settings.ConditionalIndexConfig),
		FileEndsWithMap:     make(map[string]settings.ConditionalIndexConfig),
		FolderEndsWithMap:   make(map[string]settings.ConditionalIndexConfig),
		FileStartsWithMap:   make(map[string]settings.ConditionalIndexConfig),
		FolderStartsWithMap: make(map[string]settings.ConditionalIndexConfig),
	}

	// Build maps
	maps := source.Config.ConditionalsMap
	for _, rule := range conditionals.FileNames {
		maps.FileNamesMap[rule.Value] = rule
	}
	for _, rule := range conditionals.FolderNames {
		maps.FolderNamesMap[rule.Value] = rule
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
				FolderStartsWith: []settings.ConditionalIndexConfig{
					{Value: tt.ruleValue, Index: false},
				},
			})

			result := idx.shouldSkip(true, false, tt.fullPath, tt.baseName, nil)
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
				FolderNames: []settings.ConditionalIndexConfig{
					{Value: tt.ruleValue, Index: false},
				},
			})

			result := idx.shouldSkip(true, false, "/"+tt.baseName+"/", tt.baseName, nil)
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
			ruleValue:   "/graham",  // After normalization (with leading slash)
			fullPath:    "/graham/", // Full index path
			baseName:    "graham",   // Base name
			shouldSkip:  true,
			description: "Exact path match should be skipped",
		},
		{
			name:        "Skip folder path prefix",
			ruleValue:   "/graham",
			fullPath:    "/graham/subfolder/",
			baseName:    "subfolder",
			shouldSkip:  true,
			description: "Path starting with /graham should be skipped",
		},
		{
			name:        "Don't skip unrelated path",
			ruleValue:   "/graham",
			fullPath:    "/other/",
			baseName:    "other",
			shouldSkip:  false,
			description: "Unrelated path should not be skipped",
		},
		{
			name:        "Skip nested path",
			ruleValue:   "/projects/old",
			fullPath:    "/projects/old/backup/",
			baseName:    "backup",
			shouldSkip:  true,
			description: "Nested path under rule should be skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := setupConditionalTestIndex(settings.ConditionalFilter{
				FolderPaths: []settings.ConditionalIndexConfig{
					{Value: tt.ruleValue, Index: false},
				},
			})

			result := idx.shouldSkip(true, false, tt.fullPath, tt.baseName, nil)
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
				FileStartsWith: []settings.ConditionalIndexConfig{
					{Value: tt.ruleValue, Index: false},
				},
			})

			result := idx.shouldSkip(false, false, "/"+tt.baseName, tt.baseName, nil)
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
				FileEndsWith: []settings.ConditionalIndexConfig{
					{Value: tt.ruleValue, Index: false},
				},
			})

			result := idx.shouldSkip(false, false, "/"+tt.baseName, tt.baseName, nil)
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
			description: "Exact file path match should be skipped",
		},
		{
			name:        "Skip file in excluded folder",
			ruleValue:   "/logs",
			fullPath:    "/logs/app.log",
			baseName:    "app.log",
			shouldSkip:  true,
			description: "File in excluded folder should be skipped",
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
				FilePaths: []settings.ConditionalIndexConfig{
					{Value: tt.ruleValue, Index: false},
				},
			})

			result := idx.shouldSkip(false, false, tt.fullPath, tt.baseName, nil)
			if result != tt.shouldSkip {
				t.Errorf("%s: expected shouldSkip=%v, got %v (fullPath=%s, rule=%s)",
					tt.description, tt.shouldSkip, result, tt.fullPath, tt.ruleValue)
			}
		})
	}
}

func TestShouldSkip_FileInExcludedFolderPath(t *testing.T) {
	// Special test for files whose parent directory matches FolderPaths
	idx := setupConditionalTestIndex(settings.ConditionalFilter{
		FolderPaths: []settings.ConditionalIndexConfig{
			{Value: "/graham", Index: false},
		},
	})

	tests := []struct {
		fullPath   string
		baseName   string
		shouldSkip bool
	}{
		{"/graham/file.txt", "file.txt", true},
		{"/graham/subfolder/file.txt", "file.txt", true},
		{"/other/file.txt", "file.txt", false},
	}

	for _, tt := range tests {
		result := idx.shouldSkip(false, false, tt.fullPath, tt.baseName, nil)
		if result != tt.shouldSkip {
			t.Errorf("File %s: expected shouldSkip=%v, got %v",
				tt.fullPath, tt.shouldSkip, result)
		}
	}
}

func TestShouldSkip_MultipleRules(t *testing.T) {
	// Test that multiple rules work together
	idx := setupConditionalTestIndex(settings.ConditionalFilter{
		FolderStartsWith: []settings.ConditionalIndexConfig{
			{Value: "temp", Index: false},
			{Value: "tmp", Index: false},
		},
		FolderNames: []settings.ConditionalIndexConfig{
			{Value: "node_modules", Index: false},
			{Value: "@eaDir", Index: false},
		},
		FileStartsWith: []settings.ConditionalIndexConfig{
			{Value: ".", Index: false},
		},
		FileEndsWith: []settings.ConditionalIndexConfig{
			{Value: ".tmp", Index: false},
			{Value: "~", Index: false},
		},
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
		result := idx.shouldSkip(tt.isDir, false, tt.fullPath, tt.baseName, nil)
		if result != tt.shouldSkip {
			t.Errorf("%s (%s): expected shouldSkip=%v, got %v",
				tt.baseName, tt.reason, tt.shouldSkip, result)
		}
	}
}

func TestShouldSkip_IndexTrueAllowsIndexing(t *testing.T) {
	// Test that Index:true allows indexing (opposite of Index:false)
	idx := setupConditionalTestIndex(settings.ConditionalFilter{
		FolderStartsWith: []settings.ConditionalIndexConfig{
			{Value: "important", Index: true}, // Explicitly allow
		},
	})

	// Should NOT skip folders starting with "important"
	result := idx.shouldSkip(true, false, "/important/", "important", nil)
	if result != false {
		t.Errorf("Folder with Index:true should NOT be skipped, got shouldSkip=%v", result)
	}
}

func TestShouldSkip_HiddenFiles(t *testing.T) {
	idx := setupConditionalTestIndex(settings.ConditionalFilter{
		Hidden: true, // Skip hidden files
	})

	tests := []struct {
		isHidden   bool
		shouldSkip bool
	}{
		{true, true},
		{false, false},
	}

	for _, tt := range tests {
		result := idx.shouldSkip(false, tt.isHidden, "/file.txt", "file.txt", nil)
		if result != tt.shouldSkip {
			t.Errorf("isHidden=%v: expected shouldSkip=%v, got %v",
				tt.isHidden, tt.shouldSkip, result)
		}
	}
}
