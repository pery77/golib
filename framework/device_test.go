package golib

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// raylibPath is the module package golib must not reach for directly.
const raylibPath = "github.com/gen2brain/raylib-go/raylib"

// TestPackageGolibGoesThroughTheDevice checks that nothing in package golib
// imports raylib: every call to the machine goes through
// golib/internal/device, so a second backend can be added without touching
// the code above it. See that package's documentation.
func TestPackageGolibGoesThroughTheDevice(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no Go files found: the test must run in the framework folder")
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			// Tests may use raylib to check what the raylib backend did.
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for _, imported := range parsed.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("%s: %v", file, err)
			}
			if path == raylibPath {
				t.Errorf("%s imports %s: call the machine through golib/internal/device instead, adding to its contract when it lacks something", file, raylibPath)
			}
		}
	}
}

// TestDeviceBackendsAreBuiltForOnePlatform checks that every file of the
// device package that names a backend carries a build tag, so exactly one
// backend is built into a game.
func TestDeviceBackendsAreBuiltForOnePlatform(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("internal", "device", "raylib*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no raylib backend files found in internal/device")
	}
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(source), "//go:build !js\n") {
			t.Errorf("%s doesn't start with //go:build !js: a backend file says which platforms it is built for", file)
		}
	}
}
