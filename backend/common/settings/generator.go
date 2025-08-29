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

// commentsMap[typeName][fieldName] = combined doc+inline comment text
type commentsMap map[string]map[string]string

// collectComments parses all Go source in the directory of srcPath and returns commentsMap.
func collectComments(srcPath string) (commentsMap, error) {
	// parse entire package so we capture comments on all types
	dir := filepath.Dir(srcPath)
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	out := make(commentsMap)
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
					m := make(map[string]string)
					out[ts.Name.Name] = m
					for _, field := range st.Fields.List {
						if len(field.Names) == 0 {
							continue
						}
						name := field.Names[0].Name
						var parts []string
						if field.Doc != nil {
							parts = append(parts, strings.TrimSpace(field.Doc.Text()))
						}
						if field.Comment != nil {
							parts = append(parts, strings.TrimSpace(field.Comment.Text()))
						}
						if len(parts) > 0 {
							m[name] = strings.Join(parts, " : ")
						}
					}
				}
			}
		}
	}
	return out, nil
}

// buildNode constructs a yaml.Node for any Go value, injecting comments on struct fields.
func buildNode(v reflect.Value, comm commentsMap) (*yaml.Node, error) {
	// Dereference pointers
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
		}
		return buildNode(v.Elem(), comm)
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

			// attach validate and comments inline
			var parts []string
			if cm := comm[typeName][sf.Name]; cm != "" {
				parts = append(parts, cm)
			}
			if vt := sf.Tag.Get("validate"); vt != "" {
				parts = append(parts, fmt.Sprintf(" validate:%s", vt))
			}

			if len(parts) > 0 {
				keyNode.LineComment = strings.Join(parts, " ")
			}

			valNode, err := buildNode(v.Field(i), comm)
			if err != nil {
				return nil, err
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
			n, err := buildNode(zero, comm)
			if err != nil {
				return nil, err
			}
			seq.Content = append(seq.Content, n)
			return seq, nil
		}
		for i := 0; i < v.Len(); i++ {
			n, err := buildNode(v.Index(i), comm)
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

	comm, err := collectComments(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing comments: %v\n", err)
		os.Exit(1)
	}

	node, err := buildNode(reflect.ValueOf(Config), comm)
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

	aligned := alignComments(rawBuf.String())

	if err := os.WriteFile(output, []byte(aligned), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing YAML: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Generated YAML with comments: %s\n", output)
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

// alignComments formats each line of the YAML string independently.
func alignComments(input string) string {
	lines := strings.Split(input, "\n")
	outputLines := make([]string, len(lines))

	for i, line := range lines {
		outputLines[i] = formatLine(line)
	}

	return strings.Join(outputLines, "\n")
}
