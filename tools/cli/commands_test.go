package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestBuild(t *testing.T) {
	tp := newTestProject(t, "windows", "rocks")
	if code := tp.c.build(nil); code != 0 {
		t.Fatalf("exit code %d, output:\n%s%s", code, tp.stdout.String(), tp.stderr.String())
	}
	if want := "[ok]   built games/rocks into build/rocks/rocks.exe\n\nbuild: 0 failed, 0 warning(s)\n"; tp.stdout.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", tp.stdout.String(), want)
	}
	dir := tp.c.path("games", "rocks")
	wantCalls := [][]string{
		{"build", "-o", tp.c.path("build", "rocks", "rocks.exe"), "."},
		{"list", "-m", "-json", raylibModule, ffiModule},
	}
	for i, call := range tp.calls {
		if i >= len(wantCalls) || !slices.Equal(call.args, wantCalls[i]) || call.dir != dir || call.env != nil {
			t.Errorf("go call %d = %+v, want %q in %s", i, call, wantCalls, dir)
		}
	}

	tools := readFolder(t, tp.c.path(".tools", "raylib"))
	wantTools := map[string]string{
		"raylib.dll":   "raylib for win64_msvc16",
		"libffi-8.dll": "libffi for windows/amd64",
		"VERSION":      "v0.60.1\nffi v0.7.0\n",
	}
	if !mapsEqual(tools, wantTools) {
		t.Errorf(".tools/raylib/ holds %q, want %q", tools, wantTools)
	}
	built := readFolder(t, tp.c.path("build", "rocks"))
	if len(built) != 3 || built["raylib.dll"] != wantTools["raylib.dll"] || built["libffi-8.dll"] != wantTools["libffi-8.dll"] || built["rocks.exe"] != "the game" {
		t.Errorf("build/rocks/ holds %q, want the game and both libraries", built)
	}

	// .tools/raylib/ stays as it is while VERSION matches.
	writeFile(t, tp.c.path(".tools", "raylib", "raylib.dll"), "kept")
	if code := tp.c.build([]string{"rocks"}); code != 0 {
		t.Fatalf("second build: exit code %d", code)
	}
	if built := readFolder(t, tp.c.path("build", "rocks")); built["raylib.dll"] != "kept" {
		t.Errorf("second build copied %q, want the library already in .tools/raylib/", built["raylib.dll"])
	}
	// A different raylib-go version replaces it.
	tp.modules[1].Version = "v0.61.0"
	if code := tp.c.build(nil); code != 0 {
		t.Fatalf("third build: exit code %d", code)
	}
	tools = readFolder(t, tp.c.path(".tools", "raylib"))
	if tools["raylib.dll"] != wantTools["raylib.dll"] || tools["VERSION"] != "v0.61.0\nffi v0.7.0\n" {
		t.Errorf("after a raylib-go update, .tools/raylib/ holds %q", tools)
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if other, found := b[key]; !found || other != value {
			return false
		}
	}
	return true
}

func TestBuildLinux(t *testing.T) {
	tp := newTestProject(t, "linux", "rocks")
	if code := tp.c.build(nil); code != 0 {
		t.Fatalf("exit code %d, output:\n%s", code, tp.stdout.String())
	}
	if want := "[ok]   built games/rocks into build/rocks/rocks\n"; !strings.HasPrefix(tp.stdout.String(), want) {
		t.Errorf("output:\n%s\nwant it to start with:\n%s", tp.stdout.String(), want)
	}
	// Linux games load the system's libffi.
	tools := readFolder(t, tp.c.path(".tools", "raylib"))
	if len(tools) != 2 || tools["libraylib.so.6.0.0"] != "raylib for linux_amd64" {
		t.Errorf(".tools/raylib/ holds %q, want libraylib.so.6.0.0 and VERSION", tools)
	}
	if built := readFolder(t, tp.c.path("build", "rocks")); len(built) != 2 || built["libraylib.so.6.0.0"] == "" {
		t.Errorf("build/rocks/ holds %q, want the game and libraylib.so.6.0.0", built)
	}
}

func TestBuildFailures(t *testing.T) {
	tests := []struct {
		name, failing, want string
		change              func(tp *testProject)
	}{
		{name: "failing build", failing: "build", want: "[fail] build failed for games/rocks (see the Go errors above)"},
		{name: "failing go list", failing: "list", want: "[fail] cannot find github.com/gen2brain/raylib-go/raylib and github.com/jupiterrider/ffi for games/rocks (run: golib setup)"},
		{name: "missing module", change: func(tp *testProject) { tp.modules = tp.modules[:2] }, want: "[fail] cannot find github.com/gen2brain/raylib-go/raylib and github.com/jupiterrider/ffi for games/rocks (run: golib setup)"},
		{name: "raylib-go not downloaded", change: func(tp *testProject) { tp.modules[1].Dir = "" }, want: "[fail] github.com/gen2brain/raylib-go/raylib v0.60.1 is not downloaded yet (run: golib setup)"},
		{name: "ffi not downloaded", change: func(tp *testProject) { tp.modules[2].Dir = "" }, want: "[fail] github.com/jupiterrider/ffi v0.7.0 is not downloaded yet (run: golib setup)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tp := newTestProject(t, "windows", "rocks")
			tp.failing = tt.failing
			if tt.change != nil {
				tt.change(tp)
			}
			if code := tp.c.build(nil); code != 1 {
				t.Errorf("exit code %d, want 1", code)
			}
			if want := tt.want + "\n\nbuild: 1 failed, 0 warning(s)\n"; tp.stdout.String() != want {
				t.Errorf("output:\n%s\nwant:\n%s", tp.stdout.String(), want)
			}
		})
	}

	// Without libffi in the ffi module, Linux builds still work.
	tp := newTestProject(t, "linux", "rocks")
	tp.modules[2].Dir = ""
	if code := tp.c.build(nil); code != 0 {
		t.Errorf("Linux build without the ffi module's files: exit code %d, output:\n%s", code, tp.stdout.String())
	}
}

func TestRun(t *testing.T) {
	t.Setenv("GOLIB_SHOT_FRAMES", "5")
	tp := newTestProject(t, "windows", "rocks")
	if code := tp.c.run(nil); code != 0 {
		t.Fatalf("exit code %d, output:\n%s%s", code, tp.stdout.String(), tp.stderr.String())
	}
	want := `[ok]   built games/rocks into build/rocks/rocks.exe
[info] running build/rocks/rocks.exe with games/rocks/ as the working directory

run: rocks exited with code 0
`
	if tp.stdout.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", tp.stdout.String(), want)
	}
	if len(tp.games) != 1 {
		t.Fatalf("%d games ran, want 1", len(tp.games))
	}
	game := tp.games[0]
	if game.exe != tp.c.path("build", "rocks", "rocks.exe") || game.dir != tp.c.path("games", "rocks") || game.timeout != 0 {
		t.Errorf("game run = %+v, want build/rocks/rocks.exe in games/rocks/, with no time limit", game)
	}
	if value := tp.c.lookupEnv(game.env, "GOLIB_SHOT_FRAMES"); value != "" {
		t.Errorf("the game got GOLIB_SHOT_FRAMES=%s from the user's environment", value)
	}

	tp = newTestProject(t, "windows", "rocks")
	tp.gameCode = 3
	if code := tp.c.run([]string{"rocks"}); code != 1 || !strings.HasSuffix(tp.stdout.String(), "\nrun: rocks exited with code 3\n") {
		t.Errorf("a game that exits with code 3: exit code %d, output:\n%s", code, tp.stdout.String())
	}

	tp = newTestProject(t, "windows", "rocks")
	tp.gameFails = true
	if code := tp.c.run(nil); code != 1 || !strings.Contains(tp.stdout.String(), "[fail] cannot start build/rocks/rocks.exe: file not found\n\nrun: 1 failed") {
		t.Errorf("a game that doesn't start: exit code %d, output:\n%s", code, tp.stdout.String())
	}

	tp = newTestProject(t, "windows", "rocks")
	tp.failing = "build"
	if code := tp.c.run(nil); code != 1 || len(tp.games) > 0 || !strings.HasSuffix(tp.stdout.String(), "\nrun: 1 failed, 0 warning(s)\n") {
		t.Errorf("a game that doesn't build: exit code %d, %d runs, output:\n%s", code, len(tp.games), tp.stdout.String())
	}
}

func TestRunLinux(t *testing.T) {
	t.Setenv("LD_LIBRARY_PATH", "/opt/lib")
	tp := newTestProject(t, "linux", "rocks")
	if code := tp.c.run(nil); code != 0 {
		t.Fatalf("exit code %d, output:\n%s", code, tp.stdout.String())
	}
	want := tp.c.path("build", "rocks") + string(os.PathListSeparator) + "/opt/lib"
	if got := tp.c.lookupEnv(tp.games[0].env, "LD_LIBRARY_PATH"); got != want {
		t.Errorf("LD_LIBRARY_PATH = %q, want %q", got, want)
	}
	if !strings.Contains(tp.stdout.String(), "[info] running build/rocks/rocks with games/rocks/ as the working directory\n") {
		t.Errorf("output:\n%s", tp.stdout.String())
	}
}

func TestShot(t *testing.T) {
	t.Setenv("GOLIB_SHOT_INPUT", "Space@1")
	tp := newTestProject(t, "windows", "rocks", "snake")
	writeFile(t, tp.c.path("build", "rocks", "shots", "frame-000001.png"), "an old picture")
	if code := tp.c.shot([]string{"90", "rocks", "30", "--input", "Enter@1 Right@2-9", "030"}); code != 0 {
		t.Fatalf("exit code %d, output:\n%s%s", code, tp.stdout.String(), tp.stderr.String())
	}
	want := `[ok]   built games/rocks into build/rocks/rocks.exe
[info] running rocks for 90 frame(s) in a hidden window, playing Enter@1 Right@2-9
[ok]   frame 30: build/rocks/shots/frame-000030.png
[ok]   frame 90: build/rocks/shots/frame-000090.png

shot: 0 failed, 0 warning(s)
`
	if tp.stdout.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", tp.stdout.String(), want)
	}
	if len(tp.games) != 1 {
		t.Fatalf("%d games ran, want 1", len(tp.games))
	}
	game := tp.games[0]
	shots := tp.c.path("build", "rocks", "shots")
	for name, value := range map[string]string{
		"GOLIB_SHOT_DIR":    shots,
		"GOLIB_SHOT_FRAMES": "30,90",
		"GOLIB_SHOT_INPUT":  "Enter@1 Right@2-9",
	} {
		if got := tp.c.lookupEnv(game.env, name); got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
	if game.timeout != shotTimeout || game.dir != tp.c.path("games", "rocks") {
		t.Errorf("game run = %+v, want a time limit of %v, in games/rocks/", game, shotTimeout)
	}
	if files := readFolder(t, shots); len(files) != 2 {
		t.Errorf("build/rocks/shots/ holds %q, want only the new screenshots", files)
	}

	// The default frame, --input=, and no input from the user's environment.
	tp = newTestProject(t, "linux", "rocks")
	if code := tp.c.shot([]string{"--input="}); code != 0 {
		t.Fatalf("default frame: exit code %d, output:\n%s", code, tp.stdout.String())
	}
	if want := "[info] running rocks for 60 frame(s) in a hidden window\n[ok]   frame 60: build/rocks/shots/frame-000060.png\n"; !strings.Contains(tp.stdout.String(), want) {
		t.Errorf("default frame, output:\n%s\nwant:\n%s", tp.stdout.String(), want)
	}
	env := tp.games[0].env
	if tp.c.lookupEnv(env, "GOLIB_SHOT_FRAMES") != "60" || slices.ContainsFunc(env, func(entry string) bool { return strings.HasPrefix(entry, "GOLIB_SHOT_INPUT=") }) {
		t.Errorf("default frame, environment has %q", env)
	}
	if tp.c.lookupEnv(env, "LD_LIBRARY_PATH") == "" {
		t.Error("a Linux game got no LD_LIBRARY_PATH")
	}
}

func TestShotFailures(t *testing.T) {
	tests := []struct {
		name string
		set  func(tp *testProject)
		want string
	}{
		{"missing screenshot", func(tp *testProject) { tp.shotsTaken = 1 }, "[ok]   frame 1: build/rocks/shots/frame-000001.png\n[fail] frame 2: no screenshot was saved\n"},
		{"failing game", func(tp *testProject) { tp.gameCode = 2 }, "[fail] rocks exited with code 2 (see its output above)\n[ok]   frame 1"},
		{"game that never ends", func(tp *testProject) { tp.gameHangs = true; tp.shotsTaken = 0 }, "[fail] rocks didn't finish within 120 seconds and was stopped (does Update or Draw loop forever?)\n[fail] frame 1: no screenshot was saved\n"},
		{"game that doesn't start", func(tp *testProject) { tp.gameFails = true }, "[fail] cannot start build/rocks/rocks.exe: file not found\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tp := newTestProject(t, "windows", "rocks")
			tt.set(tp)
			if code := tp.c.shot([]string{"2", "1"}); code != 1 {
				t.Errorf("exit code %d, want 1", code)
			}
			if !strings.Contains(tp.stdout.String(), tt.want) {
				t.Errorf("output:\n%s\nwant it to contain:\n%s", tp.stdout.String(), tt.want)
			}
		})
	}
}

func TestShotUsage(t *testing.T) {
	tests := []struct {
		options []string
		want    string
	}{
		{[]string{""}, "shot got an empty argument"},
		{[]string{"--input"}, `shot --input needs input to play, for example: --input "Enter@1 Right@30-90"`},
		{[]string{"-v"}, "unknown option for shot: -v"},
		{[]string{"--frames=3"}, "unknown option for shot: --frames=3"},
		{[]string{"0"}, "frame numbers go from 1 to 999999 (got: 0)"},
		{[]string{"1234567"}, "frame numbers go from 1 to 999999 (got: 1234567)"},
		{[]string{"rocks", "snake"}, "shot takes at most one game name (got: rocks snake)"},
		{[]string{"60"}, "shot needs a game name (available: rocks, snake)"},
	}
	for _, tt := range tests {
		tp := newTestProject(t, "windows", "rocks", "snake")
		if code := tp.c.shot(tt.options); code != 2 {
			t.Errorf("shot %q: exit code %d, want 2", tt.options, code)
		}
		if want := "golib: " + tt.want + "\nRun \"golib help\" for usage.\n"; tp.stderr.String() != want {
			t.Errorf("shot %q: stderr %q, want %q", tt.options, tp.stderr.String(), want)
		}
		if len(tp.calls) > 0 || len(tp.games) > 0 {
			t.Errorf("shot %q: go or the game ran after a usage mistake", tt.options)
		}
	}
}

func TestTest(t *testing.T) {
	tp := newTestProject(t, "windows", "rocks", "snake")
	writeFile(t, tp.c.path("framework", "go.mod"), "module golib\n")
	writeFile(t, tp.c.path("tools", "cli", "go.mod"), "module cli\n")
	t.Setenv("PATH", "user-path")
	if code := tp.c.test(nil); code != 0 {
		t.Fatalf("exit code %d, output:\n%s%s", code, tp.stdout.String(), tp.stderr.String())
	}
	want := `[ok]   framework: vet and tests passed
[ok]   games/rocks: vet and tests passed
[ok]   games/snake: vet and tests passed
[ok]   tools/cli: vet and tests passed

test: 0 failed, 0 warning(s)
`
	if tp.stdout.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", tp.stdout.String(), want)
	}
	libraryPath := "PATH=" + tp.c.path(".tools", "raylib") + string(os.PathListSeparator) + tp.c.path(".tools", "go", "bin") + string(os.PathListSeparator) + "user-path"
	var tests []goCall
	for _, call := range tp.calls {
		if call.args[0] == "test" {
			tests = append(tests, call)
		}
	}
	if len(tests) != 4 {
		t.Fatalf("go test ran %d times, want 4", len(tests))
	}
	for i, module := range []string{"framework", "games/rocks", "games/snake", "tools/cli"} {
		call := tests[i]
		if call.dir != tp.c.path(filepath.FromSlash(module)) || !slices.Equal(call.args, []string{"test", "./..."}) {
			t.Errorf("test %d = %+v, want go test ./... in %s", i, call, module)
		}
		wantEnv := []string{libraryPath}
		if module == "tools/cli" {
			wantEnv = nil
		}
		if !slices.Equal(call.env, wantEnv) {
			t.Errorf("%s: go test got %q, want %q", module, call.env, wantEnv)
		}
	}

	tp = newTestProject(t, "windows", "rocks")
	tp.failing = "vet"
	if code := tp.c.test(nil); code != 1 || !strings.HasPrefix(tp.stdout.String(), "[fail] games/rocks: go vet found problems (see above)\n") {
		t.Errorf("failing vet: exit code %d, output:\n%s", code, tp.stdout.String())
	}
	tp = newTestProject(t, "windows", "rocks")
	tp.failing = "test"
	if code := tp.c.test(nil); code != 1 || !strings.HasPrefix(tp.stdout.String(), "[fail] games/rocks: tests failed (see above)\n") {
		t.Errorf("failing tests: exit code %d, output:\n%s", code, tp.stdout.String())
	}
	tp = newTestProject(t, "windows", "rocks")
	tp.modules = nil
	if code := tp.c.test(nil); code != 1 || !strings.HasPrefix(tp.stdout.String(), "[fail] games/rocks: cannot find ") {
		t.Errorf("no raylib-go: exit code %d, output:\n%s", code, tp.stdout.String())
	}
	tp = newTestProject(t, "windows")
	if code := tp.c.test(nil); code != 0 || tp.stdout.String() != "[warn] no Go modules to test\n\ntest: 0 failed, 1 warning(s)\n" {
		t.Errorf("no modules: exit code %d, output:\n%s", code, tp.stdout.String())
	}
	tp = newTestProject(t, "windows", "rocks")
	if code := tp.c.test([]string{"rocks"}); code != 2 || tp.stderr.String() != "golib: test takes no options (got: rocks)\nRun \"golib help\" for usage.\n" {
		t.Errorf("test rocks: exit code %d, stderr %q", code, tp.stderr.String())
	}
}

// newTemplates gives tp's project a game template and a framework go.sum.
func newTemplates(t *testing.T, tp *testProject) {
	t.Helper()
	template := tp.c.path("tools", "template", "game")
	writeFile(t, filepath.Join(template, "go.mod.tmpl"), "module {{name}}\n\ngo {{go}}\n")
	writeFile(t, filepath.Join(template, "DESIGN.md.tmpl"), "# {{name}}\n\nStarted on {{date}}.\n")
	writeFile(t, filepath.Join(template, "notes.txt"), "not a template")
	writeFile(t, tp.c.path("framework", "go.sum"), "checksums\n")
}

func TestNewGame(t *testing.T) {
	tp := newTestProject(t, "windows", "rocks")
	newTemplates(t, tp)
	if code := tp.c.newGame([]string{"space-rocks_2"}); code != 0 {
		t.Fatalf("exit code %d, output:\n%s%s", code, tp.stdout.String(), tp.stderr.String())
	}
	want := `[ok]   created games/space-rocks_2/ from tools/template/game/
[info] next: golib run space-rocks_2, and describe the game in games/space-rocks_2/DESIGN.md

new: 0 failed, 0 warning(s)
`
	if tp.stdout.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", tp.stdout.String(), want)
	}
	dir := tp.c.path("games", "space-rocks_2")
	files := readFolder(t, dir)
	wantFiles := map[string]string{
		"go.mod":    "module space-rocks_2\n\ngo 1.27.1\n",
		"DESIGN.md": "# space-rocks_2\n\nStarted on " + time.Now().Format("2006-01-02") + ".\n",
		"go.sum":    "checksums\n",
	}
	if !mapsEqual(files, wantFiles) {
		t.Errorf("games/space-rocks_2/ holds %q, want %q", files, wantFiles)
	}
	if len(tp.calls) != 1 || tp.calls[0].dir != dir || !slices.Equal(tp.calls[0].args, []string{"mod", "tidy"}) {
		t.Errorf("go calls = %+v, want go mod tidy in the new folder", tp.calls)
	}

	tp = newTestProject(t, "windows")
	newTemplates(t, tp)
	tp.failing = "mod"
	if code := tp.c.newGame([]string{"rocks"}); code != 1 {
		t.Errorf("failing go mod tidy: exit code %d, want 1", code)
	}
	if want := "[fail] go mod tidy failed for games/rocks (see the Go errors above); the folder was deleted, so new can run again\n"; !strings.HasPrefix(tp.stdout.String(), want) {
		t.Errorf("failing go mod tidy, output:\n%s", tp.stdout.String())
	}
	if _, err := os.Stat(tp.c.path("games", "rocks")); err == nil {
		t.Error("failing go mod tidy left games/rocks/ behind")
	}

	tp = newTestProject(t, "windows")
	if code := tp.c.newGame([]string{"rocks"}); code != 1 || !strings.HasPrefix(tp.stdout.String(), "[fail] cannot create games/rocks/: tools/template/game/ has no *.tmpl files\n") {
		t.Errorf("no templates: exit code %d, output:\n%s", code, tp.stdout.String())
	}
	if _, err := os.Stat(tp.c.path("games", "rocks")); err == nil {
		t.Error("a failed new left games/rocks/ behind")
	}
}

func TestNewGameUsage(t *testing.T) {
	invalid := func(name string) string {
		return `invalid game name "` + name + `": use 1 to 32 lowercase letters, digits, - and _, starting with a letter`
	}
	tests := []struct {
		options []string
		want    string
	}{
		{nil, "new needs one game name, for example: golib new asteroids"},
		{[]string{"a", "b"}, "new needs one game name, for example: golib new asteroids"},
		{[]string{"Rocks"}, invalid("Rocks")},
		{[]string{"1up"}, invalid("1up")},
		{[]string{"space rocks"}, invalid("space rocks")},
		{[]string{""}, invalid("")},
		{[]string{strings.Repeat("a", 33)}, invalid(strings.Repeat("a", 33))},
		{[]string{"golib"}, `the game name "golib" is reserved: pick another one`},
		{[]string{"com1"}, `the game name "com1" is reserved: pick another one`},
		{[]string{"rocks"}, "games/rocks already exists: pick another name, or delete that folder first"},
	}
	for _, tt := range tests {
		tp := newTestProject(t, "windows", "rocks")
		newTemplates(t, tp)
		if code := tp.c.newGame(tt.options); code != 2 {
			t.Errorf("new %q: exit code %d, want 2", tt.options, code)
		}
		if want := "golib: " + tt.want + "\nRun \"golib help\" for usage.\n"; tp.stderr.String() != want {
			t.Errorf("new %q: stderr %q, want %q", tt.options, tp.stderr.String(), want)
		}
	}
	// 32 characters is the longest name.
	tp := newTestProject(t, "windows")
	newTemplates(t, tp)
	if code := tp.c.newGame([]string{strings.Repeat("a", 32)}); code != 0 {
		t.Errorf("a 32-character name: exit code %d, stderr %q", code, tp.stderr.String())
	}
}

func TestLibraryPath(t *testing.T) {
	sep := string(os.PathListSeparator)
	for _, tt := range []struct {
		goos string
		env  []string
		want string
	}{
		{"windows", []string{"Path=a", "OTHER=b"}, "PATH=dir" + sep + "a"},
		{"windows", nil, "PATH=dir"},
		{"linux", []string{"LD_LIBRARY_PATH=a", "LD_LIBRARY_PATH=b"}, "LD_LIBRARY_PATH=dir" + sep + "b"},
		{"linux", []string{"ld_library_path=a"}, "LD_LIBRARY_PATH=dir"},
		{"darwin", []string{"DYLD_LIBRARY_PATH="}, "DYLD_LIBRARY_PATH=dir"},
	} {
		p := &project{goos: tt.goos}
		if got := p.libraryPath("dir", tt.env); got != tt.want {
			t.Errorf("%s, %q: libraryPath = %q, want %q", tt.goos, tt.env, got, tt.want)
		}
	}
}

// TestHelperGame is a stand-in game for TestWaitForGame, which runs the test
// binary itself.
func TestHelperGame(t *testing.T) {
	switch os.Getenv("GOLIB_TEST_GAME") {
	case "exit3":
		os.Exit(3)
	case "hang":
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}

func TestWaitForGame(t *testing.T) {
	game := func(behavior string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=^TestHelperGame$")
		cmd.Env = append(os.Environ(), "GOLIB_TEST_GAME="+behavior)
		return cmd
	}
	if code, timedOut, err := waitForGame(game("exit3"), 0); code != 3 || timedOut || err != nil {
		t.Errorf("a game that exits with code 3: %d, %v, %v", code, timedOut, err)
	}
	start := time.Now()
	if code, timedOut, err := waitForGame(game("hang"), 200*time.Millisecond); code != 0 || !timedOut || err != nil {
		t.Errorf("a game that never ends: %d, %v, %v; want it stopped", code, timedOut, err)
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Errorf("stopping the game took %v", elapsed)
	}
	if _, _, err := waitForGame(exec.Command(filepath.Join(t.TempDir(), "missing.exe")), 0); err == nil {
		t.Error("a missing executable: no error")
	}
}

func TestSetup(t *testing.T) {
	tp := newTestProject(t, "windows", "rocks")
	writeFile(t, tp.c.path("framework", "go.mod"), "module golib\n")
	writeFile(t, tp.c.path("tools", "cli", "go.mod"), "module cli\n")
	if code := tp.c.setup([]string{"--warnings=2"}); code != 0 {
		t.Fatalf("exit code %d, output:\n%s%s", code, tp.stdout.String(), tp.stderr.String())
	}
	want := `[ok]   framework: modules downloaded, raylib library for raylib-go v0.60.1 in .tools/raylib/
[ok]   games/rocks: modules downloaded, raylib library for raylib-go v0.60.1 in .tools/raylib/

setup: 0 failed, 2 warning(s)
Setup complete.
`
	if tp.stdout.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", tp.stdout.String(), want)
	}
	var downloads []string
	for _, call := range tp.calls {
		if slices.Equal(call.args, []string{"mod", "download"}) {
			downloads = append(downloads, tp.c.shown(call.dir))
		}
	}
	if !slices.Equal(downloads, []string{"framework", "games/rocks"}) {
		t.Errorf("go mod download ran in %q, want the framework and the game", downloads)
	}
	if tools := readFolder(t, tp.c.path(".tools", "raylib")); tools["VERSION"] != "v0.60.1\nffi v0.7.0\n" {
		t.Errorf(".tools/raylib/ holds %q", tools)
	}

	tp = newTestProject(t, "windows", "rocks", "snake")
	tp.failing = "mod"
	if code := tp.c.setup(nil); code != 1 {
		t.Errorf("failing downloads: exit code %d, want 1", code)
	}
	want = `[fail] could not download the Go modules for games/rocks (see the errors above)
[fail] could not download the Go modules for games/snake (see the errors above)

setup: 2 failed, 0 warning(s)
Setup incomplete. Fix the [fail] items above, then run setup again.
`
	if tp.stdout.String() != want {
		t.Errorf("failing downloads, output:\n%s\nwant:\n%s", tp.stdout.String(), want)
	}

	tp = newTestProject(t, "windows", "rocks")
	tp.modules[1].Dir = ""
	if code := tp.c.setup(nil); code != 1 || !strings.HasPrefix(tp.stdout.String(), "[fail] games/rocks: github.com/gen2brain/raylib-go/raylib v0.60.1 is not downloaded yet") {
		t.Errorf("raylib-go missing: exit code %d, output:\n%s", code, tp.stdout.String())
	}

	tp = newTestProject(t, "windows")
	if code := tp.c.setup([]string{"--warnings=0"}); code != 0 || !strings.HasPrefix(tp.stdout.String(), "[info] no Go modules yet: nothing more to download\n") {
		t.Errorf("no modules: exit code %d, output:\n%s", code, tp.stdout.String())
	}

	for _, options := range [][]string{{"--all"}, {"--warnings=-1"}, {"--warnings=x"}, {"--warnings"}} {
		tp := newTestProject(t, "windows")
		if code := tp.c.setup(options); code != 2 || !strings.HasPrefix(tp.stderr.String(), "golib: setup takes no options (got: ") {
			t.Errorf("setup %q: exit code %d, stderr %q", options, code, tp.stderr.String())
		}
	}
}
