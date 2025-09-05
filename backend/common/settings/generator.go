package settings

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
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

// CollectComments parses all Go source in the directory of srcPath and returns CommentsMap.
func CollectComments(srcPath string) (CommentsMap, error) {
	comments, _, err := CollectCommentsAndSecrets(srcPath)
	return comments, err
}

// CollectCommentsAndSecrets parses all Go source and returns both comments and secret field mappings
func CollectCommentsAndSecrets(srcPath string) (CommentsMap, SecretFieldsMap, error) {
	// parse entire package so we capture comments on all types
	dir := filepath.Dir(srcPath)
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	comments := make(CommentsMap)
	secrets := make(SecretFieldsMap)
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
					comments[typeName] = commentMap
					secrets[typeName] = secretMap

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
							log.Printf("[DEBUG] Marking field %s.%s as secret", typeName, name)
						}
					}
				}
			}
		}
	}
	return comments, secrets, nil
}

// BuildNode constructs a yaml.Node for any Go value, injecting comments on struct fields.
func BuildNode(v reflect.Value, comm CommentsMap) (*yaml.Node, error) {
	return buildNodeWithDefaults(v, comm, reflect.Value{}, SecretFieldsMap{})
}

// buildNodeWithDefaults constructs a yaml.Node for any Go value, skipping fields that match defaults and redacting secrets
func buildNodeWithDefaults(v reflect.Value, comm CommentsMap, defaults reflect.Value, secrets SecretFieldsMap) (*yaml.Node, error) {
	// Dereference pointers
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
		}
		var defaultsElem reflect.Value
		if defaults.IsValid() && !defaults.IsNil() {
			defaultsElem = defaults.Elem()
		}
		return buildNodeWithDefaults(v.Elem(), comm, defaultsElem, secrets)
	}

	switch v.Kind() {
	case reflect.String:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: v.String(),
			Style: yaml.DoubleQuotedStyle,
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

			currentField := v.Field(i)

			// If we have defaults, compare and skip if values match
			if defaults.IsValid() && i < defaults.NumField() {
				defaultField := defaults.Field(i)
				currentValue := currentField.Interface()
				defaultValue := defaultField.Interface()

				isEqual := reflect.DeepEqual(currentValue, defaultValue)
				log.Printf("[DEBUG] Field %s.%s: current=%+v, default=%+v, equal=%v",
					typeName, sf.Name, currentValue, defaultValue, isEqual)

				if isEqual {
					log.Printf("[DEBUG] Skipping field %s.%s (matches default)", typeName, sf.Name)
					continue // Skip this field as it matches the default
				}
				log.Printf("[DEBUG] Including field %s.%s (differs from default)", typeName, sf.Name)
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
				log.Printf("[DEBUG] Redacting secret field %s.%s", typeName, sf.Name)
				valNode = &yaml.Node{
					Kind:  yaml.ScalarNode,
					Tag:   "!!str",
					Value: "**hidden**",
					Style: yaml.DoubleQuotedStyle,
				}
			} else {
				// Pass through the corresponding default field for recursive comparison
				var defaultField reflect.Value
				if defaults.IsValid() && i < defaults.NumField() {
					defaultField = defaults.Field(i)
				}

				valNode, err = buildNodeWithDefaults(currentField, comm, defaultField, secrets)
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
			n, err := buildNodeWithDefaults(zero, comm, defaultElem, secrets)
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
			n, err := buildNodeWithDefaults(v.Index(i), comm, defaultElem, secrets)
			if err != nil {
				return nil, err
			}
			seq.Content = append(seq.Content, n)
		}
		return seq, nil

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
	input := "common/settings/settings.go" // "path to Go source file or directory containing structs"
	output := "generated.yaml"             // "output YAML file"

	comm, err := CollectComments(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing comments: %v\n", err)
		os.Exit(1)
	}

	node, err := BuildNode(reflect.ValueOf(Config), comm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building YAML node: %v\n", err)
		os.Exit(1)
	}

	doc := &yaml.Node{Kind: yaml.DocumentNode}
	doc.Content = []*yaml.Node{node}

	var rawBuf bytes.Buffer
	enc := yaml.NewEncoder(&rawBuf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		fmt.Fprintf(os.Stderr, "error encoding YAML: %v\n", err)
		os.Exit(1)
	}

	aligned := AlignComments(rawBuf.String())

	if err := os.WriteFile(output, []byte(aligned), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing YAML: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Generated YAML with comments: %s\n", output)
}

// GenerateConfigYaml generates YAML from a given config with options for comments and filtering
func GenerateConfigYaml(config *Settings, showComments bool, showFull bool) (string, error) {
	var comm CommentsMap
	var secrets SecretFieldsMap
	var err error

	if showComments {
		// Collect comments and secrets from the settings source file
		comm, secrets, err = CollectCommentsAndSecrets("common/settings/settings.go")
		if err != nil {
			return "", err
		}
	} else {
		// Still need to collect secrets even if not showing comments
		_, secrets, err = CollectCommentsAndSecrets("common/settings/settings.go")
		if err != nil {
			return "", err
		}
		// Create empty comments map
		comm = make(CommentsMap)
	}

	var node *yaml.Node

	if showFull {
		// Show the full current config
		node, err = buildNodeWithDefaults(reflect.ValueOf(config), comm, reflect.Value{}, secrets)
	} else {
		// Show only non-default values by comparing with defaults during node building
		// Create a clean default config (no file loading, just pure defaults)
		defaultConfig := setDefaults(true)
		// Apply same setup as a fresh instance would have
		defaultConfig.Server.Sources = []Source{{Path: "."}}

		node, err = buildNodeWithDefaults(reflect.ValueOf(config), comm, reflect.ValueOf(&defaultConfig), secrets)
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
