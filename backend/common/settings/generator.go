// WARNING: the vast majority of generator.go is ai generated
// troubleshooting is best done with ai agent assistance
package settings

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// CommentsMap[typeName][fieldName] = combined doc+inline comment text
type CommentsMap map[string]map[string]string

// SecretFieldsMap[typeName][fieldName] = true if field should be redacted
type SecretFieldsMap map[string]map[string]bool

// DeprecatedFieldsMap[typeName][fieldName] = true if field is deprecated
type DeprecatedFieldsMap map[string]map[string]bool

// getStringStyle determines whether a string should be quoted in YAML
func getStringStyle(value string) yaml.Style {
	// Always quote all strings for consistency
	return yaml.DoubleQuotedStyle
}

// CollectComments parses all Go source in the directory of srcPath and returns CommentsMap.
func CollectComments(srcPath string) (CommentsMap, error) {
	comments, _, _, err := CollectCommentsAndSecrets(srcPath)
	return comments, err
}

// CollectCommentsAndSecrets parses all Go source and returns comments, secrets, and deprecated field mappings
func CollectCommentsAndSecrets(srcPath string) (CommentsMap, SecretFieldsMap, DeprecatedFieldsMap, error) {
	// Determine directories to parse - include settings and users packages
	dir := srcPath
	if filepath.IsAbs(srcPath) {
		// If it's an absolute path to a file, get the directory
		dir = filepath.Dir(srcPath)
	}

	// List of directories to parse for comments
	dirsToparse := []string{
		dir, // settings package
	}

	// Add users package directory
	// From common/settings, go up to backend root, then into database/users
	var usersDir string
	if filepath.IsAbs(dir) {
		// If absolute path, calculate relative to it
		usersDir = filepath.Join(filepath.Dir(filepath.Dir(dir)), "database/users")
	} else {
		// If relative path like "common/settings" or ".", calculate from current working directory
		usersDir = "database/users"
	}
	if absUsersDir, err := filepath.Abs(usersDir); err == nil {
		dirsToparse = append(dirsToparse, absUsersDir)
	}

	comments := make(CommentsMap)
	secrets := make(SecretFieldsMap)
	deprecated := make(DeprecatedFieldsMap)

	// Parse each directory
	for _, parseDir := range dirsToparse {
		dirComments, dirSecrets, dirDeprecated, err := parseDirectoryComments(parseDir)
		if err != nil {
			// Log error but continue with other directories
			fmt.Printf("Warning: failed to parse directory %s: %v\n", parseDir, err)
			continue
		}

		// Merge results
		for typeName, fieldMap := range dirComments {
			if comments[typeName] == nil {
				comments[typeName] = make(map[string]string)
			}
			for fieldName, comment := range fieldMap {
				comments[typeName][fieldName] = comment
			}
		}

		for typeName, fieldMap := range dirSecrets {
			if secrets[typeName] == nil {
				secrets[typeName] = make(map[string]bool)
			}
			for fieldName, isSecret := range fieldMap {
				secrets[typeName][fieldName] = isSecret
			}
		}

		for typeName, fieldMap := range dirDeprecated {
			if deprecated[typeName] == nil {
				deprecated[typeName] = make(map[string]bool)
			}
			for fieldName, isDeprecated := range fieldMap {
				deprecated[typeName][fieldName] = isDeprecated
			}
		}
	}

	return comments, secrets, deprecated, nil
}

// parseDirectoryComments parses a single directory for comments, secrets, and deprecated fields
func parseDirectoryComments(dir string) (CommentsMap, SecretFieldsMap, DeprecatedFieldsMap, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, nil, err
	}

	comments := make(CommentsMap)
	secrets := make(SecretFieldsMap)
	deprecated := make(DeprecatedFieldsMap)

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.TYPE {
					continue
				}
				for _, spec := range gen.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok {
						continue
					}
					typeName := ts.Name.Name
					commentMap := make(map[string]string)
					secretMap := make(map[string]bool)
					deprecatedMap := make(map[string]bool)
					comments[typeName] = commentMap
					secrets[typeName] = secretMap
					deprecated[typeName] = deprecatedMap

					for _, field := range st.Fields.List {
						if len(field.Names) == 0 {
							continue
						}
						name := field.Names[0].Name
						var parts []string
						var fullComment string

						if field.Doc != nil {
							docText := strings.TrimSpace(field.Doc.Text())
							parts = append(parts, docText)
							fullComment += docText + " "
						}
						if field.Comment != nil {
							commentText := strings.TrimSpace(field.Comment.Text())
							parts = append(parts, commentText)
							fullComment += commentText
						}

						if len(parts) > 0 {
							commentMap[name] = strings.Join(parts, " : ")
						}

						// Check if field should be treated as secret
						if strings.Contains(strings.ToLower(fullComment), "secret:") {
							secretMap[name] = true
						}

						// Check if field should be treated as deprecated
						if strings.Contains(strings.ToLower(fullComment), "deprecated:") {
							deprecatedMap[name] = true
						}
					}
				}
			}
		}
	}
	return comments, secrets, deprecated, nil
}

// PathToTypeField represents a mapping from YAML path to Go type and field info
type PathToTypeField struct {
	TypeName  string
	FieldName string
}

// buildStructPathMap builds a dynamic mapping from YAML paths to Go struct types/fields using reflection
func buildStructPathMap(config *Settings) map[string]PathToTypeField {
	pathMap := make(map[string]PathToTypeField)

	// Start with the root Settings struct
	buildPathMapRecursive(reflect.TypeOf(config).Elem(), reflect.ValueOf(config).Elem(), "", "Settings", pathMap)

	return pathMap
}

// buildPathMapRecursive recursively builds the path mapping using reflection
func buildPathMapRecursive(structType reflect.Type, structValue reflect.Value, currentPath string, typeName string, pathMap map[string]PathToTypeField) {
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		// Skip fields marked to ignore
		if yamlTag := field.Tag.Get("yaml"); yamlTag == "-" {
			continue
		}
		if jsonTag := field.Tag.Get("json"); jsonTag == "-" {
			continue
		}

		// Determine the YAML field name from struct tags
		yamlFieldName := getYamlFieldName(field)

		// Build the full path
		var fullPath string
		if currentPath == "" {
			fullPath = yamlFieldName
		} else {
			fullPath = currentPath + "." + yamlFieldName
		}

		// Add this field to the path map
		pathMap[fullPath] = PathToTypeField{
			TypeName:  typeName,
			FieldName: field.Name,
		}

		// If this field is a struct, recurse into it
		fieldValue := structValue.Field(i)
		fieldType := field.Type

		// Handle pointers
		if fieldType.Kind() == reflect.Ptr {
			if fieldValue.IsNil() {
				// Skip nil pointers
				continue
			}
			fieldValue = fieldValue.Elem()
			fieldType = fieldType.Elem()
		}

		// Recurse into structs (but not slices of structs for now - those are handled differently)
		if fieldType.Kind() == reflect.Struct {
			// Skip certain types that shouldn't be recursed into
			if isSkippableStructType(fieldType) {
				continue
			}

			buildPathMapRecursive(fieldType, fieldValue, fullPath, fieldType.Name(), pathMap)
		} else if fieldType.Kind() == reflect.Slice && fieldType.Elem().Kind() == reflect.Struct {
			// For slices of structs, we map to the element type
			elemType := fieldType.Elem()
			if !isSkippableStructType(elemType) {
				// Create a zero value of the element type to recurse into
				zeroValue := reflect.Zero(elemType)
				buildPathMapRecursive(elemType, zeroValue, fullPath, elemType.Name(), pathMap)
			}
		}
	}
}

// getYamlFieldName determines the YAML field name from struct tags
func getYamlFieldName(field reflect.StructField) string {
	// Check yaml tag first
	if yamlTag := field.Tag.Get("yaml"); yamlTag != "" && yamlTag != "-" {
		return strings.Split(yamlTag, ",")[0]
	}

	// Check json tag second
	if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
		return strings.Split(jsonTag, ",")[0]
	}

	// Default to field name
	return field.Name
}

// isSkippableStructType checks if a struct type should be skipped during recursion
func isSkippableStructType(t reflect.Type) bool {
	// Skip types from other packages that we don't want to recurse into
	pkgPath := t.PkgPath()

	// Skip external packages
	if pkgPath != "" && !strings.Contains(pkgPath, "filebrowser/backend") {
		return true
	}

	return false
}

// CollectCommentsFromEmbeddedYaml parses comments from embedded YAML content and returns CommentsMap, SecretFieldsMap, and DeprecatedFieldsMap
func CollectCommentsFromEmbeddedYaml(yamlContent string) (CommentsMap, SecretFieldsMap, DeprecatedFieldsMap, error) {
	comments := make(CommentsMap)
	secrets := make(SecretFieldsMap)
	deprecated := make(DeprecatedFieldsMap)

	// Build the dynamic path mapping from the actual Config struct
	pathMap := buildStructPathMap(&Config)

	lines := strings.Split(yamlContent, "\n")

	// Track the current path in the YAML structure and array context
	var currentPath []string
	var arrayDepth = -1 // Track the depth where we encountered an array element

	for _, line := range lines {
		// Skip empty lines and lines that are just comments
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Find the key and comment in the line
		if !strings.Contains(line, ":") {
			continue
		}

		// Split on the first colon to get key part
		colonIndex := strings.Index(line, ":")
		keyPart := line[:colonIndex]
		restPart := line[colonIndex+1:]

		// Calculate indentation
		indent := 0
		for i, char := range keyPart {
			if char == ' ' {
				indent++
			} else {
				keyPart = keyPart[i:]
				break
			}
		}
		depth := indent / 2

		// Get the key name
		key := strings.TrimSpace(keyPart)
		if key == "" {
			continue
		}

		// Handle YAML array elements that start with "- "
		isArrayElement := strings.HasPrefix(key, "- ")
		if isArrayElement {
			key = strings.TrimSpace(key[2:]) // Remove "- " prefix
			arrayDepth = depth               // Track that we're in an array element at this depth
		} else {
			// If we're processing a field that's part of an array element,
			// keep it at the array element depth
			if arrayDepth >= 0 && depth > arrayDepth {
				depth = arrayDepth
			} else if arrayDepth >= 0 && depth <= arrayDepth {
				// We've moved out of the array element context
				arrayDepth = -1
			}
		}

		// Adjust the current path based on depth
		if depth < len(currentPath) {
			currentPath = currentPath[:depth]
		}

		// Add current key to path
		if depth < len(currentPath) {
			currentPath[depth] = key
			currentPath = currentPath[:depth+1]
		} else {
			currentPath = append(currentPath, key)
		}

		// Look for comment in the rest of the line
		var comment string
		if strings.Contains(restPart, "#") {
			// Find the comment part (not inside quotes)
			inQuotes := false
			for i, char := range restPart {
				if char == '"' && (i == 0 || restPart[i-1] != '\\') {
					inQuotes = !inQuotes
				} else if char == '#' && !inQuotes {
					comment = strings.TrimSpace(restPart[i+1:])
					break
				}
			}
		}

		// Store comment if it exists
		if comment != "" {
			// Build the full path string
			fullPath := strings.Join(currentPath, ".")

			// Look up the type and field name from our dynamic mapping
			if pathInfo, exists := pathMap[fullPath]; exists {
				typeName := pathInfo.TypeName
				fieldName := pathInfo.FieldName

				// Initialize maps if they don't exist
				if comments[typeName] == nil {
					comments[typeName] = make(map[string]string)
				}
				if secrets[typeName] == nil {
					secrets[typeName] = make(map[string]bool)
				}
				if deprecated[typeName] == nil {
					deprecated[typeName] = make(map[string]bool)
				}

				comments[typeName][fieldName] = comment

				// Check for secret and deprecated markers
				commentLower := strings.ToLower(comment)
				if strings.Contains(commentLower, "secret:") {
					secrets[typeName][fieldName] = true
				}
				if strings.Contains(commentLower, "deprecated:") {
					deprecated[typeName][fieldName] = true
				}
			}
		}
	}

	return comments, secrets, deprecated, nil
}

// parseDefaultsFromEmbeddedYaml creates a default config by parsing the embedded YAML template
func parseDefaultsFromEmbeddedYaml(embeddedYaml string) (*Settings, error) {
	// Start with baseline defaults from setDefaults to ensure all fields are initialized
	defaultConfig := setDefaults(true)
	defaultConfig.Server.Sources = []*Source{{Path: "."}}

	// Parse the embedded YAML to overlay the documented defaults
	if err := yaml.Unmarshal([]byte(embeddedYaml), &defaultConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal embedded YAML into config: %v", err)
	}

	return &defaultConfig, nil
}

// readEmbeddedYaml attempts to read the embedded config.generated.yaml file
func readEmbeddedYaml() (string, error) {
	// Try to read from the relative path that should work when the binary includes embedded files
	embeddedContent, err := os.ReadFile("http/dist/config.generated.yaml")
	if err != nil {
		// Try alternative path
		embeddedContent, err = os.ReadFile("backend/http/dist/config.generated.yaml")
		if err != nil {
			// Try current directory path
			embeddedContent, err = os.ReadFile("dist/config.generated.yaml")
			if err != nil {
				return "", fmt.Errorf("could not read embedded YAML from any path: %v", err)
			}
		}
	}
	return string(embeddedContent), nil
}

// BuildNode constructs a yaml.Node for any Go value, injecting comments on struct fields.
func BuildNode(v reflect.Value, comm CommentsMap) (*yaml.Node, error) {
	return buildNodeWithDefaults(v, comm, reflect.Value{}, SecretFieldsMap{}, DeprecatedFieldsMap{})
}

// buildNodeWithDefaults constructs a yaml.Node for any Go value, skipping fields that match defaults, redacting secrets, and filtering deprecated fields
func buildNodeWithDefaults(v reflect.Value, comm CommentsMap, defaults reflect.Value, secrets SecretFieldsMap, deprecated DeprecatedFieldsMap) (*yaml.Node, error) {
	// Dereference pointers
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
		}
		var defaultsElem reflect.Value
		if defaults.IsValid() && !defaults.IsNil() {
			defaultsElem = defaults.Elem()
		}
		return buildNodeWithDefaults(v.Elem(), comm, defaultsElem, secrets, deprecated)
	}

	switch v.Kind() {
	case reflect.String:
		value := v.String()
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: value,
			Style: getStringStyle(value),
		}, nil
	case reflect.Struct:
		rt := v.Type()
		typeName := rt.Name()
		mapNode := &yaml.Node{Kind: yaml.MappingNode}
		for i := 0; i < rt.NumField(); i++ {
			sf := rt.Field(i)

			// skip unexported or omitted fields
			if sf.PkgPath != "" {
				continue
			}
			if yamlTag := sf.Tag.Get("yaml"); yamlTag == "-" {
				continue
			}
			if jsonTag := sf.Tag.Get("json"); jsonTag == "-" {
				continue
			}

			// Skip deprecated fields if filtering is enabled
			if len(deprecated) > 0 && deprecated[typeName][sf.Name] {
				continue
			}

			currentField := v.Field(i)

			// If we have defaults, compare and skip if values match
			if defaults.IsValid() && i < defaults.NumField() {
				defaultField := defaults.Field(i)
				currentValue := currentField.Interface()
				defaultValue := defaultField.Interface()

				isEqual := reflect.DeepEqual(currentValue, defaultValue)

				if isEqual {
					continue // Skip this field as it matches the default
				}
			}

			// determine key: yaml tag > json tag > field name
			yamlTag := sf.Tag.Get("yaml")
			jsonTag := sf.Tag.Get("json")
			var keyName string
			if yamlTag != "" && yamlTag != "-" {
				keyName = strings.Split(yamlTag, ",")[0]
			} else if jsonTag != "" && jsonTag != "-" {
				keyName = strings.Split(jsonTag, ",")[0]
			} else {
				keyName = sf.Name
			}

			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: keyName}

			// attach validate and comments inline (only when comments are enabled)
			var parts []string

			// Try to find comment - first by current type, then by field's actual type
			var comment string
			if cm := comm[typeName][sf.Name]; cm != "" {
				comment = cm
			} else {
				// Try the field's actual type name (for embedded structs)
				fieldTypeName := sf.Type.Name()
				if fieldTypeName != "" && comm[fieldTypeName][sf.Name] != "" {
					comment = comm[fieldTypeName][sf.Name]
				}
			}

			if comment != "" {
				parts = append(parts, comment)
			}
			// Only add validation tags if comments map is not empty (meaning comments are enabled)
			if len(comm) > 0 {
				if vt := sf.Tag.Get("validate"); vt != "" {
					parts = append(parts, fmt.Sprintf(" validate:%s", vt))
				}
			}

			if len(parts) > 0 {
				keyNode.LineComment = strings.Join(parts, " ")
			}

			// Check if this field should be redacted as a secret
			var valNode *yaml.Node
			var err error
			generateConfig := os.Getenv("FILEBROWSER_GENERATE_CONFIG") == "true"
			if secrets[typeName][sf.Name] && !generateConfig {
				// Check if the secret value is empty
				fieldValue := currentField.Interface()
				if str, ok := fieldValue.(string); ok && str == "" {
					// Show empty string for empty secret values
					valNode = &yaml.Node{
						Kind:  yaml.ScalarNode,
						Tag:   "!!str",
						Value: "",
						Style: yaml.DoubleQuotedStyle,
					}
				} else {
					// Show **hidden** for non-empty secret values
					valNode = &yaml.Node{
						Kind:  yaml.ScalarNode,
						Tag:   "!!str",
						Value: "**hidden**",
						Style: yaml.DoubleQuotedStyle, // Keep secrets quoted for clarity
					}
				}
			} else {
				// Pass through the corresponding default field for recursive comparison
				var defaultField reflect.Value
				if defaults.IsValid() && i < defaults.NumField() {
					defaultField = defaults.Field(i)
				}

				valNode, err = buildNodeWithDefaults(currentField, comm, defaultField, secrets, deprecated)
				if err != nil {
					return nil, err
				}
			}
			mapNode.Content = append(mapNode.Content, keyNode, valNode)
		}
		return mapNode, nil

	case reflect.Slice, reflect.Array:
		// For empty arrays, return a null node so only the key with comment appears
		if v.Len() == 0 {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: ""}, nil
		}

		seq := &yaml.Node{Kind: yaml.SequenceNode}
		// for non-struct slices, render inline [] via flow style
		elemType := v.Type().Elem()
		// Check if element is a struct or pointer to struct
		isStructSlice := elemType.Kind() == reflect.Struct ||
			(elemType.Kind() == reflect.Ptr && elemType.Elem().Kind() == reflect.Struct)
		if !isStructSlice {
			seq.Style = yaml.FlowStyle
		}
		for i := 0; i < v.Len(); i++ {
			var defaultElem reflect.Value
			if defaults.IsValid() && i < defaults.Len() {
				defaultElem = defaults.Index(i)
			}
			n, err := buildNodeWithDefaults(v.Index(i), comm, defaultElem, secrets, deprecated)
			if err != nil {
				return nil, err
			}
			seq.Content = append(seq.Content, n)
		}
		return seq, nil

	case reflect.Map:
		if v.Len() == 0 {
			// Return an empty string node for empty maps, which renders as a blank value.
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: ""}, nil
		}
		mapNode := &yaml.Node{Kind: yaml.MappingNode}

		// Special handling for ImagePreview maps - show all supported types
		if isImagePreviewMap(v) {
			return buildImagePreviewMap(v, comm, secrets, deprecated)
		}

		// Get and sort map keys alphabetically for consistent output
		mapKeys := v.MapKeys()
		keyStrings := make([]string, len(mapKeys))
		keyMap := make(map[string]reflect.Value)
		for i, key := range mapKeys {
			keyStr := fmt.Sprintf("%v", key.Interface())
			keyStrings[i] = keyStr
			keyMap[keyStr] = key
		}

		// Sort alphabetically
		sort.Strings(keyStrings)

		for _, keyStr := range keyStrings {
			key := keyMap[keyStr]

			// Handle key
			keyNode := &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: keyStr,
			}

			// Handle value recursively
			mapVal := v.MapIndex(key)
			var defaultVal reflect.Value
			if defaults.IsValid() {
				defaultVal = defaults.MapIndex(key)
			}

			valNode, err := buildNodeWithDefaults(mapVal, comm, defaultVal, secrets, deprecated)
			if err != nil {
				return nil, err
			}

			mapNode.Content = append(mapNode.Content, keyNode, valNode)
		}
		return mapNode, nil

	default:
		n := &yaml.Node{}
		if err := n.Encode(v.Interface()); err != nil {
			return nil, err
		}
		return n, nil
	}
}

func GenerateYaml() {
	_ = loadConfigWithDefaults("", true)
	Config.Server.Sources = []*Source{
		{
			Path: ".",
			Config: SourceConfig{
				Conditionals: ConditionalFilter{
					ItemRules: []ConditionalIndexConfig{
						{}, // Add empty sample rule to show all fields in generated YAML
					},
				},
			},
		},
	}

	setupLogging()
	setupAuth(true)

	// Save original paths before setupSources modifies them (for YAML generation)
	originalPaths := make(map[*Source]string)
	for _, source := range Config.Server.Sources {
		originalPaths[source] = source.Path
	}

	setupSources(true)
	setupUrls()
	setupMedia()
	setupFrontend(true)

	// Restore original paths so the YAML output has the correct paths, not the placeholder
	for _, source := range Config.Server.Sources {
		if originalPath, ok := originalPaths[source]; ok {
			source.Path = originalPath
		}
	}

	output := "../frontend/public/config.generated.yaml" // "output YAML file"

	// Generate YAML with comments enabled, full config, and deprecated fields filtered
	// Use multi-directory comment parsing to get comments from all packages
	yamlContent, err := GenerateConfigYamlWithSource(&Config, true, true, true, "common/settings")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error generating YAML: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(output, []byte(yamlContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing YAML: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Generated YAML with comments (deprecated fields filtered): %s\n", output)
}

// GenerateConfigYaml generates YAML from a given config with options for comments and filtering
func GenerateConfigYaml(config *Settings, showComments bool, showFull bool, filterDeprecated bool) (string, error) {
	// Try different source paths to handle both runtime and test scenarios
	sourcePaths := []string{
		".",                  // When running tests from settings directory
		"../common/settings", // When running from backend directory
		"common/settings",    // Alternative path
	}

	for _, sourcePath := range sourcePaths {
		yamlOutput, err := GenerateConfigYamlWithSource(config, showComments, showFull, filterDeprecated, sourcePath)
		if err == nil {
			return yamlOutput, nil
		}
		// If it's not a file not found error, return immediately
		if !strings.Contains(err.Error(), "no such file or directory") && !strings.Contains(err.Error(), "cannot find") {
			return "", err
		}
	}

	// If all paths failed, try with empty maps to at least generate basic YAML
	return GenerateConfigYamlWithEmptyMaps(config, showFull)
}

// GenerateConfigYamlWithEmbedded generates YAML from a given config using embedded YAML content as comment source
func GenerateConfigYamlWithEmbedded(config *Settings, showComments bool, showFull bool, filterDeprecated bool, embeddedYaml string) (string, error) {
	var comm CommentsMap
	var secrets SecretFieldsMap
	var deprecated DeprecatedFieldsMap
	var err error

	if embeddedYaml != "" {
		// Always parse the embedded YAML to get secrets and deprecated fields
		comm, secrets, deprecated, err = CollectCommentsFromEmbeddedYaml(embeddedYaml)
		if err != nil {
			return "", fmt.Errorf("error parsing embedded YAML comments: %w", err)
		}

		// If not showing comments, clear the comments map but keep secrets and deprecated
		if !showComments {
			comm = make(CommentsMap)
		}
	} else {
		// Create empty maps only if no embedded YAML is available
		comm = make(CommentsMap)
		secrets = make(SecretFieldsMap)
		deprecated = make(DeprecatedFieldsMap)
	}

	// If not filtering deprecated fields, clear the deprecated map
	if !filterDeprecated {
		deprecated = make(DeprecatedFieldsMap)
	}

	var node *yaml.Node

	if showFull {
		// Show the full current config
		node, err = buildNodeWithDefaults(reflect.ValueOf(config), comm, reflect.Value{}, secrets, deprecated)
	} else {
		// Show only non-default values by comparing with defaults from embedded YAML
		defaultConfig, parseErr := parseDefaultsFromEmbeddedYaml(embeddedYaml)
		if parseErr != nil {
			return "", fmt.Errorf("failed to parse defaults from embedded YAML: %v", parseErr)
		}

		node, err = buildNodeWithDefaults(reflect.ValueOf(config), comm, reflect.ValueOf(defaultConfig), secrets, deprecated)
	}

	if err != nil {
		return "", err
	}

	// Create document
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	doc.Content = []*yaml.Node{node}

	// Encode to YAML
	var rawBuf bytes.Buffer
	enc := yaml.NewEncoder(&rawBuf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return "", err
	}

	// Apply comment alignment if comments are enabled
	yamlOutput := rawBuf.String()
	if showComments {
		yamlOutput = AlignComments(yamlOutput)
	}

	return yamlOutput, nil
}

// identifySecretFieldsByReflection identifies secret fields by known field names
func identifySecretFieldsByReflection(v reflect.Value, typeName string, secrets SecretFieldsMap) {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	if secrets[typeName] == nil {
		secrets[typeName] = make(map[string]bool)
	}

	// Known secret field names
	secretFields := map[string]bool{
		"Key":           true,
		"AdminUsername": true,
		"AdminPassword": true,
		"TotpSecret":    true,
		"ClientID":      true,
		"ClientSecret":  true,
		"Secret":        true,
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if secretFields[field.Name] {
			secrets[typeName][field.Name] = true
		}

		// Recursively check nested structs
		fieldValue := v.Field(i)
		if fieldValue.Kind() == reflect.Struct || (fieldValue.Kind() == reflect.Ptr && !fieldValue.IsNil() && fieldValue.Elem().Kind() == reflect.Struct) {
			nestedTypeName := field.Type.Name()
			if field.Type.Kind() == reflect.Ptr {
				nestedTypeName = field.Type.Elem().Name()
			}
			identifySecretFieldsByReflection(fieldValue, nestedTypeName, secrets)
		}
	}
}

// identifyDeprecatedFieldsByReflection identifies deprecated fields by known field names
func identifyDeprecatedFieldsByReflection(v reflect.Value, typeName string, deprecated DeprecatedFieldsMap) {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	if deprecated[typeName] == nil {
		deprecated[typeName] = make(map[string]bool)
	}

	// Known deprecated field names
	deprecatedFields := map[string]bool{
		"IndexAlbumArt":           true,
		"DisableOfficePreviewExt": true,
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if deprecatedFields[field.Name] {
			deprecated[typeName][field.Name] = true
		}

		// Recursively check nested structs
		fieldValue := v.Field(i)
		if fieldValue.Kind() == reflect.Struct || (fieldValue.Kind() == reflect.Ptr && !fieldValue.IsNil() && fieldValue.Elem().Kind() == reflect.Struct) {
			nestedTypeName := field.Type.Name()
			if field.Type.Kind() == reflect.Ptr {
				nestedTypeName = field.Type.Elem().Name()
			}
			identifyDeprecatedFieldsByReflection(fieldValue, nestedTypeName, deprecated)
		}
	}
}

// GenerateConfigYamlWithEmptyMaps generates YAML without comment parsing when source files are unavailable
func GenerateConfigYamlWithEmptyMaps(config *Settings, showFull bool) (string, error) {
	// Create empty maps
	comm := make(CommentsMap)
	secrets := make(SecretFieldsMap)
	deprecated := make(DeprecatedFieldsMap)

	// Identify secret fields by reflection since we can't parse source files
	identifySecretFieldsByReflection(reflect.ValueOf(config), "Settings", secrets)

	var node *yaml.Node
	var err error

	if showFull {
		// Show the full current config
		node, err = buildNodeWithDefaults(reflect.ValueOf(config), comm, reflect.Value{}, secrets, deprecated)
	} else {
		// Show only non-default values by comparing with defaults during node building
		// Try to use embedded YAML defaults for consistency, fallback to setDefaults
		var defaultConfig *Settings
		embeddedYaml, readErr := readEmbeddedYaml()
		if readErr == nil {
			defaultConfig, err = parseDefaultsFromEmbeddedYaml(embeddedYaml)
		}
		if err != nil || readErr != nil {
			// Fallback to setDefaults
			defaultConfigValue := setDefaults(true)
			defaultConfigValue.Server.Sources = []*Source{{Path: "."}}
			defaultConfig = &defaultConfigValue
		}

		node, err = buildNodeWithDefaults(reflect.ValueOf(config), comm, reflect.ValueOf(defaultConfig), secrets, deprecated)
	}

	if err != nil {
		return "", err
	}

	// Convert to YAML
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	doc.Content = []*yaml.Node{node}

	var rawBuf bytes.Buffer
	enc := yaml.NewEncoder(&rawBuf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return "", err
	}

	return AlignComments(rawBuf.String()), nil
}

// GenerateConfigYamlWithSource generates YAML from a given config with options for comments and filtering, using a custom source path
func GenerateConfigYamlWithSource(config *Settings, showComments bool, showFull bool, filterDeprecated bool, sourcePath string) (string, error) {
	var comm CommentsMap
	var secrets SecretFieldsMap
	var deprecated DeprecatedFieldsMap
	var err error

	if showComments {
		// Collect comments, secrets, and deprecated fields from the settings source files
		comm, secrets, deprecated, err = CollectCommentsAndSecrets(sourcePath)
		if err != nil {
			return "", err
		}
	} else {
		// Still need to collect secrets and deprecated fields even if not showing comments
		_, secrets, deprecated, err = CollectCommentsAndSecrets(sourcePath)
		if err != nil {
			return "", err
		}
		// Create empty comments map
		comm = make(CommentsMap)
	}

	// If secrets map is empty (directory parsing failed), use reflection to identify secrets
	secretsEmpty := true
	for typeName := range secrets {
		if len(secrets[typeName]) > 0 {
			secretsEmpty = false
			break
		}
	}
	if secretsEmpty {
		identifySecretFieldsByReflection(reflect.ValueOf(config), "Settings", secrets)
	}

	// If deprecated map is empty (directory parsing failed), use reflection to identify deprecated fields
	deprecatedEmpty := true
	for typeName := range deprecated {
		if len(deprecated[typeName]) > 0 {
			deprecatedEmpty = false
			break
		}
	}
	if deprecatedEmpty {
		identifyDeprecatedFieldsByReflection(reflect.ValueOf(config), "Settings", deprecated)
	}

	// If not filtering deprecated fields, clear the deprecated map
	if !filterDeprecated {
		deprecated = make(DeprecatedFieldsMap)
	}

	var node *yaml.Node

	if showFull {
		// Show the full current config
		node, err = buildNodeWithDefaults(reflect.ValueOf(config), comm, reflect.Value{}, secrets, deprecated)
	} else {
		// Show only non-default values by comparing with defaults during node building
		// Try to use embedded YAML defaults for consistency, fallback to setDefaults
		var defaultConfig *Settings
		embeddedYaml, readErr := readEmbeddedYaml()
		if readErr == nil {
			defaultConfig, err = parseDefaultsFromEmbeddedYaml(embeddedYaml)
		}
		if err != nil || readErr != nil {
			// Fallback to setDefaults
			defaultConfigValue := setDefaults(true)
			defaultConfigValue.Server.Sources = []*Source{{Path: "."}}
			defaultConfig = &defaultConfigValue
		}

		node, err = buildNodeWithDefaults(reflect.ValueOf(config), comm, reflect.ValueOf(defaultConfig), secrets, deprecated)
	}

	if err != nil {
		return "", err
	}

	// Create document
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	doc.Content = []*yaml.Node{node}

	// Encode to YAML
	var rawBuf bytes.Buffer
	enc := yaml.NewEncoder(&rawBuf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return "", err
	}

	// Apply comment alignment if comments are enabled
	yamlOutput := rawBuf.String()
	if showComments {
		yamlOutput = AlignComments(yamlOutput)
	}

	return yamlOutput, nil
}

// formatLine applies padding to a single line so its comment starts at a target column.
func formatLine(line string) string {
	const targetCol = 42
	const defaultPadding = 1

	// Find the first # character that is NOT inside double or single quotes.
	inSingle, inDouble := false, false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch c {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				// found a comment
				contentPart := line[:i]
				commentPart := line[i:]
				trimmedContent := strings.TrimRight(contentPart, " ")
				if len(trimmedContent) >= targetCol {
					return trimmedContent + strings.Repeat(" ", defaultPadding) + commentPart
				}
				paddingNeeded := targetCol - len(trimmedContent)
				return trimmedContent + strings.Repeat(" ", paddingNeeded) + commentPart
			}
		}
	}
	// No comment found
	return line
}

// AlignComments formats each line of the YAML string independently.
func AlignComments(input string) string {
	lines := strings.Split(input, "\n")
	outputLines := make([]string, len(lines))

	for i, line := range lines {
		outputLines[i] = formatLine(line)
	}

	return strings.Join(outputLines, "\n")
}

// isImagePreviewMap checks if the given map is an ImagePreview map
func isImagePreviewMap(v reflect.Value) bool {
	if v.Kind() != reflect.Map {
		return false
	}

	// Check if the map key type is ImagePreviewType
	keyType := v.Type().Key()
	return keyType.Name() == "ImagePreviewType"
}

// buildImagePreviewMap builds a YAML node for ImagePreview maps, showing all supported types
func buildImagePreviewMap(v reflect.Value, comm CommentsMap, secrets SecretFieldsMap, deprecated DeprecatedFieldsMap) (*yaml.Node, error) {
	mapNode := &yaml.Node{Kind: yaml.MappingNode}

	// Get all supported image preview types from the global slice
	for _, imageType := range AllImagePreviewTypes {
		typeName := string(imageType)

		// Create key node
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: typeName,
		}

		// Get the value from the map, defaulting to false if not present
		var value bool
		if v.Len() > 0 {
			// Look for the key in the map
			for _, mapKey := range v.MapKeys() {
				if mapKey.String() == typeName {
					value = v.MapIndex(mapKey).Bool()
					break
				}
			}
		}

		// Create value node
		valNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!bool",
			Value: fmt.Sprintf("%t", value),
		}

		mapNode.Content = append(mapNode.Content, keyNode, valNode)
	}

	return mapNode, nil
}
