package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// gameDir returns a new game folder named rocks, holding game.json with
// content unless content is "".
func gameDir(t *testing.T, content string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "rocks")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if content != "" {
		if err := os.WriteFile(filepath.Join(dir, gameInfoFile), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestReadGameInfoDefaults(t *testing.T) {
	info, found, err := readGameInfo(gameDir(t, ""))
	if err != nil || found {
		t.Fatalf("without game.json: found = %v, error = %v; want false and no error", found, err)
	}
	if info.Title != "rocks" || info.Version != "0.0.0" || info.Author != "" || info.Copyright != "" {
		t.Errorf("defaults = %+v, want the folder name and version 0.0.0", info)
	}
	if info.version != (version{}) {
		t.Errorf("default version = %+v, want 0.0.0", info.version)
	}

	info, found, err = readGameInfo(gameDir(t, `{"version": "1.2.3"}`))
	if err != nil || !found {
		t.Fatalf("with a partial game.json: found = %v, error = %v; want true and no error", found, err)
	}
	if info.Title != "rocks" {
		t.Errorf("title = %q, want the folder name", info.Title)
	}
}

func TestReadGameInfo(t *testing.T) {
	info, _, err := readGameInfo(gameDir(t, `{
	"title": "Rocks in Space",
	"version": "1.20.3-beta.2",
	"author": "Ada Lovelace",
	"copyright": "Copyright 2026 Ada Lovelace"
}`))
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "Rocks in Space" || info.Author != "Ada Lovelace" || info.Copyright != "Copyright 2026 Ada Lovelace" {
		t.Errorf("info = %+v", info)
	}
	want := version{major: 1, minor: 20, patch: 3, label: "-beta.2"}
	if info.version != want {
		t.Errorf("version = %+v, want %+v", info.version, want)
	}
	if got := info.version.String(); got != "1.20.3-beta.2" {
		t.Errorf("version.String() = %q", got)
	}

	info, _, err = readGameInfo(gameDir(t, `{"buildTags": {"demo": "dist", "steam": "any"}}`))
	if err != nil || info.BuildTags["demo"] != tagDistOnly || info.BuildTags["steam"] != tagAnyBuild {
		t.Errorf("buildTags: %v, error %v", info.BuildTags, err)
	}

	info, _, err = readGameInfo(gameDir(t, `{"leaveOut": {"demo": ["assets/locale/*-full.txt", "assets/roms"]}}`))
	if err != nil || !slices.Equal(info.LeaveOut["demo"], []string{"assets/locale/*-full.txt", "assets/roms"}) {
		t.Errorf("leaveOut: %v, error %v", info.LeaveOut, err)
	}

	// Set-Content -Encoding UTF8 in Windows PowerShell 5.1 writes a byte order mark.
	info, _, err = readGameInfo(gameDir(t, "\xef\xbb\xbf{\"title\": \"Pequeño\"}"))
	if err != nil || info.Title != "Pequeño" {
		t.Errorf("with a byte order mark: title %q, error %v; want Pequeño and no error", info.Title, err)
	}
}

func TestReadGameInfoMistakes(t *testing.T) {
	tests := []struct {
		name, content, want string
	}{
		{"unknown field", `{"verison": "1.0.0"}`, `unknown field "verison": game.json takes "title", "version", "author", "copyright", "besideExecutable", "buildTags" and "leaveOut"`},
		{"syntax error", "{\n  \"title\": \"Rocks\",\n}", "line 3: "},
		{"number instead of text", "{\n  \"version\": 1.0\n}", `line 2: "version" must be text`},
		{"empty file", " ", "the file is empty or cut short"},
		{"two objects", `{} {}`, "more than one JSON object"},
		{"short version", `{"version": "1.0"}`, `"version" is "1.0": write major.minor.patch`},
		{"version too big", `{"version": "1.70000.0"}`, "between 0 and 65535"},
		{"v prefix", `{"version": "v1.0.0"}`, `"version" is "v1.0.0"`},
		{"empty title", `{"title": " "}`, `"title" is empty`},
		{"line break", `{"author": "Ada\nLovelace"}`, `"author" holds a line break`},
		{"long copyright", `{"copyright": "` + strings.Repeat("c", maxInfoText+1) + `"}`, `"copyright" is longer than 200 characters`},
		{"beside not a list", "{\n  \"besideExecutable\": \"assets/locale\"\n}", `line 2: "besideExecutable" must be a list of paths`},
		{"beside backslash", `{"besideExecutable": ["assets\\locale"]}`, "forward slashes"},
		{"beside outside", `{"besideExecutable": ["../other"]}`, "a path inside the game's folder"},
		{"beside absolute", `{"besideExecutable": ["/etc"]}`, "a path inside the game's folder"},
		{"beside the folder", `{"besideExecutable": ["."]}`, "a path inside the game's folder"},
		{"beside twice", `{"besideExecutable": ["assets/locale", "docs/locale"]}`, `"assets/locale" and "docs/locale", which would both be copied as locale`},
		{"tags a list", "{\n  \"buildTags\": [\"demo\"]\n}", `line 2: "buildTags" must give each build tag "dist" or "any"`},
		{"tag's build", `{"buildTags": {"demo": "release"}}`, `"buildTags" gives demo "release": write "dist" for dist and web builds only, or "any"`},
		{"tag's name", `{"buildTags": {"!demo": "dist"}}`, `"buildTags" has "!demo": a build tag is letters, digits, _ and .`},
		{"GoLib's tag", `{"buildTags": {"golib_dist": "any"}}`, `"buildTags" has golib_dist, which GoLib sets itself`},
		{"leave out a list", "{\n  \"leaveOut\": [\"assets/roms\"]\n}", `line 2: "leaveOut" must give each build tag a list of paths`},
		{"leave out a path", "{\n  \"leaveOut\": {\"demo\": \"assets/roms\"}\n}", `line 2: "leaveOut" must give each build tag a list of paths`},
		{"leave out's tag", `{"leaveOut": {"demo steam": ["assets/roms"]}}`, `"leaveOut" has "demo steam": a build tag is letters, digits, _ and .`},
		{"leave out GoLib's tag", `{"leaveOut": {"golib_dist": ["assets/roms"]}}`, `"leaveOut" has golib_dist, which GoLib sets itself`},
		{"leave out backslash", `{"leaveOut": {"demo": ["assets\\roms"]}}`, `"leaveOut" has "assets\\roms" for demo: write paths with forward slashes`},
		{"leave out outside", `{"leaveOut": {"demo": ["../roms"]}}`, `"leaveOut" has "../roms" for demo: write a path inside the game's folder`},
		{"leave out empty", `{"leaveOut": {"demo": [""]}}`, `"leaveOut" has "" for demo: write a path inside the game's folder`},
		{"leave out bad pattern", `{"leaveOut": {"demo": ["assets/locale/[a-z.txt"]}}`, `"leaveOut" has "assets/locale/[a-z.txt" for demo, which isn't a pattern path.Match takes`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, found, err := readGameInfo(gameDir(t, tt.content))
			if !found {
				t.Error("found = false, want true")
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want one containing %q", err, tt.want)
			}
		})
	}
}

// TestGameVersionFlag checks the linker flag that gives golib.GameVersion
// the version game.json says, and none without one.
func TestGameVersionFlag(t *testing.T) {
	for _, tt := range []struct{ content, want string }{
		{"", ""},
		{`{"title": "Rocks"}`, ""},
		{`{"version": null}`, ""},
		{`{"version": "0.0.0"}`, "-X golib.gameVersion=0.0.0"},
		{`{"version": "1.20.3-beta.2"}`, "-X golib.gameVersion=1.20.3-beta.2"},
	} {
		info, _, err := readGameInfo(gameDir(t, tt.content))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.versionFlag(); got != tt.want {
			t.Errorf("%s: versionFlag() = %q, want %q", tt.content, got, tt.want)
		}
		if info.Version == "" {
			t.Errorf("%s: Version is empty, want 0.0.0 for the executable's details", tt.content)
		}
	}
}
