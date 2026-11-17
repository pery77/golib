package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeSDK returns the files of a Steamworks SDK, by their path in its zip.
func fakeSDK(version string) map[string]string {
	return map[string]string{
		"sdk/Readme.txt":                                     "Welcome to the Steamworks SDK.\n\n-----\nv" + version + " 11th March 2026\n-----\n\nv1.63 earlier\n",
		"sdk/public/steam/steam_api.h":                       "the headers stay out",
		"sdk/redistributable_bin/win64/steam_api64.dll":      "steam_api64.dll " + version,
		"sdk/redistributable_bin/win64/steam_api64.lib":      "an import library",
		"sdk/redistributable_bin/linux64/libsteam_api.so":    "linux64 " + version,
		"sdk/redistributable_bin/linuxarm64/libsteam_api.so": "linuxarm64 " + version,
		"sdk/redistributable_bin/linux32/libsteam_api.so":    "linux32, which GoLib doesn't build for",
		"sdk/redistributable_bin/osx/libsteam_api.dylib":     "osx " + version,
		"sdk/redistributable_bin/steam_api.dll":              "32-bit Windows",
	}
}

// A zip or a folder of the SDK gives .tools/steamworks/ its libraries and
// version, and nothing else of it; a newer SDK replaces them.
func TestSteamSDK(t *testing.T) {
	tp := newTestProject(t, "darwin")
	zipPath := filepath.Join(t.TempDir(), "steamworks_sdk_164.zip")
	writeFile(t, zipPath, zipOf(t, fakeSDK("1.64")))
	if code := tp.c.steamSDK([]string{zipPath}); code != 0 {
		t.Fatalf("exit code %d, output:\n%s%s", code, tp.stdout.String(), tp.stderr.String())
	}
	want := map[string]string{
		"win64/steam_api64.dll":      "steam_api64.dll 1.64",
		"linux64/libsteam_api.so":    "linux64 1.64",
		"linuxarm64/libsteam_api.so": "linuxarm64 1.64",
		"osx/libsteam_api.dylib":     "osx 1.64",
		"VERSION":                    "1.64\n",
	}
	checkSteamworks(t, tp, want)
	for _, line := range []string{
		"[ok]   copied the Steamworks API 1.64 from the SDK into .tools/steamworks/: win64/steam_api64.dll, linux64/libsteam_api.so, linuxarm64/libsteam_api.so and osx/libsteam_api.dylib\n",
		"[ok]   builds made here with --tags steam carry osx/libsteam_api.dylib\n",
	} {
		if !strings.Contains(tp.stdout.String(), line) {
			t.Errorf("output doesn't say %q:\n%s", line, tp.stdout.String())
		}
	}

	// A newer SDK, from its sdk folder, takes the place of the old one.
	sdk := filepath.Join(t.TempDir(), "sdk")
	for name, content := range fakeSDK("1.65") {
		writeFile(t, filepath.Join(sdk, filepath.FromSlash(strings.TrimPrefix(name, "sdk/"))), content)
	}
	os.Remove(filepath.Join(sdk, "redistributable_bin", "linuxarm64", "libsteam_api.so"))
	tp.stdout.Reset()
	if code := tp.c.steamSDK([]string{sdk}); code != 0 {
		t.Fatalf("from the folder: exit code %d, output:\n%s", code, tp.stdout.String())
	}
	checkSteamworks(t, tp, map[string]string{
		"win64/steam_api64.dll":   "steam_api64.dll 1.65",
		"linux64/libsteam_api.so": "linux64 1.65",
		"osx/libsteam_api.dylib":  "osx 1.65",
		"VERSION":                 "1.65\n",
	})
	if !strings.Contains(tp.stdout.String(), "[warn] the SDK has no redistributable_bin/linuxarm64/libsteam_api.so") {
		t.Errorf("no warning for the missing library:\n%s", tp.stdout.String())
	}
	// The folder that holds sdk/ will do too.
	tp.stdout.Reset()
	if code := tp.c.steamSDK([]string{filepath.Dir(sdk)}); code != 0 {
		t.Fatalf("from the folder above: exit code %d, output:\n%s", code, tp.stdout.String())
	}

	// Without Readme.txt, the zip's name gives the version.
	files := fakeSDK("1.66")
	delete(files, "sdk/Readme.txt")
	zipPath = filepath.Join(t.TempDir(), "steamworks_sdk_166.zip")
	writeFile(t, zipPath, zipOf(t, files))
	tp.stdout.Reset()
	if code := tp.c.steamSDK([]string{zipPath}); code != 0 || tp.c.steamVersion() != "1.66" {
		t.Errorf("version from the name: exit code %d, version %q, output:\n%s", code, tp.c.steamVersion(), tp.stdout.String())
	}
}

// checkSteamworks checks that .tools/steamworks/ holds want and nothing else.
func checkSteamworks(t *testing.T, tp *testProject, want map[string]string) {
	t.Helper()
	got := map[string]string{}
	filepath.WalkDir(tp.c.steamDir(), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		relative, _ := filepath.Rel(tp.c.steamDir(), path)
		got[filepath.ToSlash(relative)] = string(data)
		return nil
	})
	if len(got) != len(want) {
		t.Errorf(".tools/steamworks/ holds %q, want %q", got, want)
	}
	for name, content := range want {
		if got[name] != content {
			t.Errorf(".tools/steamworks/%s holds %q, want %q", name, got[name], content)
		}
	}
}

func TestSteamSDKFailures(t *testing.T) {
	tp := newTestProject(t, "linux")
	notSDK := filepath.Join(t.TempDir(), "photos.zip")
	writeFile(t, notSDK, zipOf(t, map[string]string{"cat.png": "a cat"}))
	notZip := filepath.Join(t.TempDir(), "notes.txt")
	writeFile(t, notZip, "notes")
	for _, tt := range []struct{ path, want string }{
		{notSDK, "holds no redistributable_bin folder"},
		{notZip, "isn't a zip nor a folder"},
		{t.TempDir(), "has no redistributable_bin folder"},
		{filepath.Join(t.TempDir(), "missing.zip"), "no such file"},
	} {
		tp.stdout.Reset()
		tp.c.failures = 0
		if code := tp.c.steamSDK([]string{tt.path}); code != 1 || !strings.Contains(tp.stdout.String(), tt.want) {
			t.Errorf("%s: exit code %d, output:\n%s", tt.path, code, tp.stdout.String())
		}
	}
	if isDir(tp.c.steamDir()) {
		t.Error("a failure left .tools/steamworks/ behind")
	}
	for _, options := range [][]string{nil, {"a", "b"}, {"--all"}} {
		tp.stderr.Reset()
		if code := tp.c.steamSDK(options); code != 2 || !strings.Contains(tp.stderr.String(), "steam-sdk needs the Steamworks SDK") {
			t.Errorf("%q: exit code %d, stderr %q", options, code, tp.stderr.String())
		}
	}
}

// steamProject returns a project with the game rocks, whose game.json gives
// a Steam app ID, and the Steamworks SDK imported.
func steamProject(t *testing.T, goos string) *testProject {
	t.Helper()
	tp := newTestProject(t, goos, "rocks")
	writeFile(t, tp.c.path("games", "rocks", gameInfoFile), `{"title": "Rocks", "version": "1.0.0", "steamAppID": 480}`)
	zipPath := filepath.Join(t.TempDir(), "steamworks_sdk_164.zip")
	writeFile(t, zipPath, zipOf(t, fakeSDK("1.64")))
	if code := tp.c.steamSDK([]string{zipPath}); code != 0 {
		t.Fatalf("steam-sdk: exit code %d, output:\n%s", code, tp.stdout.String())
	}
	tp.stdout.Reset()
	return tp
}

// A debug build with the build tag steam carries the library and
// steam_appid.txt, and every build the app ID.
func TestSteamDebugBuild(t *testing.T) {
	tp := steamProject(t, "windows")
	if code := tp.c.build([]string{"--tags", "steam"}); code != 0 {
		t.Fatalf("exit code %d, output:\n%s", code, tp.stdout.String())
	}
	built := readFolder(t, tp.c.path("build", "rocks"))
	if built["steam_api64.dll"] != "steam_api64.dll 1.64" || built[steamAppIDFile] != "480\n" {
		t.Errorf("build/rocks/ holds %q, want steam_api64.dll and steam_appid.txt", built)
	}
	builds := tp.builds()
	if len(builds) != 1 || !slices.ContainsFunc(builds[0], func(arg string) bool {
		return strings.HasPrefix(arg, "-ldflags=") && strings.Contains(arg, "-X golib.gameVersion=1.0.0 -X golib.steamAppID=480")
	}) {
		t.Errorf("go build got %q, want the app ID", builds)
	}

	// Without the tag, the build carries neither, though it knows the app ID.
	tp = steamProject(t, "linux")
	if code := tp.c.build(nil); code != 0 {
		t.Fatalf("without the tag: exit code %d, output:\n%s", code, tp.stdout.String())
	}
	built = readFolder(t, tp.c.path("build", "rocks"))
	if _, found := built["libsteam_api.so"]; found {
		t.Errorf("a build without the tag holds %q", built)
	}
	if _, found := built[steamAppIDFile]; found {
		t.Errorf("a build without the tag holds %q", built)
	}
	if !slices.ContainsFunc(tp.builds()[0], func(arg string) bool { return strings.Contains(arg, "-X golib.steamAppID=480") }) {
		t.Errorf("go build got %q, want the app ID", tp.builds()[0])
	}
}

// A dist build with the build tag steam carries the library and its notice,
// and no steam_appid.txt.
func TestSteamDist(t *testing.T) {
	tp := steamProject(t, "windows")
	if code := tp.c.dist([]string{"--tags", "steam"}); code != 0 {
		t.Fatalf("exit code %d, output:\n%s", code, tp.stdout.String())
	}
	folder := tp.folder(t, "rocks")
	if folder["steam_api64.dll"] != "steam_api64.dll 1.64" {
		t.Errorf("the dist build holds %q, want steam_api64.dll", folder)
	}
	if _, found := folder[steamAppIDFile]; found {
		t.Error("the dist build carries steam_appid.txt")
	}
	for _, want := range []string{
		"\nSteamworks API, from the Steamworks SDK 1.64\nhttps://partner.steamgames.com\nsteam_api64.dll, next to rocks.exe: Valve's Steamworks API, for Steam\n",
		"Steamworks SDK Access Agreement",
	} {
		if !strings.Contains(folder[noticesFile], want) {
			t.Errorf("%s doesn't contain %q:\n%s", noticesFile, want, folder[noticesFile])
		}
	}
	if !strings.Contains(tp.stdout.String(), "[ok]   put the Steamworks API in the game, steam_api64.dll, next to rocks.exe, for Steam\n") {
		t.Errorf("output:\n%s", tp.stdout.String())
	}
	if !slices.ContainsFunc(tp.builds()[0], func(arg string) bool { return strings.Contains(arg, "-X golib.steamAppID=480") }) {
		t.Errorf("go build got %q, want the app ID", tp.builds()[0])
	}

	// Without the tag, a dist build has nothing of Steam's but the app ID.
	tp = steamProject(t, "windows")
	if code := tp.c.dist(nil); code != 0 {
		t.Fatalf("without the tag: exit code %d, output:\n%s", code, tp.stdout.String())
	}
	if folder := tp.folder(t, "rocks"); folder["steam_api64.dll"] != "" || strings.Contains(folder[noticesFile], "Steamworks") {
		t.Errorf("a dist build without the tag got Steam: %q", folder)
	}
}

// On macOS, the library goes in the app's Frameworks folder, signed before
// the app.
func TestSteamInTheMacOSApp(t *testing.T) {
	tp := steamProject(t, "darwin")
	if code := tp.c.dist([]string{"--tags", "steam"}); code != 0 {
		t.Fatalf("exit code %d, output:\n%s", code, tp.stdout.String())
	}
	app := tp.c.path("build", "rocks", "dist", "rocks", "Rocks.app")
	frameworks := readFolder(t, filepath.Join(app, "Contents", "Frameworks"))
	if frameworks["libsteam_api.dylib"] != "osx 1.64" || len(frameworks) != 1 {
		t.Errorf("Rocks.app/Contents/Frameworks/ holds %q, want libsteam_api.dylib", frameworks)
	}
	if _, err := os.Stat(filepath.Join(app, "Contents", "MacOS", steamAppIDFile)); err == nil {
		t.Error("the app carries steam_appid.txt")
	}
	want := []string{filepath.Join(app, "Contents", "Frameworks", "libsteam_api.dylib"), app}
	if !slices.Equal(tp.signed, want) {
		t.Errorf("signed %q, want %q", tp.signed, want)
	}
	if notices := tp.folder(t, "rocks")[noticesFile]; !strings.Contains(notices, "Rocks.app/Contents/Frameworks/libsteam_api.dylib: Valve's Steamworks API") {
		t.Errorf("%s doesn't say where the Steamworks API is:\n%s", noticesFile, notices)
	}
}

// The build tag steam needs a Steam app ID and the SDK; a web build takes it
// without either, and has no Steam.
func TestSteamFailures(t *testing.T) {
	tp := steamProject(t, "linux")
	writeFile(t, tp.c.path("games", "rocks", gameInfoFile), `{"title": "Rocks"}`)
	for name, do := range map[string]func() int{
		"build": func() int { return tp.c.build([]string{"--tags", "steam"}) },
		"run":   func() int { return tp.c.run([]string{"--tags", "steam"}) },
		"shot":  func() int { return tp.c.shot([]string{"--tags", "steam"}) },
		"dist":  func() int { return tp.c.dist([]string{"--tags", "demo,steam"}) },
	} {
		tp.stdout.Reset()
		tp.c.failures, tp.calls = 0, nil
		if code := do(); code != 1 || !strings.Contains(tp.stdout.String(), `[fail] the build tag steam needs the game's Steam app ID: add it to games/rocks/game.json, such as "steamAppID": 480`) {
			t.Errorf("%s without an app ID: exit code %d, output:\n%s", name, code, tp.stdout.String())
		}
		if len(tp.builds()) != 0 {
			t.Errorf("%s without an app ID built anyway", name)
		}
	}

	tp = newTestProject(t, "linux", "rocks")
	writeFile(t, tp.c.path("games", "rocks", gameInfoFile), `{"steamAppID": 480}`)
	if code := tp.c.build([]string{"--tags", "steam"}); code != 1 || !strings.Contains(tp.stdout.String(), "[fail] the build tag steam needs the Steamworks API, and .tools/steamworks/ has no linux64/libsteam_api.so: download the Steamworks SDK from partner.steamgames.com and run golib steam-sdk <path to steamworks_sdk_XXX.zip>") {
		t.Errorf("without the SDK: exit code %d, output:\n%s", code, tp.stdout.String())
	}

	// Steam has nothing for Windows on ARM.
	tp = steamProject(t, "windows")
	tp.c.goarch = "arm64"
	if code := tp.c.build([]string{"--tags", "steam"}); code != 1 || !strings.Contains(tp.stdout.String(), "[fail] Steam has no library for windows/arm64") {
		t.Errorf("Windows on ARM: exit code %d, output:\n%s", code, tp.stdout.String())
	}

	for _, content := range []string{`{"steamAppID": 0}`, `{"steamAppID": "480"}`, `{"steamAppID": -1}`} {
		tp := newTestProject(t, "linux", "rocks")
		writeFile(t, tp.c.path("games", "rocks", gameInfoFile), content)
		if code := tp.c.dist(nil); code != 1 || !strings.Contains(tp.stdout.String(), `"steamAppID"`) {
			t.Errorf("%s: exit code %d, output:\n%s", content, code, tp.stdout.String())
		}
	}

	// A web build with the tag needs neither: the browser has no Steam.
	tp = webProject(t, "rocks")
	if code := tp.c.dist([]string{"--web", "--tags", "steam"}); code != 0 {
		t.Errorf("web: exit code %d, output:\n%s", code, tp.stdout.String())
	}
}

// golib test --tags steam gives test binaries the library on the library
// search path.
func TestSteamTests(t *testing.T) {
	tp := steamProject(t, "linux")
	writeFile(t, tp.c.path("framework", "go.mod"), "module golib\n")
	t.Setenv("LD_LIBRARY_PATH", "")
	if code := tp.c.test([]string{"--tags", "steam", "rocks"}); code != 0 {
		t.Fatalf("exit code %d, output:\n%s", code, tp.stdout.String())
	}
	want := "LD_LIBRARY_PATH=" + strings.Join([]string{tp.c.path(".tools", "raylib"), tp.c.path(".tools", "native"), tp.c.path(".tools", "llm"), tp.c.path(".tools", "steamworks", "linux64")}, string(os.PathListSeparator))
	for _, call := range tp.calls {
		if call.args[0] == "test" && !slices.Contains(call.env, want) {
			t.Errorf("go test got the environment %q, want %s", call.env, want)
		}
	}
}
