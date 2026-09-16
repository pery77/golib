package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"debug/pe"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// testProject is a project in a temporary folder, with a fake Go toolchain
// and fake modules. Its go command records its calls instead of running.
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
	writeFile(t, filepath.Join(root, ".tools", "go", "VERSION"), "go1.27.1\ntime 2026-09-01\n")
	writeFile(t, filepath.Join(root, ".tools", "go", "LICENSE"), "Go's license\n")

	// The modules a game is built from, with the files dist reads.
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

	tp := &testProject{modules: []goModule{
		purego, raylib, ffi,
		{Path: "golib", Version: "v0.0.0", Dir: filepath.Join(root, "framework")},
	}}
	p := &project{root: root, goos: goos, goarch: "amd64"}
	p.goRun = func(dir string, args ...string) error {
		_, err := tp.fakeGo(t, dir, args)
		return err
	}
	p.goOutput = func(dir string, args ...string) ([]byte, error) {
		return tp.fakeGo(t, dir, args)
	}
	tp.c = &cli{project: p, stdout: &tp.stdout, stderr: &tp.stderr}
	return tp
}

func (tp *testProject) fakeGo(t *testing.T, dir string, args []string) ([]byte, error) {
	tp.calls = append(tp.calls, goCall{dir: dir, args: args})
	if args[0] == tp.failing {
		return nil, errors.New("exit status 1")
	}
	switch args[0] {
	case "build":
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
		writeFile(t, output, "the game")
	case "list":
		// Standard packages, then the modules' packages, then the game's.
		packages := []goPackage{{ImportPath: "fmt"}}
		for _, m := range tp.modules {
			packages = append(packages, goPackage{ImportPath: m.Path, DepOnly: true, Module: &m, EmbedPatterns: tp.depEmbeds})
		}
		packages = append(packages, goPackage{ImportPath: "github.com/ebitengine/purego/internal/fakecgo", DepOnly: true, Module: &tp.modules[0]})
		packages = append(packages, goPackage{ImportPath: "rocks", Module: &goModule{Path: "rocks", Main: true, Dir: dir}, EmbedPatterns: tp.embeds})
		var output bytes.Buffer
		for _, p := range packages {
			data, err := json.MarshalIndent(p, "", "\t")
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
	files := map[string]string{}
	entries, err := os.ReadDir(tp.c.path("build", game, "dist", game))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(tp.c.path("build", game, "dist", game, entry.Name()))
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

// readZip returns the names and contents of the entries in a zip file.
func readZip(t *testing.T, path string) (names []string, contents map[string]string) {
	t.Helper()
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	contents = map[string]string{}
	for _, file := range archive.File {
		names = append(names, file.Name)
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		contents[file.Name] = string(data)
	}
	return names, contents
}

func TestDistWindows(t *testing.T) {
	tp := newTestProject(t, "windows", "rocks")
	writeFile(t, tp.c.path("games", "rocks", gameInfoFile), `{"title": "Rocks", "version": "1.0.0", "author": "Ada"}`)
	tp.addIcon(t, "rocks")
	writeFile(t, tp.c.path("build", "rocks", "dist", "rocks.exe"), "from an earlier build")
	writeFile(t, tp.c.path("games", "rocks", "assets", attributionFile), "# Music by Ada\n")
	tp.embeds = []string{"all:assets"}

	if code := tp.c.dist(nil); code != 0 {
		t.Fatalf("exit code %d, output:\n%s%s", code, tp.stdout.String(), tp.stderr.String())
	}
	zipFile := tp.c.path("build", "rocks", "dist", "rocks-1.0.0-windows-amd64.zip")
	stat, err := os.Stat(zipFile)
	if err != nil {
		t.Fatal(err)
	}
	want := `[ok]   games/rocks/game.json: the executable's details say "Rocks", version 1.0.0, by Ada
[ok]   games/rocks/icon.png (64 by 64 pixels): the game's icon, in 8 sizes from 16 to 256 pixels
[ok]   built games/rocks into build/rocks/dist/rocks/rocks.exe
[ok]   copied raylib.dll and libffi-8.dll next to it: the game loads them when it starts
[ok]   wrote THIRD-PARTY-LICENSES.txt next to it, with the licenses of Go 1.27.1, github.com/ebitengine/purego v0.10.0, github.com/gen2brain/raylib-go/raylib v0.60.1, github.com/jupiterrider/ffi v0.7.0, raylib 6.0, libffi and assets/ATTRIBUTION.md
[ok]   zipped build/rocks/dist/rocks/ into build/rocks/dist/rocks-1.0.0-windows-amd64.zip (0.0 MB): share this file
[info] players unzip it and start rocks.exe, which needs the files next to it and writes nothing to their machine

dist: 0 failed, 0 warning(s)
`
	if tp.stdout.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", tp.stdout.String(), want)
	}
	if stat.Size() == 0 {
		t.Error("the zip file is empty")
	}

	dir := tp.c.path("games", "rocks")
	const tags = "-tags=golib_dist,raylib_no_embed,ffi_no_embed"
	wantList := []string{"list", "-deps", tags, "-json=ImportPath,DepOnly,Module,EmbedPatterns", "."}
	wantBuild := []string{"build", "-trimpath", tags, "-ldflags=-s -w -H=windowsgui", "-o", tp.c.path("build", "rocks", "dist", "rocks", "rocks.exe"), "."}
	if len(tp.calls) != 2 || !slices.Equal(tp.calls[0].args, wantList) || !slices.Equal(tp.calls[1].args, wantBuild) {
		t.Errorf("go calls = %q, want %q and %q", tp.calls, wantList, wantBuild)
	}
	for _, call := range tp.calls {
		if call.dir != dir {
			t.Errorf("go ran in %s, want the game's folder", call.dir)
		}
	}

	folder := tp.folder(t, "rocks")
	if len(folder) != 4 || folder["rocks.exe"] != "the game" || folder["raylib.dll"] != "raylib for win64_msvc16" || folder["libffi-8.dll"] != "libffi for windows/amd64" {
		t.Errorf("build/rocks/dist/rocks/ holds %q, want the game, raylib.dll, libffi-8.dll and %s", folder, noticesFile)
	}
	if entries, _ := os.ReadDir(tp.c.path("build", "rocks", "dist")); len(entries) != 2 {
		t.Errorf("build/rocks/dist/ holds %v, want the folder and the zip only", entries)
	}

	names, contents := readZip(t, zipFile)
	wantNames := []string{"rocks/", "rocks/" + noticesFile, "rocks/libffi-8.dll", "rocks/raylib.dll", "rocks/rocks.exe"}
	if !slices.Equal(names, wantNames) {
		t.Errorf("the zip holds %q, want %q", names, wantNames)
	}
	for name, content := range folder {
		if contents["rocks/"+name] != content {
			t.Errorf("rocks/%s in the zip differs from the file in the folder", name)
		}
	}

	notices := folder[noticesFile]
	rule := strings.Repeat("=", 78)
	for _, part := range []string{
		"Third-party licenses for Rocks\n\nRocks includes the software and files below, made by others.",
		"\nGo 1.27.1\nhttps://go.dev\nBuilt into rocks.exe: the Go runtime and standard library\n",
		"\n--- LICENSE ---\n\nGo's license\n",
		"\ngithub.com/ebitengine/purego v0.10.0\nhttps://pkg.go.dev/github.com/ebitengine/purego@v0.10.0\nBuilt into rocks.exe\n",
		"\n--- LICENSE ---\n\npurego's license\n",
		"\n--- LICENSE ---\n\n  raylib-go's license, indented\n",
		"\n--- COPYRIGHT.txt ---\n\nffi's copyright\n\n--- LICENSE ---\n\nffi's license\n",
		"\nraylib 6.0\nhttps://www.raylib.com\nraylib.dll, next to rocks.exe, from github.com/gen2brain/raylib-go/raylib v0.60.1\n",
		"\n--- LICENSE ---\n\nraylib's license\n",
		"\nlibffi\nhttps://sourceware.org/libffi/\nlibffi-8.dll, next to rocks.exe, from github.com/jupiterrider/ffi v0.7.0\n",
		"\n--- LICENSE ---\n\nlibffi's license\n",
		"\n" + rule + "\nassets/ATTRIBUTION.md\nBuilt into rocks.exe: files in the game's assets folder that were not made for it\n" + rule + "\n\n--- ATTRIBUTION.md ---\n\n# Music by Ada\n",
	} {
		if !strings.Contains(notices, part) {
			t.Errorf("%s doesn't contain %q:\n%s", noticesFile, part, notices)
		}
	}
	if strings.Contains(notices, "golib") || strings.Contains(notices, "not a license") || strings.Contains(notices, "\r") {
		t.Errorf("%s lists GoLib, a README or carriage returns:\n%s", noticesFile, notices)
	}

	const syso = "golib_dist_windows_amd64.syso"
	data, found := tp.resources[syso]
	if !found || len(tp.resources) != 1 {
		t.Fatalf("the build saw %d .syso files, want only %s", len(tp.resources), syso)
	}
	if _, err := os.Stat(filepath.Join(dir, syso)); err == nil {
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
[ok]   built games/rocks into build/rocks/dist/rocks/rocks.exe
`
	if !strings.HasPrefix(tp.stdout.String(), want) {
		t.Errorf("output:\n%s\nwant it to start with:\n%s", tp.stdout.String(), want)
	}
	if _, err := os.Stat(tp.c.path("build", "rocks", "dist", "rocks-0.0.0-windows-amd64.zip")); err != nil {
		t.Errorf("zip with version 0.0.0: %v", err)
	}
	if notices := tp.folder(t, "rocks")[noticesFile]; !strings.HasPrefix(notices, "Third-party licenses for rocks\n") {
		t.Errorf("%s starts:\n%s\nwant the folder name as the title", noticesFile, notices)
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

func TestDistWindowsOnARM(t *testing.T) {
	tp := newTestProject(t, "windows", "rocks")
	tp.c.goarch = "arm64"
	if code := tp.c.dist(nil); code != 0 {
		t.Fatalf("exit code %d, output:\n%s%s", code, tp.stdout.String(), tp.stderr.String())
	}
	want := "[ok]   copied raylib.dll next to it: the game loads them when it starts\n" +
		"[warn] github.com/jupiterrider/ffi ships no libffi for windows/arm64, so the game stops as soon as it starts (see docs/roadmap.md)\n"
	if !strings.Contains(tp.stdout.String(), want) {
		t.Errorf("output:\n%s\nwant it to contain:\n%s", tp.stdout.String(), want)
	}
	if folder := tp.folder(t, "rocks"); len(folder) != 3 || folder["raylib.dll"] != "raylib for winarm64_msvc16" {
		t.Errorf("build/rocks/dist/rocks/ holds %q, want the game, the ARM raylib.dll and %s", folder, noticesFile)
	}
	if _, found := tp.resources["golib_dist_windows_arm64.syso"]; !found {
		t.Errorf("the build saw %v, want the arm64 resources", tp.resources)
	}
	if _, err := os.Stat(tp.c.path("build", "rocks", "dist", "rocks-0.0.0-windows-arm64.zip")); err != nil {
		t.Error(err)
	}
}

func TestDistLinux(t *testing.T) {
	tp := newTestProject(t, "linux", "rocks", "snake")
	if code := tp.c.dist([]string{"snake"}); code != 0 {
		t.Fatalf("exit code %d, output:\n%s%s", code, tp.stdout.String(), tp.stderr.String())
	}
	want := `[ok]   built games/snake into build/snake/dist/snake/snake
[ok]   copied libraylib.so.6.0.0 next to it: the game loads them when it starts
[ok]   wrote THIRD-PARTY-LICENSES.txt next to it, with the licenses of Go 1.27.1, github.com/ebitengine/purego v0.10.0, github.com/gen2brain/raylib-go/raylib v0.60.1, github.com/jupiterrider/ffi v0.7.0 and raylib 6.0
[ok]   zipped build/snake/dist/snake/ into build/snake/dist/snake-0.0.0-linux-amd64.zip (0.0 MB): share this file
[info] players unzip it and start snake, which needs the files next to it, and libX11.so.6, libGL.so.1 and libffi.so.8 from their system
`
	if !strings.HasPrefix(tp.stdout.String(), want) {
		t.Errorf("output:\n%s\nwant it to start with:\n%s", tp.stdout.String(), want)
	}
	wantBuild := []string{"build", "-trimpath", "-tags=golib_dist,raylib_no_embed,ffi_no_embed", "-ldflags=-s -w -r $ORIGIN", "-o", tp.c.path("build", "snake", "dist", "snake", "snake"), "."}
	if builds := tp.builds(); len(builds) != 1 || !slices.Equal(builds[0], wantBuild) {
		t.Errorf("go builds = %q, want one: %q", builds, wantBuild)
	}
	if len(tp.resources) != 0 {
		t.Errorf("the build saw .syso files: %v", tp.resources)
	}
	if folder := tp.folder(t, "snake"); len(folder) != 3 || folder["libraylib.so.6.0.0"] != "raylib for linux_amd64" {
		t.Errorf("build/snake/dist/snake/ holds %q, want the game, libraylib.so.6.0.0 and %s", folder, noticesFile)
	}
	if info, err := os.Stat(tp.c.path("build", "snake", "dist", "snake", "libraylib.so.6.0.0")); err != nil || (os.PathSeparator == '/' && info.Mode().Perm() != 0o755) {
		t.Errorf("libraylib.so.6.0.0: %v, %v; want the archive's mode, 755", info, err)
	}

	// game.json and icon.png are checked, and said to be left out.
	tp = newTestProject(t, "linux", "rocks")
	writeFile(t, tp.c.path("games", "rocks", gameInfoFile), `{"title": "Rocks", "version": "2.0.0"}`)
	tp.addIcon(t, "rocks")
	if code := tp.c.dist(nil); code != 0 {
		t.Fatalf("with game.json and icon.png: exit code %d, output:\n%s", code, tp.stdout.String())
	}
	if want := "[info] only Windows builds carry the icon from icon.png and the details from game.json so far\n[ok]   built"; !strings.HasPrefix(tp.stdout.String(), want) {
		t.Errorf("with game.json and icon.png, output:\n%s\nwant it to start with:\n%s", tp.stdout.String(), want)
	}
	if _, err := os.Stat(tp.c.path("build", "rocks", "dist", "rocks-2.0.0-linux-amd64.zip")); err != nil {
		t.Error(err)
	}
}

func TestDistMacOS(t *testing.T) {
	tp := newTestProject(t, "darwin", "rocks")
	if code := tp.c.dist(nil); code != 0 {
		t.Fatalf("exit code %d, output:\n%s%s", code, tp.stdout.String(), tp.stderr.String())
	}
	want := `[ok]   built games/rocks into build/rocks/dist/rocks/rocks
[ok]   wrote THIRD-PARTY-LICENSES.txt next to it, with the licenses of Go 1.27.1, github.com/ebitengine/purego v0.10.0, github.com/gen2brain/raylib-go/raylib v0.60.1, github.com/jupiterrider/ffi v0.7.0, raylib 6.0 and libffi
[ok]   zipped build/rocks/dist/rocks/ into build/rocks/dist/rocks-0.0.0-macos-amd64.zip (0.0 MB): share this file
[info] players unzip it and start rocks. On macOS it carries libraylib.6.0.0.dylib and libffi.8.dylib inside, and copies them into the player's ~/Library/Caches folder when it first starts
`
	if !strings.HasPrefix(tp.stdout.String(), want) {
		t.Errorf("output:\n%s\nwant it to start with:\n%s", tp.stdout.String(), want)
	}
	// The libraries stay embedded: no raylib_no_embed and ffi_no_embed.
	wantBuild := []string{"build", "-trimpath", "-tags=golib_dist", "-ldflags=-s -w", "-o", tp.c.path("build", "rocks", "dist", "rocks", "rocks"), "."}
	if builds := tp.builds(); len(builds) != 1 || !slices.Equal(builds[0], wantBuild) {
		t.Errorf("go builds = %q, want one: %q", builds, wantBuild)
	}
	folder := tp.folder(t, "rocks")
	if len(folder) != 2 {
		t.Errorf("build/rocks/dist/rocks/ holds %q, want the game and %s only", folder, noticesFile)
	}
	for _, part := range []string{
		"\nraylib 6.0\nhttps://www.raylib.com\nBuilt into rocks, from github.com/gen2brain/raylib-go/raylib v0.60.1\n",
		"\nlibffi\nhttps://sourceware.org/libffi/\nBuilt into rocks, from github.com/jupiterrider/ffi v0.7.0\n",
	} {
		if !strings.Contains(folder[noticesFile], part) {
			t.Errorf("%s doesn't contain %q", noticesFile, part)
		}
	}
}

func TestDistModuleWithoutLicense(t *testing.T) {
	tp := newTestProject(t, "windows", "rocks")
	tp.modules = append(tp.modules, goModule{Path: "example.com/unlicensed", Version: "v1.0.0", Dir: t.TempDir()})
	if code := tp.c.dist(nil); code != 0 {
		t.Fatalf("exit code %d, output:\n%s", code, tp.stdout.String())
	}
	want := "[warn] example.com/unlicensed has no license file in its folder: find its license and add its notice to THIRD-PARTY-LICENSES.txt by hand\n"
	if !strings.Contains(tp.stdout.String(), want) || !strings.HasSuffix(tp.stdout.String(), "dist: 0 failed, 1 warning(s)\n") {
		t.Errorf("output:\n%s\nwant a warning:\n%s", tp.stdout.String(), want)
	}
	if notices := tp.folder(t, "rocks")[noticesFile]; !strings.Contains(notices, "\nexample.com/unlicensed v1.0.0\n") {
		t.Errorf("%s doesn't name the module:\n%s", noticesFile, notices)
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
	if len(tp.builds()) > 0 {
		t.Error("go build ran for a game whose assets would be missing")
	}

	for _, patterns := range [][]string{{"assets"}, {"shaders/*.fs", "all:assets"}} {
		tp := newTestProject(t, "windows", "rocks")
		writeFile(t, tp.c.path("games", "rocks", "assets", "music.xm"), "music")
		tp.embeds = patterns
		if code := tp.c.dist(nil); code != 0 || len(tp.builds()) != 1 {
			t.Errorf("embed patterns %q: exit code %d and %d builds, want 0 and 1; output:\n%s", patterns, code, len(tp.builds()), tp.stdout.String())
		}
	}

	// Only the game's own package counts.
	tp = newTestProject(t, "windows", "rocks")
	writeFile(t, tp.c.path("games", "rocks", "assets", "music.xm"), "music")
	tp.depEmbeds = []string{"assets"}
	if code := tp.c.dist(nil); code != 1 {
		t.Errorf("a dependency that embeds nothing: exit code %d, want 1", code)
	}
}

func TestDistFailures(t *testing.T) {
	tests := []struct {
		name, goos, goarch, gameInfo, icon, failing, want string
		noRaylib                                          bool
	}{
		{name: "failing go list", goos: "windows", failing: "list", want: "[fail] could not inspect games/rocks (see the Go errors above)"},
		{name: "bad game.json", goos: "windows", gameInfo: `{"version": "one"}`, want: `[fail] games/rocks/game.json: "version" is "one"`},
		{name: "bad game.json on Linux", goos: "linux", gameInfo: `{"version": "one"}`, want: `[fail] games/rocks/game.json: "version" is "one"`},
		{name: "bad icon.png", goos: "windows", icon: "not a png", want: "[fail] games/rocks/icon.png: not a PNG image"},
		{name: "bad icon.png on macOS", goos: "darwin", icon: "not a png", want: "[fail] games/rocks/icon.png: not a PNG image"},
		{name: "unknown processor", goos: "windows", goarch: "386", want: `[fail] cannot make the Windows resources of games/rocks: GoLib can't make Windows resources for "386" processors`},
		{name: "failing build", goos: "windows", failing: "build", want: "[fail] dist build failed for games/rocks (see the Go errors above)"},
		{name: "failing build on Linux", goos: "linux", failing: "build", want: "[fail] dist build failed for games/rocks (see the Go errors above)"},
		{name: "no raylib library", goos: "linux", noRaylib: true, want: "[fail] cannot find the libraries the game loads: github.com/gen2brain/raylib-go/raylib v0.60.1 has no single libs/raylib-*_linux_amd64.tar.gz (run: golib setup)"},
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
			if tt.noRaylib {
				if err := os.RemoveAll(filepath.Join(tp.modules[1].Dir, "libs")); err != nil {
					t.Fatal(err)
				}
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
			if tt.failing == "" && !tt.noRaylib && len(tp.builds()) > 0 {
				t.Error("go build ran after a failure")
			}
			sysos, _ := filepath.Glob(tp.c.path("games", "rocks", "*.syso"))
			if len(sysos) > 0 {
				t.Errorf("left behind in the game's folder: %v", sysos)
			}
			if zips, _ := filepath.Glob(tp.c.path("build", "rocks", "dist", "*.zip")); len(zips) > 0 {
				t.Errorf("a failed dist build made %v", zips)
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

func TestJoinWords(t *testing.T) {
	for _, tt := range []struct {
		words []string
		want  string
	}{
		{nil, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b"}, "a and b"},
		{[]string{"a", "b", "c"}, "a, b and c"},
	} {
		if got := joinWords(tt.words); got != tt.want {
			t.Errorf("joinWords(%q) = %q, want %q", tt.words, got, tt.want)
		}
	}
}
