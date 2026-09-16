package main

import (
	"os"
	"path/filepath"
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
}

func TestReadGameInfoMistakes(t *testing.T) {
	tests := []struct {
		name, content, want string
	}{
		{"unknown field", `{"verison": "1.0.0"}`, `unknown field "verison": game.json takes "title", "version", "author" and "copyright"`},
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
