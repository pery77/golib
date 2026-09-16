package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNewProject(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "build", "golib", "golib.exe")
	if _, err := newProject(exe, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "start it with golib") {
		t.Errorf("outside a project: error = %v, want one that says to start it with golib", err)
	}

	writeFile(t, filepath.Join(root, "tools", "cli", "go.mod"), "module cli\n")
	p, err := newProject(exe, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if p.root != root {
		t.Errorf("root = %s, want %s", p.root, root)
	}
	if p.path("games", "rocks") != filepath.Join(root, "games", "rocks") {
		t.Errorf("path(games, rocks) = %s", p.path("games", "rocks"))
	}
}

func TestGoCommand(t *testing.T) {
	p := &project{root: t.TempDir(), goos: "windows"}
	cmd := p.goCommand("somewhere", []string{"build", "."})
	if want := p.path(".tools", "go", "bin", "go.exe"); cmd.Path != want {
		t.Errorf("path = %s, want %s", cmd.Path, want)
	}
	if !slices.Equal(cmd.Args[1:], []string{"build", "."}) || cmd.Dir != "somewhere" {
		t.Errorf("args %q in %s, want build . in somewhere", cmd.Args[1:], cmd.Dir)
	}
	if p.executable("go") != "go.exe" || (&project{goos: "linux"}).executable("go") != "go" {
		t.Error("executable names: want .exe on Windows only")
	}
}

func TestGoEnv(t *testing.T) {
	t.Setenv("GOFLAGS", "-v")
	t.Setenv("PATH", "user-path")
	for _, tt := range []struct {
		goos, configVariable, configFolder string
	}{
		{"windows", "APPDATA", "config"},
		{"linux", "XDG_CONFIG_HOME", "config"},
		{"darwin", "HOME", "home"},
	} {
		p := &project{root: t.TempDir(), goos: tt.goos}
		env := p.goEnv()
		// The go command gets the last value of each variable.
		last := map[string]string{}
		for _, entry := range env {
			name, value, _ := strings.Cut(entry, "=")
			last[strings.ToUpper(name)] = value
		}
		want := map[string]string{
			"GOROOT":          p.path(".tools", "go"),
			"GOPATH":          p.path(".tools", "gopath"),
			"GOMODCACHE":      p.path(".tools", "gopath", "pkg", "mod"),
			"GOCACHE":         p.path(".tools", "gocache"),
			"GOENV":           "off",
			"GOTOOLCHAIN":     "local",
			"CGO_ENABLED":     "0",
			"GOFLAGS":         "-tags=raylib_no_embed,ffi_no_embed",
			"PATH":            p.path(".tools", "go", "bin") + string(os.PathListSeparator) + "user-path",
			tt.configVariable: p.path(".tools", tt.configFolder),
		}
		for name, value := range want {
			if last[name] != value {
				t.Errorf("%s: %s = %q, want %q", tt.goos, name, last[name], value)
			}
		}
	}
}

func TestGames(t *testing.T) {
	p := &project{root: t.TempDir()}
	if games := p.games(); len(games) != 0 {
		t.Errorf("without games/: games = %q, want none", games)
	}
	writeFile(t, p.path("games", "snake", "go.mod"), "module snake\n")
	writeFile(t, p.path("games", "rocks", "go.mod"), "module rocks\n")
	writeFile(t, p.path("games", "notes", "README.md"), "not a game: no go.mod")
	writeFile(t, p.path("games", "go.mod"), "a file, not a game folder")
	if games := p.games(); !slices.Equal(games, []string{"rocks", "snake"}) {
		t.Errorf("games = %q, want rocks and snake", games)
	}
}

func TestResolveGame(t *testing.T) {
	var stderr bytes.Buffer
	c := &cli{project: &project{root: t.TempDir()}, stdout: io.Discard, stderr: &stderr}
	writeFile(t, c.path("games", "rocks", "go.mod"), "module rocks\n")
	for _, options := range [][]string{nil, {"rocks"}, {"Rocks"}} {
		if game, code := c.resolveGame("build", options); game != "rocks" || code != 0 {
			t.Errorf("resolveGame(%q) = %q, %d; want rocks, 0", options, game, code)
		}
	}
	if game, code := c.resolveGame("build", []string{"pong"}); game != "" || code != 2 {
		t.Errorf("an unknown game: resolveGame = %q, %d; want \"\", 2", game, code)
	}
	if want := "golib: no game named \"pong\" in games/ (available: rocks)\nRun \"golib help\" for usage.\n"; stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}
