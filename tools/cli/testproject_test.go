package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// testProject is a project in a temporary folder, with a fake Go toolchain
// and fake modules. Its go command and its games record their runs instead
// of running.
type testProject struct {
	c              *cli
	stdout, stderr bytes.Buffer
	calls          []goCall
	embeds         []string // the embed patterns of the game's main package
	depEmbeds      []string // the embed patterns of the other packages
	modules        []goModule
	failing        string // the go subcommand that fails, such as "build"
	// resources holds the .syso files that were next to main.go during the
	// last go build, by name.
	resources map[string][]byte

	games      []gameRun
	gameCode   int  // the exit code of every game
	gameHangs  bool // games run until they are stopped
	gameFails  bool // games don't start
	shotsTaken int  // how many of the requested screenshots games save; -1 for all
}

// newTestProject returns a project that builds for goos on amd64 processors,
// with a game folder for each of games.
func newTestProject(t *testing.T, goos string, games ...string) *testProject {
	t.Helper()
	root := t.TempDir()
	for _, game := range games {
		writeFile(t, filepath.Join(root, "games", game, "go.mod"), "module "+game+"\n")
	}
	writeFile(t, filepath.Join(root, ".tools", "go", "VERSION"), "go1.27.1\ntime 2026-09-01\n")
	writeFile(t, filepath.Join(root, ".tools", "go", "LICENSE"), "Go's license\n")
	writeFile(t, filepath.Join(root, "framework", jfxrLicenseFile), "jfxr's license\n")

	// The modules a game is built from, with the files golib reads.
	cache := filepath.Join(root, ".tools", "gopath", "pkg", "mod")
	purego := goModule{Path: "github.com/ebitengine/purego", Version: "v0.10.0", Dir: filepath.Join(cache, "purego")}
	writeFile(t, filepath.Join(purego.Dir, "LICENSE"), "purego's license\r\n")
	raylib := goModule{Path: raylibModule, Version: "v0.60.1", Dir: filepath.Join(cache, "raylib")}
	writeFile(t, filepath.Join(raylib.Dir, "LICENSE"), "  raylib-go's license, indented\n\n")
	writeFile(t, filepath.Join(raylib.Dir, "libs", "LICENSE"), "raylib's license\n")
	for suffix, name := range map[string]string{
		"win64_msvc16":    "raylib.dll",
		"winarm64_msvc16": "raylib.dll",
		"linux_amd64":     "libraylib.so.6.0.0",
		"macos":           "libraylib.6.0.0.dylib",
	} {
		writeArchive(t, filepath.Join(raylib.Dir, "libs", "raylib-6.0_"+suffix+".tar.gz"), name, "raylib for "+suffix)
	}
	ffi := goModule{Path: ffiModule, Version: "v0.7.0", Dir: filepath.Join(cache, "ffi")}
	writeFile(t, filepath.Join(ffi.Dir, "LICENSE"), "ffi's license\n")
	writeFile(t, filepath.Join(ffi.Dir, "COPYRIGHT.txt"), "ffi's copyright\n")
	writeFile(t, filepath.Join(ffi.Dir, "README.md"), "not a license\n")
	writeFile(t, filepath.Join(ffi.Dir, "assets", "libffi", "LICENSE"), "libffi's license\n")
	for platform, file := range libffiFiles {
		writeFile(t, filepath.Join(ffi.Dir, filepath.FromSlash(file)), "libffi for "+platform)
	}

	tp := &testProject{shotsTaken: -1, modules: []goModule{
		purego, raylib, ffi,
		{Path: "golib", Version: "v0.0.0", Dir: filepath.Join(root, "framework")},
	}}
	p := &project{root: root, goos: goos, goarch: "amd64"}
	p.runGo = func(call goCall) ([]byte, error) {
		return tp.fakeGo(t, call)
	}
	p.runGame = func(run gameRun) (int, bool, error) {
		return tp.fakeGame(t, run)
	}
	tp.c = &cli{project: p, stdout: &tp.stdout, stderr: &tp.stderr}
	return tp
}

func (tp *testProject) fakeGo(t *testing.T, call goCall) ([]byte, error) {
	tp.calls = append(tp.calls, call)
	args := call.args
	if args[0] == tp.failing {
		return nil, errors.New("exit status 1")
	}
	switch args[0] {
	case "build":
		tp.resources = map[string][]byte{}
		sysos, _ := filepath.Glob(filepath.Join(call.dir, "*.syso"))
		for _, path := range sysos {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			tp.resources[filepath.Base(path)] = data
		}
		output := args[slices.Index(args, "-o")+1]
		writeFile(t, output, "the game")
	case "list":
		var values []any
		if args[1] == "-m" {
			// go list -m -json: the modules named after -json.
			for _, m := range tp.modules {
				if slices.Contains(args, m.Path) {
					values = append(values, m)
				}
			}
		} else {
			// go list -deps: standard packages, then the modules'
			// packages, then the game's.
			values = append(values, goPackage{ImportPath: "fmt"})
			for _, m := range tp.modules {
				values = append(values, goPackage{ImportPath: m.Path, DepOnly: true, Module: &m, EmbedPatterns: tp.depEmbeds})
			}
			values = append(values, goPackage{ImportPath: "github.com/ebitengine/purego/internal/fakecgo", DepOnly: true, Module: &tp.modules[0]})
			values = append(values, goPackage{ImportPath: "rocks", Module: &goModule{Path: "rocks", Main: true, Dir: call.dir}, EmbedPatterns: tp.embeds})
		}
		var output bytes.Buffer
		for _, value := range values {
			data, err := json.MarshalIndent(value, "", "\t")
			if err != nil {
				t.Fatal(err)
			}
			output.Write(data)
			output.WriteByte('\n')
		}
		return output.Bytes(), nil
	}
	return nil, nil
}

func (tp *testProject) fakeGame(t *testing.T, run gameRun) (int, bool, error) {
	tp.games = append(tp.games, run)
	if tp.gameFails {
		return 0, false, errors.New("file not found")
	}
	if tp.gameHangs {
		return 0, true, nil
	}
	// Under golib shot, save the screenshots golib.Run would.
	shots := tp.c.lookupEnv(run.env, "GOLIB_SHOT_DIR")
	if frames := tp.c.lookupEnv(run.env, "GOLIB_SHOT_FRAMES"); shots != "" && frames != "" {
		for i, frame := range strings.Split(frames, ",") {
			if tp.shotsTaken >= 0 && i >= tp.shotsTaken {
				break
			}
			writeFile(t, filepath.Join(shots, "frame-"+strings.Repeat("0", 6-len(frame))+frame+".png"), "a picture")
		}
	}
	return tp.gameCode, false, nil
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

// folder returns the names and contents of the files in the game's dist
// folder.
func (tp *testProject) folder(t *testing.T, game string) map[string]string {
	t.Helper()
	return readFolder(t, tp.c.path("build", game, "dist", game))
}

// readFolder returns the names and contents of the files in dir.
func readFolder(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = string(data)
	}
	return files
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeArchive writes a .tar.gz file that holds one file.
func writeArchive(t *testing.T, path, name, content string) {
	t.Helper()
	var data bytes.Buffer
	zipped := gzip.NewWriter(&data)
	files := tar.NewWriter(zipped)
	if err := files.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := files.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(files.Close(), zipped.Close()); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, data.String())
}
