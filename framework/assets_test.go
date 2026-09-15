package golib

import (
	"os"
	"path/filepath"
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
