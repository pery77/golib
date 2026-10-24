package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// test runs go vet and go test for every Go module in the project: the
// framework, each game and GoLib's own programs. Given a game's name, it
// tests only that game, so a game in progress elsewhere doesn't get in the
// way. With --tags, vet and the tests see the files those tags build. With
// --run <pattern>, only the tests whose names match run, and what they log
// shows even when they pass: the way to read what a test prints, since
// golib go test can't start a test that loads raylib on macOS.
func (c *cli) test(options []string) int {
	options, exitCode := c.takeTags("test", options)
	if exitCode != 0 {
		return exitCode
	}
	var run string
	for i := 0; i < len(options); i++ {
		switch option := options[i]; {
		case option == "--run":
			if i+1 >= len(options) || options[i+1] == "" {
				return c.usage("test --run needs the names of the tests to run, as a pattern, such as: golib test --run TestJump")
			}
			run = options[i+1]
			options = append(options[:i:i], options[i+2:]...)
			i--
		case strings.HasPrefix(option, "--run="):
			run = strings.TrimPrefix(option, "--run=")
			options = append(options[:i:i], options[i+1:]...)
			i--
		}
	}
	modules := c.modules()
	if len(options) > 0 {
		game, exitCode := c.resolveGame("test", options)
		if game == "" {
			return exitCode
		}
		modules = []string{"games/" + game}
	}
	if len(modules) == 0 {
		c.check("warn", "no Go modules to test")
	}
	c.reportTags()
	for _, module := range modules {
		dir := c.path(filepath.FromSlash(module))
		vet := append([]string{"vet"}, c.tagsArgs()...)
		if err := c.goRun(dir, append(vet, "./...")...); err != nil {
			c.check("fail", module+": go vet found problems (see above)")
			continue
		}
		test := append([]string{"test"}, c.tagsArgs()...)
		if run != "" {
			test = append(test, "-v", "-run", run)
		}
		if strings.HasPrefix(module, "games/") {
			// A game's tests see its version in golib.GameVersion, as its
			// builds do. A game.json with a mistake gives none: build and
			// dist report it.
			if info, _, err := readGameInfo(dir); err == nil {
				test = append(test, versionArgs(info)...)
			}
		}
		var env []string
		if !isToolModule(module) {
			// Test binaries load raylib and libffi when they start, so
			// .tools/raylib/ goes on the library search path, and
			// .tools/native/ after it, for tests of rigid bodies.
			if _, err := c.syncRaylib(dir); err != nil {
				c.check("fail", module+": "+err.Error())
				continue
			}
			libraryPath := c.libraryPath(c.path(".tools", "raylib")+string(os.PathListSeparator)+c.path(".tools", "native"), c.goEnv())
			env = append(env, libraryPath)
			if c.goos == "darwin" {
				exec, err := testExec(libraryPath)
				if err != nil {
					c.check("fail", module+": "+err.Error())
					continue
				}
				test = append(test, "-exec", exec)
			}
		}
		if _, err := c.runGo(goCall{dir: dir, args: append(test, "./..."), env: env}); err != nil {
			c.check("fail", module+": tests failed (see above)")
			continue
		}
		c.check("ok", module+": vet and tests passed")
	}
	return c.summary("test")
}

// testExec returns the -exec command that makes go test start each test
// binary with libraryPath, a DYLD_LIBRARY_PATH=... assignment, on macOS.
//
// There the go command is signed with the hardened runtime, so macOS takes
// every DYLD_ variable out of its environment when it starts, and the test
// binaries it starts never see the library search path: they stop at once,
// unable to load libffi. /usr/bin/env sets the variable again for each one.
// Test binaries are linked by the go command and not hardened, so they keep
// it. go test caches no results run this way.
//
// go test splits the command at spaces, with quotes around a part that has
// them, and no escapes inside: a project path with spaces and both kinds of
// quotes can't be written.
func testExec(libraryPath string) (string, error) {
	if !strings.ContainsAny(libraryPath, " \t\n\r") {
		return "/usr/bin/env " + libraryPath, nil
	}
	for _, quote := range []string{"'", `"`} {
		if !strings.Contains(libraryPath, quote) {
			return "/usr/bin/env " + quote + libraryPath + quote, nil
		}
	}
	return "", fmt.Errorf("go test can't be told where the libraries are: the project's path has spaces and both kinds of quotes in it (%s); move the project to a path without quotes", libraryPath)
}
