package main

import (
	"archive/zip"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// dist --web builds the game to share into build/<game>/dist/web/ and zips
// what is in that folder, so that index.html is at the top of the zip.
func TestDistWeb(t *testing.T) {
	tp := webProject(t, "rocks")
	writeFile(t, tp.c.path("games", "rocks", gameInfoFile), `{"title": "Rocks", "version": "1.2.0"}`)
	if code := tp.c.dist([]string{"--web"}); code != 0 {
		t.Fatalf("exit code %d, output:\n%s%s", code, tp.stdout.String(), tp.stderr.String())
	}
	folder := tp.c.path("build", "rocks", "dist", "web")
	for _, name := range []string{webPageFile, wasmExecFile, webGlueFile, "rocks.wasm", noticesFile} {
		if _, err := os.Stat(filepath.Join(folder, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	// A web build carries no raylib, so its notices are Go's and jfxr's.
	notices, err := os.ReadFile(filepath.Join(folder, noticesFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(notices), "raylib") {
		t.Errorf("%s names raylib, which a web build doesn't carry:\n%s", noticesFile, notices)
	}
	for _, want := range []string{"Go 1.27.1", "jfxr"} {
		if !strings.Contains(string(notices), want) {
			t.Errorf("%s doesn't name %s:\n%s", noticesFile, want, notices)
		}
	}

	archive, err := zip.OpenReader(tp.c.path("build", "rocks", "dist", "rocks-1.2.0-web.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	var names []string
	for _, file := range archive.File {
		names = append(names, file.Name)
	}
	slices.Sort(names)
	want := []string{noticesFile, "golib.js", "index.html", "rocks.wasm", "wasm_exec.js"}
	slices.Sort(want)
	if !slices.Equal(names, want) {
		t.Errorf("the zip holds %q, want %q at its top", names, want)
	}
	if !strings.Contains(tp.stdout.String(), "index.html is at the top of the zip") {
		t.Errorf("output:\n%s\nwant it to say where index.html is", tp.stdout.String())
	}
}

// A game whose assets hold music or sounds no browser decodes is told while
// it builds, not when someone opens the page. It still builds: the rest of
// the game plays. Music with nothing beside it to play instead only goes
// quiet in a browser; a .qoa sound stops the game there.
func TestWebWarnsAboutSoundsBrowsersCannotPlay(t *testing.T) {
	tp := webProject(t, "rocks")
	tp.embeds = []string{"all:assets"}
	writeFile(t, tp.c.path("games", "rocks", "assets", "music", "tune.xm"), "tracker music")
	writeFile(t, tp.c.path("games", "rocks", "assets", "sounds", "hit.qoa"), "a qoa sound")
	writeFile(t, tp.c.path("games", "rocks", "assets", "sounds", "coin.wav"), "a wav sound")
	if folder := tp.c.buildWeb("rocks"); folder == "" {
		t.Fatalf("no web build, output:\n%s%s", tp.stdout.String(), tp.stderr.String())
	}
	out := tp.stdout.String()
	for _, want := range []string{"assets/music/tune.xm", "without that music", "assets/sounds/hit.qoa", "stops with a message"} {
		if !strings.Contains(out, want) {
			t.Errorf("output:\n%s\nwant a warning naming %q", out, want)
		}
	}
	if strings.Contains(out, "coin.wav") {
		t.Errorf("output:\n%s\nwarns about a .wav, which browsers play", out)
	}
	if tp.c.warnings != 2 {
		t.Errorf("%d warnings, want 2: one for the music that goes quiet, one for the sound that stops the game", tp.c.warnings)
	}
	if tp.c.failures != 0 {
		t.Errorf("%d failures, want none: a game with such a file still builds", tp.c.failures)
	}
}

// Music in a format no browser decodes, with a file of the same name beside
// it that every browser does, is not a problem: package golib plays that one
// in a web build, and the build says which file it will play.
func TestWebSaysWhichFileItPlaysInsteadOfTrackerMusic(t *testing.T) {
	tp := webProject(t, "rocks")
	tp.embeds = []string{"all:assets"}
	writeFile(t, tp.c.path("games", "rocks", "assets", "music", "tune.xm"), "tracker music")
	writeFile(t, tp.c.path("games", "rocks", "assets", "music", "tune.mp3"), "the same tune")
	writeFile(t, tp.c.path("games", "rocks", "assets", "music", "tune.ogg"), "the same tune, smaller")
	if folder := tp.c.buildWeb("rocks"); folder == "" {
		t.Fatalf("no web build, output:\n%s%s", tp.stdout.String(), tp.stderr.String())
	}
	out := tp.stdout.String()
	// .ogg comes first in what package golib looks for, so it wins.
	if want := "assets/music/tune.xm, so a web build plays assets/music/tune.ogg instead"; !strings.Contains(out, want) {
		t.Errorf("output:\n%s\nwant a line saying %q", out, want)
	}
	if tp.c.warnings != 0 {
		t.Errorf("%d warnings, want none: the music plays in a browser", tp.c.warnings)
	}
}

// shot --web hands the game the same settings the terminal gives it on the
// desktop, through the page.
func TestShotWebSettings(t *testing.T) {
	got := shotSettings([]int{1, 60, 240}, `Enter@1 Right@30-90`, 3)
	values, err := url.ParseQuery(got)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"GOLIB_SHOT_DIR":    "shots",
		"GOLIB_SHOT_FRAMES": "1,60,240",
		"GOLIB_SHOT_INPUT":  "Enter@1 Right@30-90",
		"GOLIB_SHOT_SCALE":  "3",
	}
	for name, value := range want {
		if values.Get(name) != value {
			t.Errorf("%s = %q, want %q", name, values.Get(name), value)
		}
	}
	// Without an input script or a scale, the game hears nothing about them.
	plain, err := url.ParseQuery(shotSettings([]int{60}, "", 1))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"GOLIB_SHOT_INPUT", "GOLIB_SHOT_SCALE"} {
		if plain.Has(name) {
			t.Errorf("%s is set when it was not asked for", name)
		}
	}
}

// The page posts each screenshot back, and says when the game stopped.
func TestShotHandler(t *testing.T) {
	folder, shots := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(folder, webPageFile), "<!DOCTYPE html>")
	arrived := make(chan string, 4)
	stopped := make(chan string, 1)
	handler := shotHandler(folder, shots, arrived, stopped)

	post := func(path, body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
		return recorder
	}
	if code := post(shotWebPath+"?name=frame-000060.png", "a picture").Code; code != http.StatusOK {
		t.Errorf("posting a screenshot: %d, want 200", code)
	}
	if got, err := os.ReadFile(filepath.Join(shots, "frame-000060.png")); err != nil || string(got) != "a picture" {
		t.Errorf("the screenshot was written as %q, %v", got, err)
	}
	if name := <-arrived; name != "frame-000060.png" {
		t.Errorf("arrived %q, want frame-000060.png", name)
	}

	// A name that would climb out of the folder is cut down to its file.
	post(shotWebPath+"?name=../../escaped.png", "a picture")
	if isFile(filepath.Join(filepath.Dir(shots), "escaped.png")) {
		t.Error("a posted name wrote outside the screenshots folder")
	}
	if code := post(shotWebPath+"?name=notes.txt", "not a picture").Code; code != http.StatusBadRequest {
		t.Errorf("posting something that isn't a screenshot: %d, want 400", code)
	}

	post(shotWebErrorPath, "  the game stopped  ")
	if message := <-stopped; message != "the game stopped" {
		t.Errorf("the game's message came back as %q", message)
	}

	// Everything else is the game itself.
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "DOCTYPE") {
		t.Errorf("GET /: %d, %q", recorder.Code, recorder.Body.String())
	}
}
