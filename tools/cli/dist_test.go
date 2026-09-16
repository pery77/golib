package main

import (
	"bytes"
	"debug/pe"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// testProject is a project in a temporary folder with a fake go command,
// which records its calls instead of running.
type testProject struct {
	c              *cli
	stdout, stderr bytes.Buffer
	calls          []goCall
	listOutput     string // what go list prints
	failing        string // the go subcommand that fails, such as "build"
	// resources holds the .syso files that were next to main.go during the
	// last go build, by name.
	resources map[string][]byte
}

// goCall is one run of the fake go command.
type goCall struct {
	dir  string
	args []string
}

// newTestProject returns a project that builds for goos on amd64 processors,
// with a game folder for each of games.
func newTestProject(t *testing.T, goos string, games ...string) *testProject {
	t.Helper()
	root := t.TempDir()
	for _, game := range games {
		writeFile(t, filepath.Join(root, "games", game, "go.mod"), "module "+game+"\n")
	}
	tp := &testProject{}
	p := &project{root: root, goos: goos, goarch: "amd64"}
	p.goRun = func(dir string, args ...string) error {
		return tp.fakeGo(t, dir, args)
	}
	p.goOutput = func(dir string, args ...string) ([]byte, error) {
		return []byte(tp.listOutput), tp.fakeGo(t, dir, args)
	}
	tp.c = &cli{project: p, stdout: &tp.stdout, stderr: &tp.stderr}
	return tp
}

func (tp *testProject) fakeGo(t *testing.T, dir string, args []string) error {
	tp.calls = append(tp.calls, goCall{dir: dir, args: args})
	if args[0] == tp.failing {
		return errors.New("exit status 1")
	}
	if args[0] == "build" {
		tp.resources = map[string][]byte{}
		sysos, _ := filepath.Glob(filepath.Join(dir, "*.syso"))
		for _, path := range sysos {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			tp.resources[filepath.Base(path)] = data
		}
		output := args[slices.Index(args, "-o")+1]
		writeFile(t, output, "an executable")
	}
	return nil
}

// builds returns the arguments of each go build.
func (tp *testProject) builds() [][]string {
	var builds [][]string
	for _, call := range tp.calls {
		if call.args[0] == "build" {
			builds = append(builds, call.args)
		}
	}
	return builds
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// addIcon gives game a valid icon.png.
func (tp *testProject) addIcon(t *testing.T, game string) {
	t.Helper()
	icon, err := os.ReadFile(writePNG(t, checkerboard(64)))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, tp.c.path("games", game, iconFile), string(icon))
}

func TestDistWindows(t *testing.T) {
	tp := newTestProject(t, "windows", "rocks")
	writeFile(t, tp.c.path("games", "rocks", gameInfoFile), `{"title": "Rocks", "version": "1.0.0", "author": "Ada"}`)
	tp.addIcon(t, "rocks")
	writeFile(t, tp.c.path("build", "rocks", "dist", "old.txt"), "from an earlier build")

	if code := tp.c.dist(nil); code != 0 {
		t.Fatalf("exit code %d, output:\n%s%s", code, tp.stdout.String(), tp.stderr.String())
	}
	want := `[ok]   games/rocks/game.json: the executable's details say "Rocks", version 1.0.0, by Ada
[ok]   games/rocks/icon.png (64 by 64 pixels): the game's icon, in 8 sizes from 16 to 256 pixels
[ok]   built games/rocks into build/rocks/dist/rocks.exe
[info] one file with raylib, libffi and the assets inside; it copies raylib and libffi into the player's %LOCALAPPDATA% folder when it first starts

dist: 0 failed, 0 warning(s)
`
	if tp.stdout.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", tp.stdout.String(), want)
	}

	outDir := tp.c.path("build", "rocks", "dist")
	wantBuild := []string{"build", "-trimpath", "-tags=golib_dist", "-ldflags=-s -w -H=windowsgui", "-o", filepath.Join(outDir, "rocks.exe"), "."}
	if builds := tp.builds(); len(builds) != 1 || !slices.Equal(builds[0], wantBuild) {
		t.Errorf("go builds = %q, want one: %q", builds, wantBuild)
	}
	if tp.calls[0].dir != tp.c.path("games", "rocks") {
		t.Errorf("go ran in %s, want the game's folder", tp.calls[0].dir)
	}
	if entries, _ := os.ReadDir(outDir); len(entries) != 1 || entries[0].Name() != "rocks.exe" {
		t.Errorf("build/rocks/dist/ holds %v, want only the new rocks.exe", entries)
	}

	const syso = "golib_dist_windows_amd64.syso"
	data, found := tp.resources[syso]
	if !found || len(tp.resources) != 1 {
		t.Fatalf("the build saw %d .syso files, want only %s", len(tp.resources), syso)
	}
	if _, err := os.Stat(tp.c.path("games", "rocks", syso)); err == nil {
		t.Errorf("%s is still in the game's folder after the build", syso)
	}
	file, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if file.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		t.Errorf("machine = %#x, want amd64", file.Machine)
	}
	section, err := file.Section(".rsrc").Data()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(readResources(t, section)); got != 1+len(iconSizes)+1 {
		t.Errorf("%d resources, want the version, %d icons and their group", got, len(iconSizes))
	}
}

func TestDistWindowsWithoutGameInfoOrIcon(t *testing.T) {
	tp := newTestProject(t, "windows", "rocks")
	if code := tp.c.dist([]string{"rocks"}); code != 0 {
		t.Fatalf("exit code %d, output:\n%s%s", code, tp.stdout.String(), tp.stderr.String())
	}
	want := `[info] games/rocks has no game.json, so the executable's details say "rocks", version 0.0.0: add one to set the title, version and author (see docs/tooling.md)
[info] games/rocks has no icon.png, so the game shows Windows' default icon: add a square PNG, ideally 256 by 256 pixels
[ok]   built games/rocks into build/rocks/dist/rocks.exe
`
	if !strings.HasPrefix(tp.stdout.String(), want) {
		t.Errorf("output:\n%s\nwant it to start with:\n%s", tp.stdout.String(), want)
	}
	file, err := pe.NewFile(bytes.NewReader(tp.resources["golib_dist_windows_amd64.syso"]))
	if err != nil {
		t.Fatal(err)
	}
	section, err := file.Section(".rsrc").Data()
	if err != nil {
		t.Fatal(err)
	}
	if resources := readResources(t, section); len(resources) != 1 || resources[0].kind != rtVersion {
		t.Errorf("resources = %v, want the version information only", resources)
	}
}

func TestDistLinuxAndMacOS(t *testing.T) {
	for _, tt := range []struct {
		goos, last string
	}{
		{"linux", "[info] one file with raylib and the assets inside; it copies raylib into the player's ~/.cache folder when it first starts. Players need libX11.so.6, libGL.so.1 and libffi.so.8"},
		{"darwin", "[info] one file with raylib, libffi and the assets inside; it copies raylib and libffi into the player's ~/Library/Caches folder when it first starts"},
	} {
		t.Run(tt.goos, func(t *testing.T) {
			tp := newTestProject(t, tt.goos, "rocks", "snake")
			if code := tp.c.dist([]string{"snake"}); code != 0 {
				t.Fatalf("exit code %d, output:\n%s%s", code, tp.stdout.String(), tp.stderr.String())
			}
			want := "[ok]   built games/snake into build/snake/dist/snake\n" + tt.last + "\n"
			if !strings.HasPrefix(tp.stdout.String(), want) {
				t.Errorf("output:\n%s\nwant it to start with:\n%s", tp.stdout.String(), want)
			}
			wantBuild := []string{"build", "-trimpath", "-tags=golib_dist", "-ldflags=-s -w", "-o", tp.c.path("build", "snake", "dist", "snake"), "."}
			if builds := tp.builds(); len(builds) != 1 || !slices.Equal(builds[0], wantBuild) {
				t.Errorf("go builds = %q, want one: %q", builds, wantBuild)
			}
			if len(tp.resources) != 0 {
				t.Errorf("the build saw .syso files: %v", tp.resources)
			}

			// game.json and icon.png are checked, and said to be left out.
			tp = newTestProject(t, tt.goos, "rocks")
			writeFile(t, tp.c.path("games", "rocks", gameInfoFile), `{"title": "Rocks"}`)
			tp.addIcon(t, "rocks")
			if code := tp.c.dist(nil); code != 0 {
				t.Fatalf("with game.json and icon.png: exit code %d, output:\n%s", code, tp.stdout.String())
			}
			if want := "[info] only Windows builds carry the icon from icon.png and the details from game.json so far\n[ok]   built"; !strings.HasPrefix(tp.stdout.String(), want) {
				t.Errorf("with game.json and icon.png, output:\n%s\nwant it to start with:\n%s", tp.stdout.String(), want)
			}
		})
	}
}

func TestDistChecksAssets(t *testing.T) {
	tp := newTestProject(t, "windows", "rocks")
	writeFile(t, tp.c.path("games", "rocks", "assets", "music.xm"), "music")
	if code := tp.c.dist(nil); code != 1 {
		t.Errorf("assets without assets.go: exit code %d, want 1", code)
	}
	if want := "[fail] games/rocks/assets/ would be missing from the dist build: add games/rocks/assets.go, as the golib.EmbedAssets documentation shows\n"; !strings.HasPrefix(tp.stdout.String(), want) {
		t.Errorf("output:\n%s\nwant it to start with:\n%s", tp.stdout.String(), want)
	}
	wantList := []string{"list", "-tags=golib_dist", "-f", "{{range .EmbedPatterns}}{{println .}}{{end}}", "."}
	if len(tp.calls) != 1 || !slices.Equal(tp.calls[0].args, wantList) {
		t.Errorf("go calls = %v, want only %q", tp.calls, wantList)
	}

	for _, patterns := range []string{"assets\n", "shaders/*.fs\nall:assets\n"} {
		tp := newTestProject(t, "windows", "rocks")
		writeFile(t, tp.c.path("games", "rocks", "assets", "music.xm"), "music")
		tp.listOutput = patterns
		if code := tp.c.dist(nil); code != 0 || len(tp.builds()) != 1 {
			t.Errorf("embed patterns %q: exit code %d and %d builds, want 0 and 1; output:\n%s", patterns, code, len(tp.builds()), tp.stdout.String())
		}
	}

	tp = newTestProject(t, "windows", "rocks")
	writeFile(t, tp.c.path("games", "rocks", "assets", "music.xm"), "music")
	tp.failing = "list"
	if code := tp.c.dist(nil); code != 1 || !strings.HasPrefix(tp.stdout.String(), "[fail] could not inspect games/rocks (see the Go errors above)\n") {
		t.Errorf("failing go list: exit code %d, output:\n%s", code, tp.stdout.String())
	}
}

func TestDistFailures(t *testing.T) {
	tests := []struct {
		name, goos, goarch, gameInfo, icon, failing, want string
	}{
		{name: "bad game.json", goos: "windows", gameInfo: `{"version": "one"}`, want: `[fail] games/rocks/game.json: "version" is "one"`},
		{name: "bad game.json on Linux", goos: "linux", gameInfo: `{"version": "one"}`, want: `[fail] games/rocks/game.json: "version" is "one"`},
		{name: "bad icon.png", goos: "windows", icon: "not a png", want: "[fail] games/rocks/icon.png: not a PNG image"},
		{name: "bad icon.png on macOS", goos: "darwin", icon: "not a png", want: "[fail] games/rocks/icon.png: not a PNG image"},
		{name: "unknown processor", goos: "windows", goarch: "386", want: `[fail] cannot make the Windows resources of games/rocks: GoLib can't make Windows resources for "386" processors`},
		{name: "failing build", goos: "windows", failing: "build", want: "[fail] dist build failed for games/rocks (see the Go errors above)"},
		{name: "failing build on Linux", goos: "linux", failing: "build", want: "[fail] dist build failed for games/rocks (see the Go errors above)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tp := newTestProject(t, tt.goos, "rocks")
			if tt.goarch != "" {
				tp.c.goarch = tt.goarch
			}
			if tt.gameInfo != "" {
				writeFile(t, tp.c.path("games", "rocks", gameInfoFile), tt.gameInfo)
			}
			if tt.icon != "" {
				writeFile(t, tp.c.path("games", "rocks", iconFile), tt.icon)
			}
			tp.failing = tt.failing
			if code := tp.c.dist(nil); code != 1 {
				t.Errorf("exit code %d, want 1", code)
			}
			if !strings.Contains(tp.stdout.String(), "\n"+tt.want) && !strings.HasPrefix(tp.stdout.String(), tt.want) {
				t.Errorf("output:\n%s\nwant a line starting %q", tp.stdout.String(), tt.want)
			}
			if !strings.HasSuffix(tp.stdout.String(), "\ndist: 1 failed, 0 warning(s)\n") {
				t.Errorf("output:\n%s\nwant it to end with the summary of one failure", tp.stdout.String())
			}
			if tt.failing == "" && len(tp.builds()) > 0 {
				t.Error("go build ran after a failure")
			}
			sysos, _ := filepath.Glob(tp.c.path("games", "rocks", "*.syso"))
			if len(sysos) > 0 {
				t.Errorf("left behind in the game's folder: %v", sysos)
			}
		})
	}
}

func TestDistUsage(t *testing.T) {
	tests := []struct {
		games   []string
		options []string
		want    string
	}{
		{nil, nil, "golib: there are no games in games/ yet (create one: golib new <name>)\n"},
		{[]string{"rocks", "snake"}, nil, "golib: dist needs a game name (available: rocks, snake)\n"},
		{[]string{"rocks", "snake"}, []string{"pong"}, `golib: no game named "pong" in games/ (available: rocks, snake)` + "\n"},
		{[]string{"rocks"}, []string{"rocks", "snake"}, "golib: dist takes at most one game name (got: rocks snake)\n"},
	}
	for _, tt := range tests {
		tp := newTestProject(t, "windows", tt.games...)
		if code := tp.c.dist(tt.options); code != 2 {
			t.Errorf("games %v, options %q: exit code %d, want 2", tt.games, tt.options, code)
		}
		if want := tt.want + "Run \"golib help\" for usage.\n"; tp.stderr.String() != want || tp.stdout.Len() > 0 {
			t.Errorf("games %v, options %q: printed %q and %q, want %q on stderr only", tt.games, tt.options, tp.stdout.String(), tp.stderr.String(), want)
		}
		if len(tp.calls) > 0 {
			t.Errorf("games %v, options %q: go ran after a usage mistake", tt.games, tt.options)
		}
	}
}
