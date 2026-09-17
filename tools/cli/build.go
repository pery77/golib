package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// build makes a debug build of a game: build/<game>/<game>.exe, next to
// copies of the libraries it loads (see docs/tooling.md#dist-builds).
func (c *cli) build(options []string) int {
	game, exitCode := c.resolveGame("build", options)
	if game == "" {
		return exitCode
	}
	c.buildGame(game)
	return c.summary("build")
}

// run makes a debug build of a game, then runs it from its folder, with the
// user's environment. With --dist it runs the build players get instead.
func (c *cli) run(options []string) int {
	var names []string
	dist := false
	for _, option := range options {
		switch {
		case option == "--dist":
			dist = true
		case strings.HasPrefix(option, "-"):
			return c.usage("unknown option for run: " + option)
		default:
			names = append(names, option)
		}
	}
	game, exitCode := c.resolveGame("run", names)
	if game == "" {
		return exitCode
	}
	if dist {
		return c.runDist(game)
	}
	exe := c.buildGame(game)
	if exe == "" {
		return c.summary("run")
	}
	shownExe := c.shown(exe)
	c.check("info", fmt.Sprintf("running %s with games/%s/ as the working directory", shownExe, game))
	code, _, err := c.runGame(gameRun{exe: exe, dir: c.path("games", game), env: c.gameEnv(game)})
	if err != nil {
		c.check("fail", fmt.Sprintf("cannot start %s: %v", shownExe, err))
		return c.summary("run")
	}
	fmt.Fprintf(c.stdout, "\nrun: %s exited with code %d\n", game, code)
	if code != 0 {
		return 1
	}
	return 0
}

// runDist makes the dist build of a game and runs it the way a player would:
// from the folder golib dist puts it in, next to the libraries it loads, with
// nothing of GoLib's around it. It is how to see what players get, because a
// dist build carries its assets inside the executable, where a debug build
// reads them from the game's folder.
func (c *cli) runDist(game string) int {
	if !c.distGame(game) {
		return c.summary("run")
	}
	folder := c.path("build", game, "dist", game)
	exe := filepath.Join(folder, c.executable(game))
	shownExe := c.shown(exe)
	c.check("info", fmt.Sprintf("running %s as a player would, from its own folder", shownExe))
	code, _, err := c.runGame(gameRun{exe: exe, dir: folder, env: c.cleanEnv()})
	if err != nil {
		c.check("fail", fmt.Sprintf("cannot start %s: %v", shownExe, err))
		return c.summary("run")
	}
	fmt.Fprintf(c.stdout, "\nrun: %s exited with code %d\n", game, code)
	if code != 0 {
		return 1
	}
	return 0
}

// buildGame makes a debug build of game, and returns the executable's path,
// or "" after reporting a failure. Debug builds keep their debug symbols,
// open a console window on Windows, read the game's assets from disk, and
// load raylib and libffi from their own folder, where buildGame copies them
// from .tools/raylib/. On Windows they carry the game's icon and details, as
// dist builds do, without reporting them.
func (c *cli) buildGame(game string) string {
	dir := c.path("games", game)
	outDir := c.path("build", game)
	exe := filepath.Join(outDir, c.executable(game))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		c.check("fail", fmt.Sprintf("cannot create build/%s/: %v", game, err))
		return ""
	}
	removeResources := func() {}
	if c.goos == "windows" {
		var ok bool
		_, removeResources, ok = c.addWindowsResources(game, false)
		if !ok {
			removeResources()
			return ""
		}
	}
	err := c.goRun(dir, "build", "-o", exe, ".")
	removeResources()
	if err != nil {
		c.check("fail", fmt.Sprintf("build failed for games/%s (see the Go errors above)", game))
		return ""
	}
	synced, err := c.syncRaylib(dir)
	if err != nil {
		c.check("fail", err.Error())
		return ""
	}
	for _, library := range synced.libraries {
		if err := syncFile(library, filepath.Join(outDir, filepath.Base(library))); err != nil {
			c.check("fail", fmt.Sprintf("cannot copy %s into build/%s/ (is the game still running?): %v", filepath.Base(library), game, err))
			return ""
		}
	}
	c.check("ok", fmt.Sprintf("built games/%s into %s", game, c.shown(exe)))
	return exe
}
