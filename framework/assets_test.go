package golib

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func TestReadAssetNames(t *testing.T) {
	files := fstest.MapFS{
		"assets/levels/intro.txt": {Data: []byte("hello")},
		"main.go":                 {Data: []byte("package main")},
	}
	tests := []struct {
		name    string
		asset   string
		want    string
		wantErr string // part of the error message; empty when no error is expected
	}{
		{name: "file in a subfolder", asset: "levels/intro.txt", want: "hello"},
		{name: "missing file", asset: "levels/outro.txt", wantErr: "assets/levels/outro.txt not found in the test files"},
		{name: "outside the assets folder", asset: "../main.go", wantErr: "invalid asset name"},
		{name: "absolute path", asset: "/levels/intro.txt", wantErr: "invalid asset name"},
		{name: "backslashes", asset: `levels\intro.txt`, wantErr: "forward slashes"},
		{name: "the folder itself", asset: ".", wantErr: "invalid asset name"},
		{name: "a folder", asset: "levels", wantErr: "cannot read assets/levels"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readAsset(files, "in the test files", tt.asset)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("readAsset(%q) error = %v, want one containing %q", tt.asset, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("readAsset(%q) error = %v", tt.asset, err)
			}
			if string(got) != tt.want {
				t.Errorf("readAsset(%q) = %q, want %q", tt.asset, got, tt.want)
			}
		})
	}
}

func TestReadAssetFromWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "note.txt"), []byte("from disk"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	got, err := ReadAsset("note.txt")
	if err != nil {
		t.Fatalf("ReadAsset() error = %v", err)
	}
	if string(got) != "from disk" {
		t.Errorf("ReadAsset() = %q, want %q", got, "from disk")
	}
}

func TestDebugGameDir(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	gameDir := filepath.Join(root, "games", "rocks")
	buildDir := filepath.Join(root, "build", "rocks")
	exe := filepath.Join(buildDir, "rocks.exe")
	folders := map[string]bool{
		filepath.Join(gameDir, "assets"): true,
	}
	isDir := func(path string) bool { return folders[path] }

	tests := []struct {
		name    string
		workDir string
		exe     string
		want    string
	}{
		{name: "started in the game's folder", workDir: gameDir, exe: exe, want: gameDir},
		{name: "started from Explorer", workDir: buildDir, exe: exe, want: gameDir},
		{name: "started from another folder", workDir: root, exe: exe, want: gameDir},
		{name: "F5 output name", workDir: buildDir, exe: filepath.Join(buildDir, "debug"), want: gameDir},
		{name: "executable unknown", workDir: buildDir, exe: "", want: buildDir},
		{name: "executable outside build", workDir: root, exe: filepath.Join(root, "rocks", "rocks.exe"), want: root},
		{name: "game without assets", workDir: root, exe: filepath.Join(root, "build", "other", "other.exe"), want: root},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := debugGameDir(tt.workDir, tt.exe, isDir); got != tt.want {
				t.Errorf("debugGameDir(%q, %q) = %q, want %q", tt.workDir, tt.exe, got, tt.want)
			}
		})
	}

	// A working directory with an assets folder wins, even in build/<game>/.
	folders[filepath.Join(buildDir, "assets")] = true
	if got := debugGameDir(buildDir, exe, isDir); got != buildDir {
		t.Errorf("debugGameDir with assets in the working directory = %q, want %q", got, buildDir)
	}
}

func TestAssetSource(t *testing.T) {
	if _, where, err := assetSource(fstest.MapFS{}, true); err != nil || where != "in the executable" {
		t.Errorf("with embedded assets: where = %q, error = %v; want %q and no error", where, err, "in the executable")
	}
	if _, _, err := assetSource(nil, true); err == nil || !strings.Contains(err.Error(), "assets.go") {
		t.Errorf("dist build without embedded assets: error = %v, want one that mentions assets.go", err)
	}
	if _, where, err := assetSource(nil, false); err != nil || !strings.HasPrefix(where, "in ") {
		t.Errorf("debug build: where = %q, error = %v; want the working directory and no error", where, err)
	}
}

func TestListAssets(t *testing.T) {
	useAssets(t, map[string][]byte{
		"maps/level2.tmx":      []byte("two"),
		"maps/level1.tmx":      []byte("one"),
		"maps/tiles/tiles.png": []byte("picture"),
		"sounds/coin.wav":      []byte("coin"),
		"readme.txt":           []byte("hello"),
	})
	tests := []struct {
		folder string
		want   []string
	}{
		{"maps", []string{"maps/level1.tmx", "maps/level2.tmx"}},
		{"", []string{"readme.txt"}},
		{"maps/tiles", []string{"maps/tiles/tiles.png"}},
	}
	for _, test := range tests {
		got, err := ListAssets(test.folder)
		if err != nil || !slices.Equal(got, test.want) {
			t.Errorf("ListAssets(%q) = %v, %v; want %v", test.folder, got, err, test.want)
		}
	}
	for _, folder := range []string{"levels", "..", `maps\tiles`, "."} {
		if _, err := ListAssets(folder); err == nil {
			t.Errorf("ListAssets(%q): no error", folder)
		}
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
}
