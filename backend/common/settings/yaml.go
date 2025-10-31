package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// combineYAMLFiles combines multiple YAML files from a directory into a single YAML document
// This allows YAML anchors defined in one file to be referenced in another
//
// Pattern matching: If main config is "config.yaml", only files matching "*-config.yaml" are merged
// Examples:
//   - config.yaml + server-config.yaml + auth-config.yaml
//   - test.yaml + database-test.yaml + frontend-test.yaml
//   - myapp.yml + server-myapp.yml
func combineYAMLFiles(configFilePath string) ([]byte, error) {
	// Get absolute path and expand tilde
	expandedPath := configFilePath
	if strings.HasPrefix(configFilePath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to expand home directory: %v", err)
		}
		expandedPath = filepath.Join(homeDir, configFilePath[2:])
	}

	absPath, err := filepath.Abs(expandedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config file path: %v", err)
	}

	configDir := filepath.Dir(absPath)
	configFileName := filepath.Base(absPath)

	// Extract base name pattern for matching related config files
	// e.g., "config.yaml" -> match "*-config.yaml"
	//       "test.yml" -> match "*-test.yml"
	ext := filepath.Ext(configFileName)
	baseName := strings.TrimSuffix(configFileName, ext)
	pattern := "-" + baseName + ext

	// Find all YAML files in the directory that match the pattern
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read config directory: %v", err)
	}

	var yamlFiles []string
	var mainConfigContent []byte

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip hidden files (dotfiles like .goreleaser.yaml)
		if strings.HasPrefix(name, ".") {
			continue
		}

		fileExt := strings.ToLower(filepath.Ext(name))

		// Only process .yaml and .yml files
		if fileExt != ".yaml" && fileExt != ".yml" {
			continue
		}

		fullPath := filepath.Join(configDir, name)

		// Keep track of the main config file to process it last
		if name == configFileName {
			content, err := os.ReadFile(fullPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read main config file: %v", err)
			}
			mainConfigContent = content
		} else if strings.HasSuffix(name, pattern) {
			// Only include files that match the pattern (e.g., *-config.yaml)
			yamlFiles = append(yamlFiles, fullPath)
		}
	}

	// Sort the auxiliary files for consistent ordering
	sort.Strings(yamlFiles)

	// Combine all YAML files as raw text into a single document
	// We do NOT use `---` separators because anchors don't work across document boundaries
	var combined strings.Builder

	// First, add all auxiliary YAML files (these typically contain anchor definitions)
	for _, file := range yamlFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read YAML file %s: %v", file, err)
		}
		combined.Write(content)
		combined.WriteString("\n") // Add newline between files
	}

	// Finally, add the main config file (this typically contains the references)
	if mainConfigContent != nil {
		combined.Write(mainConfigContent)
	}

	return []byte(combined.String()), nil
}
