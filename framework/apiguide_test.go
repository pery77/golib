package golib

import (
	_ "embed"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// apiGuide is README.md, the guide to this package for agents, by task. These
// tests keep it in step with the code. It is embedded rather than read, so
// that go test runs them again whenever it changes.
//
//go:embed README.md
var apiGuide string

func TestAPIGuideListsEveryExport(t *testing.T) {
	for _, name := range exportedNames(t) {
		if !strings.Contains(apiGuide, "`"+name+"`") {
			t.Errorf("framework/README.md doesn't list %s: add `%s` to the section for its task", name, name)
		}
	}
}

func TestAPIGuideNamesExist(t *testing.T) {
	exported := map[string]bool{}
	for _, name := range exportedNames(t) {
		exported[name] = true
	}
	reported := map[string]bool{}
	check := func(name, written string) {
		if !exported[name] && !reported[name] {
			reported[name] = true
			t.Errorf("framework/README.md names %s, which package golib doesn't export: fix the guide or the code", written)
		}
	}
	// golib.Run, golib.KeyEnter and the like, in text and in code.
	for _, match := range regexp.MustCompile(`\bgolib\.([A-Z]\w*)`).FindAllStringSubmatch(apiGuide, -1) {
		check(match[1], match[0])
	}
	// Methods and fields, written as `Input.KeyDown` or `Config.Width`.
	for _, match := range regexp.MustCompile("`([A-Z]\\w*\\.[A-Z]\\w*)`").FindAllStringSubmatch(apiGuide, -1) {
		check(match[1], match[0])
	}
}

// exportedNames returns every exported name the package declares, sorted:
// package-level names as "Run", and the methods and fields of exported types
// as "Input.KeyDown" and "Config.Width".
func exportedNames(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	var names []string
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fileSet, file, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				switch {
				case !decl.Name.IsExported():
				case decl.Recv == nil:
					names = append(names, decl.Name.Name)
				case ast.IsExported(receiverType(decl.Recv)):
					names = append(names, receiverType(decl.Recv)+"."+decl.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						if spec.Name.IsExported() {
							names = append(names, spec.Name.Name)
							names = append(names, memberNames(spec)...)
						}
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							if name.IsExported() {
								names = append(names, name.Name)
							}
						}
					}
				}
			}
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// receiverType returns the name of the type a method belongs to, such as
// "Input" for func (in *Input) KeyDown.
func receiverType(receiver *ast.FieldList) string {
	typ := receiver.List[0].Type
	if pointer, ok := typ.(*ast.StarExpr); ok {
		typ = pointer.X
	}
	if name, ok := typ.(*ast.Ident); ok {
		return name.Name
	}
	return ""
}

// memberNames returns the exported fields of a struct type, or the methods of
// an interface type, as "Type.Name".
func memberNames(spec *ast.TypeSpec) []string {
	var members *ast.FieldList
	switch typ := spec.Type.(type) {
	case *ast.StructType:
		members = typ.Fields
	case *ast.InterfaceType:
		members = typ.Methods
	default:
		return nil
	}
	var names []string
	for _, member := range members.List {
		for _, name := range member.Names {
			if name.IsExported() {
				names = append(names, spec.Name.Name+"."+name.Name)
			}
		}
	}
	return names
}
