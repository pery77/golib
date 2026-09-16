package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// project is a GoLib project: the folder that holds framework/, games/ and
// tools/, and the platform that golib builds games for, which is this
// machine's.
type project struct {
	root         string
	goos, goarch string

	// runGo runs the project's go command, and runGame a game's executable.
	// Tests replace both.
	runGo   func(call goCall) ([]byte, error)
	runGame func(run gameRun) (exitCode int, timedOut bool, err error)
}

// goCall is one run of the project's go command.
type goCall struct {
	dir  string
	args []string
	// env holds variables to set on top of GoLib's environment (goEnv).
	env []string
	// output makes runGo return the command's standard output instead of
	// showing it.
	output bool
}

// gameRun is one run of a game's executable.
type gameRun struct {
	exe, dir string
	env      []string
	timeout  time.Duration // 0 for none
}

// newProject returns the project that exe, this program, belongs to. The
// scripts build it into build/golib/, so the root is two folders up.
func newProject(exe string, stdout, stderr io.Writer) (*project, error) {
	root := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
	if !isFile(filepath.Join(root, "tools", "cli", "go.mod")) {
		return nil, fmt.Errorf("%s is not in the build/golib/ folder of a GoLib project: start it with golib, which builds it there", exe)
	}
	p := &project{root: root, goos: runtime.GOOS, goarch: runtime.GOARCH}
	p.runGo = func(call goCall) ([]byte, error) {
		cmd := p.goCommand(call.dir, call.args)
		cmd.Env = append(cmd.Env, call.env...)
		cmd.Stderr = stderr
		if call.output {
			return cmd.Output()
		}
		cmd.Stdout = stdout
		return nil, cmd.Run()
	}
	p.runGame = func(run gameRun) (int, bool, error) {
		cmd := exec.Command(run.exe)
		cmd.Dir, cmd.Env = run.dir, run.env
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, stdout, stderr
		return waitForGame(cmd, run.timeout)
	}
	return p, nil
}

// waitForGame starts cmd and waits for it to end, or stops it after timeout,
// unless timeout is 0. It returns the game's exit code.
func waitForGame(cmd *exec.Cmd, timeout time.Duration) (exitCode int, timedOut bool, err error) {
	// Ctrl+C reaches the game too, which ends; this program stays to say how.
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	if err := cmd.Start(); err != nil {
		return 0, false, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var expired <-chan time.Time
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		expired = timer.C
	}
	select {
	case err = <-done:
	case <-expired:
		_ = cmd.Process.Kill()
		<-done
		return 0, true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), false, nil
	}
	return 0, false, err
}

// goRun runs the project's go command in dir, with its output going to the
// terminal.
func (p *project) goRun(dir string, args ...string) error {
	_, err := p.runGo(goCall{dir: dir, args: args})
	return err
}

// goOutput runs the project's go command in dir, and returns its standard
// output. Errors still go to the terminal.
func (p *project) goOutput(dir string, args ...string) ([]byte, error) {
	return p.runGo(goCall{dir: dir, args: args, output: true})
}

// path returns the full path of a file or folder in the project, given as
// the names along its path from the root.
func (p *project) path(names ...string) string {
	return filepath.Join(append([]string{p.root}, names...)...)
}

// shown returns how messages name path: from the project's root, with
// forward slashes.
func (p *project) shown(path string) string {
	relative, err := filepath.Rel(p.root, path)
	if err != nil || strings.HasPrefix(relative, "..") {
		return path
	}
	return filepath.ToSlash(relative)
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

// libraryPath returns the variable that makes programs look for shared
// libraries in dir before anywhere else, given the environment env: PATH on
// Windows, DYLD_LIBRARY_PATH on macOS, LD_LIBRARY_PATH on Linux.
func (p *project) libraryPath(dir string, env []string) string {
	name := "LD_LIBRARY_PATH"
	switch p.goos {
	case "windows":
		name = "PATH"
	case "darwin":
		name = "DYLD_LIBRARY_PATH"
	}
	if current := p.lookupEnv(env, name); current != "" {
		dir += string(os.PathListSeparator) + current
	}
	return name + "=" + dir
}

// lookupEnv returns the value of the variable name in env: its last one, as
// programs get it. Windows ignores the letter case of names.
func (p *project) lookupEnv(env []string, name string) string {
	value := ""
	for _, entry := range env {
		key, v, _ := strings.Cut(entry, "=")
		if key == name || (p.goos == "windows" && strings.EqualFold(key, name)) {
			value = v
		}
	}
	return value
}

// gameEnv returns the environment a debug build of game runs with: the
// user's, without any GOLIB_SHOT_ variables, with the game's build folder
// on the library search path outside Windows, where the executable's own
// folder is searched first anyway, and with extra.
func (c *cli) gameEnv(game string, extra ...string) []string {
	var env []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(entry), "GOLIB_SHOT_") {
			env = append(env, entry)
		}
	}
	if c.goos != "windows" {
		env = append(env, c.libraryPath(c.path("build", game), env))
	}
	return append(env, extra...)
}

// toolModules are GoLib's own Go programs, which don't use raylib.
var toolModules = []string{"tools/cli"}

// modules returns the project's Go modules, as paths from the root with
// forward slashes: the framework, each game, then GoLib's own programs.
func (p *project) modules() []string {
	var modules []string
	if isFile(p.path("framework", "go.mod")) {
		modules = append(modules, "framework")
	}
	for _, game := range p.games() {
		modules = append(modules, "games/"+game)
	}
	for _, tool := range toolModules {
		if isFile(p.path(filepath.FromSlash(tool), "go.mod")) {
			modules = append(modules, tool)
		}
	}
	return modules
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
