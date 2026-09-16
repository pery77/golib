package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// test runs go vet and go test for every Go module in the project: the
// framework, each game and GoLib's own programs.
func (c *cli) test(options []string) int {
	if len(options) > 0 {
		return c.usage(fmt.Sprintf("test takes no options (got: %s)", strings.Join(options, " ")))
	}
	modules := c.modules()
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
		if !slices.Contains(toolModules, module) {
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
