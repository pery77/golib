package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// webProject returns a project with the two files a web build copies next to
// the game: Go's loader and the web backend's JavaScript.
func webProject(t *testing.T, games ...string) *testProject {
	t.Helper()
	tp := newTestProject(t, "windows", games...)
	writeFile(t, tp.c.path(".tools", "go", "lib", "wasm", wasmExecFile), "// Go's own loader\n")
	writeFile(t, tp.c.path("framework", "internal", "device", "web.js"), "// the web backend\n")
	return tp
}

func TestWebBuild(t *testing.T) {
	tp := webProject(t, "rocks")
	writeFile(t, tp.c.path("games", "rocks", gameInfoFile), `{"title": "Rocks & Dust", "version": "2.0.0"}`)
	folder := tp.c.buildWeb("rocks")
	if folder == "" {
		t.Fatalf("no web build, output:\n%s%s", tp.stdout.String(), tp.stderr.String())
	}

	// The game is built for the browser, with its assets inside.
	wasm := tp.c.path("build", "rocks", "web", "rocks.wasm")
	wantBuild := []string{"build", "-trimpath", "-tags=golib_dist", "-o", wasm, "."}
	builds := tp.builds()
	if len(builds) != 1 || !slices.Equal(builds[0], wantBuild) {
		t.Fatalf("go builds = %q, want one: %q", builds, wantBuild)
	}
	var built goCall
	for _, call := range tp.calls {
		if call.args[0] == "build" {
			built = call
		}
	}
	for _, want := range []string{"GOOS=js", "GOARCH=wasm"} {
		if !slices.Contains(built.env, want) {
			t.Errorf("the build ran with %q, want %s among them", built.env, want)
		}
	}

	// The page and the two JavaScript files sit next to it.
	for _, name := range []string{webPageFile, wasmExecFile, webGlueFile, "rocks.wasm"} {
		if _, err := os.Stat(filepath.Join(folder, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	page, err := os.ReadFile(filepath.Join(folder, webPageFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<title>Rocks &amp; Dust</title>", // from game.json, with its & escaped
		`<canvas id="game">`,
		`src="` + webGlueFile + `"`,
		`src="` + wasmExecFile + `"`,
		`fetch('rocks.wasm')`,
	} {
		if !strings.Contains(string(page), want) {
			t.Errorf("%s doesn't hold %q:\n%s", webPageFile, want, page)
		}
	}
	if want := "[ok]   built games/rocks into build/rocks/web/rocks.wasm"; !strings.Contains(tp.stdout.String(), want) {
		t.Errorf("output:\n%s\nwant a line starting %q", tp.stdout.String(), want)
	}
}

// A game with an assets folder needs assets.go: a web build carries its
// assets inside, as a dist build does, and a page has no folder to read.
func TestWebBuildNeedsEmbeddedAssets(t *testing.T) {
	tp := webProject(t, "rocks")
	writeFile(t, tp.c.path("games", "rocks", "assets", "ship.png"), "a picture")
	if folder := tp.c.buildWeb("rocks"); folder != "" {
		t.Fatalf("built %s, want a failure because assets.go is missing", folder)
	}
	if want := "games/rocks/assets/ would be missing from the web build"; !strings.Contains(tp.stdout.String(), want) {
		t.Errorf("output:\n%s\nwant it to hold %q", tp.stdout.String(), want)
	}

	// With assets.go it builds.
	tp = webProject(t, "rocks")
	writeFile(t, tp.c.path("games", "rocks", "assets", "ship.png"), "a picture")
	tp.embeds = []string{"all:assets"}
	if folder := tp.c.buildWeb("rocks"); folder == "" {
		t.Fatalf("no web build with assets.go, output:\n%s%s", tp.stdout.String(), tp.stderr.String())
	}
}

func TestWebBuildFailureIsReported(t *testing.T) {
	tp := webProject(t, "rocks")
	tp.failing = "build"
	if folder := tp.c.buildWeb("rocks"); folder != "" {
		t.Fatalf("built %s, want a failure", folder)
	}
	if want := "web build failed for games/rocks"; !strings.Contains(tp.stdout.String(), want) {
		t.Errorf("output:\n%s\nwant it to hold %q", tp.stdout.String(), want)
	}
}

// A port that isn't a port is a usage mistake, and nothing is built.
func TestWebPortOption(t *testing.T) {
	for _, options := range [][]string{
		{"--port"},
		{"--port", "nine"},
		{"--port", "70000"},
		{"--elsewhere"},
	} {
		tp := webProject(t, "rocks")
		if code := tp.c.web(options); code != 2 {
			t.Errorf("web %q: exit code %d, want 2, output:\n%s%s", options, code, tp.stdout.String(), tp.stderr.String())
		}
		if builds := tp.builds(); len(builds) != 0 {
			t.Errorf("web %q built something: %q", options, builds)
		}
	}
}

// Browsers only run WebAssembly served as application/wasm, and a rebuild has
// to show on the next reload.
func TestWebFilesServeWasm(t *testing.T) {
	folder := t.TempDir()
	writeFile(t, filepath.Join(folder, "rocks.wasm"), "not really WebAssembly")
	writeFile(t, filepath.Join(folder, webPageFile), "<!DOCTYPE html>")
	files := webFiles(folder)

	for _, test := range []struct{ path, wantType string }{
		{"/rocks.wasm", "application/wasm"},
		{"/", "text/html"},
	} {
		recorder := httptest.NewRecorder()
		files.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
		if recorder.Code != http.StatusOK {
			t.Errorf("GET %s: %d, want 200", test.path, recorder.Code)
		}
		if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, test.wantType) {
			t.Errorf("GET %s: Content-Type %q, want %s", test.path, got, test.wantType)
		}
		if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("GET %s: Cache-Control %q, want no-store", test.path, got)
		}
	}
}

func TestFileSize(t *testing.T) {
	for _, test := range []struct {
		bytes int64
		want  string
	}{
		{0, "0 bytes"},
		{512, "512 bytes"},
		{2048, "2 KB"},
		{6 * 1024 * 1024, "6.0 MB"},
	} {
		if got := fileSize(test.bytes); got != test.want {
			t.Errorf("fileSize(%d) = %q, want %q", test.bytes, got, test.want)
		}
	}
}
