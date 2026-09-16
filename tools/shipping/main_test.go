package main

import (
	"bytes"
	"debug/pe"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunUsage(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"something-else"},
		{"windows-resources"},
		{"windows-resources", "-game", "games/rocks", "-arch", "amd64"},
		{"windows-resources", "-game", "games/rocks", "-arch", "amd64", "-out", "x.syso", "extra"},
		{"windows-resources", "-unknown"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Errorf("run(%q) = %d, want 2", args, code)
		}
		if !strings.Contains(stderr.String(), "usage: ") || stdout.Len() > 0 {
			t.Errorf("run(%q) printed %q and %q, want usage on stderr only", args, stdout.String(), stderr.String())
		}
	}
}

func TestRunWithoutGameInfoOrIcon(t *testing.T) {
	dir := gameDir(t, "")
	output := filepath.Join(t.TempDir(), "resources.syso")
	var stdout, stderr bytes.Buffer
	code := run([]string{"windows-resources", "-game", dir, "-arch", "amd64", "-out", output}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code %d, output %q %q", code, stdout.String(), stderr.String())
	}
	want := `info games/rocks has no game.json, so the executable's details say "rocks", version 0.0.0: add one to set the title, version and author (see docs/tooling.md)
info games/rocks has no icon.png, so the game shows Windows' default icon: add a square PNG, ideally 256 by 256 pixels
`
	if stdout.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", stdout.String(), want)
	}

	file, err := pe.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	data, err := file.Section(".rsrc").Data()
	if err != nil {
		t.Fatal(err)
	}
	resources := readResources(t, data)
	if len(resources) != 1 || resources[0].kind != rtVersion {
		t.Errorf("resources = %v, want the version information only", resources)
	}
}

func TestRunWithGameInfoAndIcon(t *testing.T) {
	dir := gameDir(t, `{"title": "Rocks", "version": "1.0.0", "author": "Ada"}`)
	icon, err := os.ReadFile(writePNG(t, checkerboard(64)))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, iconFile), icon, 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "resources.syso")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"windows-resources", "-game", dir, "-arch", "arm64", "-out", output}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d, output %q %q", code, stdout.String(), stderr.String())
	}
	want := `ok games/rocks/game.json: the executable's details say "Rocks", version 1.0.0, by Ada
ok games/rocks/icon.png (64 by 64 pixels): the game's icon, in 8 sizes from 16 to 256 pixels
`
	if stdout.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", stdout.String(), want)
	}
	file, err := pe.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if file.Machine != pe.IMAGE_FILE_MACHINE_ARM64 {
		t.Errorf("machine = %#x, want arm64", file.Machine)
	}
	data, err := file.Section(".rsrc").Data()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(readResources(t, data)); got != 1+len(iconSizes)+1 {
		t.Errorf("%d resources, want the version, %d icons and their group", got, len(iconSizes))
	}
}

func TestRunFailures(t *testing.T) {
	tests := []struct {
		name, gameInfo, arch, want string
	}{
		{"bad game.json", `{"version": "one"}`, "amd64", `fail games/rocks/game.json: "version" is "one"`},
		{"unknown processor", "", "386", `fail GoLib can't make Windows resources for "386" processors`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "resources.syso")
			var stdout, stderr bytes.Buffer
			code := run([]string{"windows-resources", "-game", gameDir(t, tt.gameInfo), "-arch", tt.arch, "-out", output}, &stdout, &stderr)
			if code != 1 || !strings.HasPrefix(stdout.String(), tt.want) {
				t.Errorf("exit code %d, output %q; want 1 and a line starting %q", code, stdout.String(), tt.want)
			}
			if _, err := os.Stat(output); err == nil {
				t.Error("a failed run wrote the output file")
			}
		})
	}

	dir := gameDir(t, "")
	if err := os.WriteFile(filepath.Join(dir, iconFile), []byte("not a png"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"windows-resources", "-game", dir, "-arch", "amd64", "-out", filepath.Join(t.TempDir(), "r.syso")}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stdout.String(), "fail games/rocks/icon.png: not a PNG image") {
		t.Errorf("a broken icon.png: exit code %d, output %q", code, stdout.String())
	}
}
