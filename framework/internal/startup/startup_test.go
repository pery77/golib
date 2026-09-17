package startup

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestImports keeps this package ahead of ffi and raylib-go when Go
// initializes packages: it may import only syscall and unsafe (see
// the package documentation).
func TestImports(t *testing.T) {
	allowed := []string{"syscall", "unsafe"}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range parsed.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(allowed, path) {
				t.Errorf("%s imports %s: golib/internal/startup may import only %q, so that Go initializes it before raylib-go and ffi", file, path, allowed)
			}
		}
	}
}
