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
	// parse entire package so we capture comments on all types
	dir := srcPath
	if filepath.IsAbs(srcPath) {
		// If it's an absolute path to a file, get the directory
		dir = filepath.Dir(srcPath)
	}

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
			if cm := comm[typeName][sf.Name]; cm != "" {
				parts = append(parts, cm)
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
			if secrets[typeName][sf.Name] {
				valNode = &yaml.Node{
					Kind:  yaml.ScalarNode,
					Tag:   "!!str",
					Value: "**hidden**",
					Style: yaml.DoubleQuotedStyle, // Keep secrets quoted for clarity
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
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		// for non-struct slices, render inline [] via flow style
		if v.Type().Elem().Kind() != reflect.Struct {
			seq.Style = yaml.FlowStyle
		}
		// placeholder for empty slice of structs
		if v.Len() == 0 && v.Type().Elem().Kind() == reflect.Struct {
			zero := reflect.Zero(v.Type().Elem())
			var defaultElem reflect.Value
			if defaults.IsValid() && defaults.Len() > 0 {
				defaultElem = defaults.Index(0)
			}
			n, err := buildNodeWithDefaults(zero, comm, defaultElem, secrets, deprecated)
			if err != nil {
				return nil, err
			}
			seq.Content = append(seq.Content, n)
			return seq, nil
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
		mapNode := &yaml.Node{Kind: yaml.MappingNode}

		for _, key := range v.MapKeys() {
			// Handle key
			keyStr := fmt.Sprintf("%v", key.Interface())
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
	Config.Server.Sources = []Source{
		{
			Path: ".",
		},
	}

	setupLogging()
	setupAuth(true)
	setupSources(true)
	setupUrls()
	setupFrontend(true)
	output := "generated.yaml" // "output YAML file"

	// Generate YAML with comments enabled, full config, and deprecated fields filtered
	// Force the source path to be correct for static generation
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
		"common/settings", // When running from backend directory
		".",               // When running tests from settings directory
		"../settings",     // Alternative test path
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

// GenerateConfigYamlWithEmptyMaps generates YAML without comment parsing when source files are unavailable
func GenerateConfigYamlWithEmptyMaps(config *Settings, showFull bool) (string, error) {
	// Create empty maps
	comm := make(CommentsMap)
	secrets := make(SecretFieldsMap)
	deprecated := make(DeprecatedFieldsMap)

	var node *yaml.Node
	var err error

	if showFull {
		// Show the full current config
		node, err = buildNodeWithDefaults(reflect.ValueOf(config), comm, reflect.Value{}, secrets, deprecated)
	} else {
		// Show only non-default values by comparing with defaults during node building
		defaultConfig := setDefaults(true)
		defaultConfig.Server.Sources = []Source{{Path: "."}}

		node, err = buildNodeWithDefaults(reflect.ValueOf(config), comm, reflect.ValueOf(&defaultConfig), secrets, deprecated)
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
		// Create a clean default config (no file loading, just pure defaults)
		defaultConfig := setDefaults(true)
		// Apply same setup as a fresh instance would have
		defaultConfig.Server.Sources = []Source{{Path: "."}}

		node, err = buildNodeWithDefaults(reflect.ValueOf(config), comm, reflect.ValueOf(&defaultConfig), secrets, deprecated)
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
