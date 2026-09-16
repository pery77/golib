package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// project is a GoLib project: the folder that holds framework/, games/ and
// tools/, and the platform that golib builds games for, which is this
// machine's.
type project struct {
	root         string
	goos, goarch string

	// goRun runs the project's go command in dir, with its output going to
	// the terminal. goOutput returns its standard output instead. Tests
	// replace both.
	goRun    func(dir string, args ...string) error
	goOutput func(dir string, args ...string) ([]byte, error)
}

// newProject returns the project that exe, this program, belongs to. The
// scripts build it into build/golib/, so the root is two folders up.
func newProject(exe string, stdout, stderr io.Writer) (*project, error) {
	root := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
	if !isFile(filepath.Join(root, "tools", "cli", "go.mod")) {
		return nil, fmt.Errorf("%s is not in the build/golib/ folder of a GoLib project: start it with golib, which builds it there", exe)
	}
	p := &project{root: root, goos: runtime.GOOS, goarch: runtime.GOARCH}
	p.goRun = func(dir string, args ...string) error {
		cmd := p.goCommand(dir, args)
		cmd.Stdout, cmd.Stderr = stdout, stderr
		return cmd.Run()
	}
	p.goOutput = func(dir string, args ...string) ([]byte, error) {
		cmd := p.goCommand(dir, args)
		cmd.Stderr = stderr
		return cmd.Output()
	}
	return p, nil
}

// path returns the full path of a file or folder in the project, given as
// the names along its path from the root.
func (p *project) path(names ...string) string {
	return filepath.Join(append([]string{p.root}, names...)...)
}

// executable returns the file name of a program called name on the platform.
func (p *project) executable(name string) string {
	if p.goos == "windows" {
		return name + ".exe"
	}
	return name
}

// goCommand returns the project's go command, ready to run in dir with args.
func (p *project) goCommand(dir string, args []string) *exec.Cmd {
	cmd := exec.Command(p.path(".tools", "go", "bin", p.executable("go")), args...)
	cmd.Dir = dir
	cmd.Env = p.goEnv()
	if p.goos == "darwin" {
		// HOME points here; Go expects the folder to exist.
		_ = os.MkdirAll(p.path(".tools", "home"), 0o755)
	}
	return cmd
}

// goEnv returns the environment of every go command golib starts: the
// user's, with Go pointed at the toolchain and the caches in .tools/, so
// nothing is read from or written to the user's folders (see
// docs/tooling.md). The golib scripts set the same variables for the go
// commands they still run: change both together.
func (p *project) goEnv() []string {
	goRoot := p.path(".tools", "go")
	// When a variable appears twice, the go command gets the last value.
	env := append(os.Environ(),
		"GOROOT="+goRoot,
		"GOPATH="+p.path(".tools", "gopath"),
		"GOMODCACHE="+p.path(".tools", "gopath", "pkg", "mod"),
		"GOCACHE="+p.path(".tools", "gocache"),
		"GOENV=off",         // ignore any global "go env -w" settings
		"GOTOOLCHAIN=local", // never download a different Go version
		"CGO_ENABLED=0",     // raylib-go without a C compiler
		// Debug builds load raylib and libffi from build/ or .tools/ instead
		// of extracting them into a user folder. dist replaces these tags.
		"GOFLAGS=-tags=raylib_no_embed,ffi_no_embed",
		"PATH="+filepath.Join(goRoot, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	// Go writes telemetry counters to the user's config folder: keep them in
	// .tools/ too.
	switch p.goos {
	case "windows":
		env = append(env, "APPDATA="+p.path(".tools", "config"))
	case "darwin":
		env = append(env, "HOME="+p.path(".tools", "home"))
	default:
		env = append(env, "XDG_CONFIG_HOME="+p.path(".tools", "config"))
	}
	return env
}

// games returns the names of the games: the folders in games/ that hold a
// go.mod, in alphabetical order.
func (p *project) games() []string {
	entries, _ := os.ReadDir(p.path("games")) // no games/ folder: no games
	var names []string
	for _, entry := range entries {
		if entry.IsDir() && isFile(p.path("games", entry.Name(), "go.mod")) {
			names = append(names, entry.Name())
		}
	}
	return names
}

// resolveGame returns the game that a command's options name, or the only
// game when they name none. When they don't pick one game, it explains why
// and returns "" and exit code 2.
func (c *cli) resolveGame(command string, options []string) (game string, exitCode int) {
	if len(options) > 1 {
		return "", c.usage(fmt.Sprintf("%s takes at most one game name (got: %s)", command, strings.Join(options, " ")))
	}
	games := c.games()
	if len(options) == 1 {
		name := options[0]
		for _, game := range games {
			if game == name {
				return game, 0
			}
		}
		// Then whatever the letter case, as Windows finds folders.
		for _, game := range games {
			if strings.EqualFold(game, name) {
				return game, 0
			}
		}
		if len(games) == 0 {
			return "", c.usage(fmt.Sprintf("no game named %q: games/ has no games yet (create one: golib new <name>)", name))
		}
		return "", c.usage(fmt.Sprintf("no game named %q in games/ (available: %s)", name, strings.Join(games, ", ")))
	}
	switch len(games) {
	case 0:
		return "", c.usage("there are no games in games/ yet (create one: golib new <name>)")
	case 1:
		return games[0], 0
	}
	return "", c.usage(fmt.Sprintf("%s needs a game name (available: %s)", command, strings.Join(games, ", ")))
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
