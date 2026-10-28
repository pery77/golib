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
	"slices"
	"strings"
	"time"
)

// project is a GoLib project: the folder that holds framework/, examples/,
// games/ and tools/, and the platform that golib builds games for, which is this
// machine's.
type project struct {
	root         string
	goos, goarch string

	// runGo runs the project's go command, runGame a game's executable,
	// signApp signs a macOS app with macOS's codesign, and download fetches
	// a file from the internet. Tests replace them.
	runGo    func(call goCall) ([]byte, error)
	runGame  func(run gameRun) (exitCode int, timedOut bool, err error)
	signApp  func(app string) error
	download func(url string) ([]byte, error)

	// natives and joltJS are the files of Jolt that setup downloads, with
	// their checksums: nativeChecksums and joltJSFiles, which tests replace.
	natives map[string]string
	joltJS  []namedSum
	// llms holds the checksums of golibllm's zips: llmChecksums, which tests
	// replace.
	llms map[string]string
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
	p := &project{root: root, goos: runtime.GOOS, goarch: runtime.GOARCH, download: download, natives: nativeChecksums, joltJS: joltJSFiles, llms: llmChecksums}
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
	p.signApp = func(app string) error {
		// An ad hoc signature, "-", needs no Apple developer account: it
		// seals the app's files, without saying who made them.
		output, err := exec.Command("codesign", "--force", "--sign", "-", app).CombinedOutput()
		if err != nil {
			return fmt.Errorf("codesign: %v: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
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
		// Windows gives exit codes as unsigned 32-bit numbers: -1, the code
		// of a game ended from Task Manager, comes as 4294967295.
		return int(int32(exit.ExitCode())), false, nil
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
		"GOFLAGS=-tags="+debugTags,
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
	env := c.cleanEnv()
	if c.goos != "windows" {
		env = append(env, c.libraryPath(c.path("build", game), env))
	}
	return append(env, extra...)
}

// cleanEnv returns this program's environment without the variables golib shot
// speaks to a game through, so that a game never inherits them by accident. A
// dist build runs with exactly this: a player has no GoLib around them, and
// pointing it at the debug build's libraries would hide one missing from the
// folder to share.
func (c *cli) cleanEnv() []string {
	var env []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(entry), "GOLIB_SHOT_") {
			env = append(env, entry)
		}
	}
	return env
}

// toolModules are GoLib's own Go programs, which don't use raylib.
var toolModules = []string{"tools/cli"}

// isToolModule reports whether module, a path from the root, is one of GoLib's
// own programs.
func isToolModule(module string) bool {
	return slices.Contains(toolModules, module)
}

// The two folders that hold games, each game a Go module of its own: games/,
// the user's games, which .gitignore keeps out of the GoLib repository so
// each can have a git repository of its own, and examples/, GoLib's
// examples, which are part of it (see docs/architecture.md#your-games).
const (
	gamesFolder    = "games"
	examplesFolder = "examples"
)

// modules returns the project's Go modules, as paths from the root with
// forward slashes: the framework, each example, each game, then GoLib's own
// programs.
func (p *project) modules() []string {
	var modules []string
	if isFile(p.path("framework", "go.mod")) {
		modules = append(modules, "framework")
	}
	for _, example := range p.examples() {
		modules = append(modules, examplesFolder+"/"+example)
	}
	for _, game := range p.games() {
		modules = append(modules, gamesFolder+"/"+game)
	}
	for _, tool := range toolModules {
		if isFile(p.path(filepath.FromSlash(tool), "go.mod")) {
			modules = append(modules, tool)
		}
	}
	return modules
}

// isGameModule reports whether module, a path from the root, is a game or an
// example.
func isGameModule(module string) bool {
	return strings.HasPrefix(module, gamesFolder+"/") || strings.HasPrefix(module, examplesFolder+"/")
}

// games returns the names of the user's games: the folders in games/ that
// hold a go.mod, in alphabetical order.
func (p *project) games() []string {
	return p.gamesIn(gamesFolder)
}

// examples returns the names of GoLib's examples: the folders in examples/
// that hold a go.mod, in alphabetical order.
func (p *project) examples() []string {
	return p.gamesIn(examplesFolder)
}

// gamesIn returns the names of the folders in folder, games or examples,
// that hold a go.mod, in alphabetical order.
func (p *project) gamesIn(folder string) []string {
	entries, _ := os.ReadDir(p.path(folder)) // no folder: no games
	var names []string
	for _, entry := range entries {
		if entry.IsDir() && isFile(p.path(folder, entry.Name(), "go.mod")) {
			names = append(names, entry.Name())
		}
	}
	return names
}

// gameModule returns the folder of game, a name resolveGame returned, as a
// path from the root with forward slashes: examples/<game> for one of
// GoLib's examples, games/<game> for any other. Messages name a game's
// folder this way.
func (p *project) gameModule(game string) string {
	if !isFile(p.path(gamesFolder, game, "go.mod")) && isFile(p.path(examplesFolder, game, "go.mod")) {
		return examplesFolder + "/" + game
	}
	return gamesFolder + "/" + game
}

// gameDir returns the full path of game's folder, or of a file or folder in
// it, given as the names along its path.
func (p *project) gameDir(game string, names ...string) string {
	return p.path(append([]string{filepath.FromSlash(p.gameModule(game))}, names...)...)
}

// resolveGame returns the game or example that a command's options name, or
// the only game in games/ when they name none: examples are always named.
// When they don't pick one, it explains why and returns "" and exit code 2.
func (c *cli) resolveGame(command string, options []string) (game string, exitCode int) {
	if len(options) > 1 {
		return "", c.usage(fmt.Sprintf("%s takes at most one game name (got: %s)", command, strings.Join(options, " ")))
	}
	games, examples := c.games(), c.examples()
	if len(options) == 1 {
		name := options[0]
		// First the exact name, then whatever the letter case, as Windows
		// finds folders.
		for _, same := range []func(a, b string) bool{func(a, b string) bool { return a == b }, strings.EqualFold} {
			var found []string
			for _, folder := range [][]string{games, examples} {
				for _, game := range folder {
					if same(game, name) {
						found = append(found, game)
					}
				}
			}
			switch len(found) {
			case 1:
				return found[0], 0
			case 2:
				// build/<game>/ and golib.SaveData go by the name alone, so
				// a game can't share one with an example.
				return "", c.usage(fmt.Sprintf("there are both games/%s and examples/%s, one of GoLib's examples: rename your game's folder", found[0], found[1]))
			}
		}
		return "", c.usage(fmt.Sprintf("no game named %q in games/ or examples/ (%s)", name, c.available(games, examples)))
	}
	switch len(games) {
	case 0:
		if len(examples) == 0 {
			return "", c.usage("there are no games in games/ yet (create one: golib new <name>)")
		}
		return "", c.usage(fmt.Sprintf("there are no games in games/ yet: create one with golib new <name>, or name an example (%s)", c.available(games, examples)))
	case 1:
		return games[0], 0
	}
	return "", c.usage(fmt.Sprintf("%s needs a game name (%s)", command, c.available(games, examples)))
}

// available names the games and the examples there are, for messages.
func (c *cli) available(games, examples []string) string {
	var lists []string
	if len(games) > 0 {
		lists = append(lists, "games: "+strings.Join(games, ", "))
	}
	if len(examples) > 0 {
		lists = append(lists, "examples: "+strings.Join(examples, ", "))
	}
	if len(lists) == 0 {
		return "there are none yet: create one with golib new <name>"
	}
	return strings.Join(lists, "; ")
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
