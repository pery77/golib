package main

import (
	"path/filepath"
)

// test runs go vet and go test for every Go module in the project: the
// framework, each game and GoLib's own programs. Given a game's name, it
// tests only that game, so a game in progress elsewhere doesn't get in the
// way.
func (c *cli) test(options []string) int {
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
	for _, module := range modules {
		dir := c.path(filepath.FromSlash(module))
		if err := c.goRun(dir, "vet", "./..."); err != nil {
			c.check("fail", module+": go vet found problems (see above)")
			continue
		}
		var env []string
		if !isToolModule(module) {
			// Test binaries load raylib and libffi when they start, so
			// .tools/raylib/ goes on the library search path.
			if _, err := c.syncRaylib(dir); err != nil {
				c.check("fail", module+": "+err.Error())
				continue
			}
			env = append(env, c.libraryPath(c.path(".tools", "raylib"), c.goEnv()))
		}
		if _, err := c.runGo(goCall{dir: dir, args: []string{"test", "./..."}, env: env}); err != nil {
			c.check("fail", module+": tests failed (see above)")
			continue
		}
		c.check("ok", module+": vet and tests passed")
	}
	return c.summary("test")
}
