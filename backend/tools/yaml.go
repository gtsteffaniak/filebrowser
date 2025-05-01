// cmd/yamlgen/main.go
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
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
					ts := spec.(*ast.TypeSpec)
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
			if vt := sf.Tag.Get("validate"); vt != "" {
				parts = append(parts, fmt.Sprintf("validate:%s", vt))
			}
			if cm := comm[typeName][sf.Name]; cm != "" {
				parts = append(parts, fmt.Sprintf("comments:\"%s\"", cm))
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

func main() {
	input := flag.String("input", "settings/settings.go", "path to Go source file or directory containing structs")
	output := flag.String("output", "settings/config.generated.yaml", "path to write generated YAML")
	flag.Parse()

	comm, err := collectComments(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing comments: %v\n", err)
		os.Exit(1)
	}

	cfg := &settings.Settings{}
	node, err := buildNode(reflect.ValueOf(cfg), comm)
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

	if err := os.WriteFile(*output, []byte(aligned), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing YAML: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Generated YAML with comments: %s\n", *output)
}

// alignComments scans YAML text and aligns all inline '#' within each indentation block
func alignComments(input string) string {
	lines := strings.Split(input, "\n")
	var out []string
	blockStart := 0
	maxPos := 0
	for i, line := range lines {
		trim := strings.TrimLeft(line, " ")
		// new block when indentation decreases
		if i > 0 {
			prevIndent := len(lines[i-1]) - len(strings.TrimLeft(lines[i-1], " "))
			curIndent := len(line) - len(trim)
			if curIndent < prevIndent {
				for j := blockStart; j < i; j++ {
					out = append(out, padLine(lines[j], maxPos))
				}
				blockStart = i
				maxPos = 0
			}
		}
		if idx := strings.Index(line, "#"); idx >= 0 {
			if idx > maxPos {
				maxPos = idx
			}
		}
		if i == len(lines)-1 {
			for j := blockStart; j <= i; j++ {
				out = append(out, padLine(lines[j], maxPos))
			}
		}
	}
	return strings.Join(out, "\n")
}

// padLine inserts spaces before '#' so it lands at column maxPos
func padLine(line string, maxPos int) string {
	if idx := strings.Index(line, "#"); idx >= 0 && idx < maxPos {
		return line[:idx] + strings.Repeat(" ", maxPos-idx) + line[idx:]
	}
	return line
}
