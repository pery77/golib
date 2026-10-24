package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A game that makes rigid bodies gets Jolt next to it, in each kind of build,
// with its license; a game that doesn't, doesn't.
func TestRigidBodiesBringJolt(t *testing.T) {
	tp := newTestProject(t, "windows", "rocks")
	tp.rigidBodies = true
	writeFile(t, tp.c.nativeFile(), "golibnative")
	if tp.c.buildGame("rocks") == "" {
		t.Fatalf("no build, output:\n%s", tp.stdout.String())
	}
	if built := readFolder(t, tp.c.path("build", "rocks")); built["golibnative.dll"] != "golibnative" {
		t.Errorf("build/rocks/ holds %q, want golibnative.dll", built)
	}

	if code := tp.c.dist(nil); code != 0 {
		t.Fatalf("dist: exit code %d, output:\n%s", code, tp.stdout.String())
	}
	folder := tp.folder(t, "rocks")
	if folder["golibnative.dll"] != "golibnative" {
		t.Errorf("the dist build holds %q, want golibnative.dll", folder)
	}
	for _, want := range []string{"\nJolt Physics\nhttps://github.com/jrouwe/JoltPhysics\ngolibnative.dll, next to rocks.exe: GoLib's native library, for the rigid bodies\n", "Copyright 2021 Jorrit Rouwe"} {
		if !strings.Contains(folder[noticesFile], want) {
			t.Errorf("%s doesn't contain %q", noticesFile, want)
		}
	}
	if !strings.Contains(tp.stdout.String(), "[ok]   put GoLib's native library in the game, golibnative.dll, next to rocks.exe, for its rigid bodies\n") {
		t.Errorf("dist doesn't say it put golibnative in, output:\n%s", tp.stdout.String())
	}

	// Without it, the build says what to do.
	os.Remove(tp.c.nativeFile())
	tp.stdout.Reset()
	if tp.c.buildGame("rocks") != "" || !strings.Contains(tp.stdout.String(), "games/rocks uses rigid bodies, which need GoLib's native library, and .tools/native/ has no golibnative.dll: run golib setup") {
		t.Errorf("a build without golibnative, output:\n%s", tp.stdout.String())
	}

	// A game without rigid bodies has none of it.
	tp = newTestProject(t, "windows", "rocks")
	writeFile(t, tp.c.nativeFile(), "golibnative")
	if code := tp.c.dist(nil); code != 0 {
		t.Fatalf("dist without rigid bodies: exit code %d, output:\n%s", code, tp.stdout.String())
	}
	if folder := tp.folder(t, "rocks"); folder["golibnative.dll"] != "" || strings.Contains(folder[noticesFile], "Jolt") {
		t.Errorf("a game without rigid bodies got Jolt: %q", folder)
	}
}

// On macOS, the native library goes in the app's Frameworks folder, signed
// before the app.
func TestRigidBodiesInTheMacOSApp(t *testing.T) {
	tp := newTestProject(t, "darwin", "rocks")
	tp.rigidBodies = true
	writeFile(t, tp.c.nativeFile(), "golibnative")
	if code := tp.c.dist(nil); code != 0 {
		t.Fatalf("exit code %d, output:\n%s", code, tp.stdout.String())
	}
	app := tp.c.path("build", "rocks", "dist", "rocks", "rocks.app")
	frameworks := readFolder(t, filepath.Join(app, "Contents", "Frameworks"))
	if frameworks["golibnative.dylib"] != "golibnative" || len(frameworks) != 1 {
		t.Errorf("rocks.app/Contents/Frameworks/ holds %q, want golibnative.dylib", frameworks)
	}
	want := []string{filepath.Join(app, "Contents", "Frameworks", "golibnative.dylib"), app}
	if len(tp.signed) != 2 || tp.signed[0] != want[0] || tp.signed[1] != want[1] {
		t.Errorf("signed %q, want %q", tp.signed, want)
	}
	if notices := tp.folder(t, "rocks")[noticesFile]; !strings.Contains(notices, "rocks.app/Contents/Frameworks/golibnative.dylib: GoLib's native library") {
		t.Errorf("%s doesn't say where Jolt is:\n%s", noticesFile, notices)
	}
}

// A web build of a game with rigid bodies carries JoltPhysics.js.
func TestRigidBodiesOnTheWeb(t *testing.T) {
	tp := webProject(t, "rocks")
	tp.rigidBodies = true
	if code := tp.c.dist([]string{"--web"}); code != 1 || !strings.Contains(tp.stdout.String(), "games/rocks uses rigid bodies, which need JoltPhysics.js in a browser, and .tools/jolt/ has no jolt-physics.wasm.js: run golib setup") {
		t.Errorf("without JoltPhysics.js: exit code %d, output:\n%s", code, tp.stdout.String())
	}
	for _, f := range joltJSFiles {
		writeFile(t, tp.c.path(".tools", "jolt", f.name), "JoltPhysics.js's "+f.name)
	}
	tp.stdout.Reset()
	tp.c.failures = 0
	if code := tp.c.dist([]string{"--web"}); code != 0 {
		t.Fatalf("exit code %d, output:\n%s", code, tp.stdout.String())
	}
	web := readFolder(t, tp.c.path("build", "rocks", "dist", "web"))
	for _, f := range joltJSFiles {
		if web[f.name] != "JoltPhysics.js's "+f.name {
			t.Errorf("the web build has no %s: %q", f.name, web)
		}
	}
	for _, want := range []string{"\nJoltPhysics.js 1.1.0, with Jolt Physics\n", "--- LICENSE of JoltPhysics.js 1.1.0 ---", "--- LICENSE of Jolt Physics v5.6.0 ---"} {
		if !strings.Contains(web[noticesFile], want) {
			t.Errorf("%s doesn't contain %q:\n%s", noticesFile, want, web[noticesFile])
		}
	}
}

// A download that isn't the file GoLib expects is thrown away.
func TestDownloadsAreChecked(t *testing.T) {
	tp := newTestProject(t, "linux")
	tp.served[tp.c.joltJSURL(tp.c.joltJS[0].name)] = "something else"
	tp.c.syncNative()
	if !strings.Contains(tp.stdout.String(), "[fail] cannot download JoltPhysics.js 1.1.0: ") || !strings.Contains(tp.stdout.String(), "isn't the file GoLib expects") {
		t.Errorf("output:\n%s", tp.stdout.String())
	}
	if _, err := os.Stat(tp.c.path(".tools", "jolt", tp.c.joltJS[0].name)); err == nil {
		t.Error("the wrong file was kept")
	}

	// A platform with no release keeps a library built by hand.
	tp = newTestProject(t, "linux")
	tp.c.goarch = "riscv64"
	tp.c.syncNative()
	if !strings.Contains(tp.stdout.String(), "[info] GoLib's native library has no release for linux/riscv64 yet") {
		t.Errorf("no release, output:\n%s", tp.stdout.String())
	}
	writeFile(t, tp.c.nativeFile(), "built by hand")
	tp.stdout.Reset()
	tp.c.syncNative()
	if !strings.Contains(tp.stdout.String(), "[ok]   GoLib's native library, for rigid bodies, built by hand in .tools/native/") {
		t.Errorf("built by hand, output:\n%s", tp.stdout.String())
	}
}
