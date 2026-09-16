package main

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// setup finishes golib setup once the scripts have checked the machine and
// installed Go: it downloads the Go modules of the framework and every game,
// and fills .tools/raylib/. The scripts pass --warnings=N, the number of
// warnings they printed, so the summary counts them; they stop before this
// after a failure. People never pass options: the scripts refuse them.
func (c *cli) setup(options []string) int {
	for _, option := range options {
		count, found := strings.CutPrefix(option, "--warnings=")
		warnings, err := strconv.Atoi(count)
		if !found || err != nil || warnings < 0 {
			return c.usage(fmt.Sprintf("setup takes no options (got: %s)", strings.Join(options, " ")))
		}
		c.warnings += warnings
	}

	var modules []string
	for _, module := range c.modules() {
		if !isToolModule(module) {
			modules = append(modules, module)
		}
	}
	if len(modules) == 0 {
		c.check("info", "no Go modules yet: nothing more to download")
	}
	for _, module := range modules {
		dir := c.path(filepath.FromSlash(module))
		if err := c.goRun(dir, "mod", "download"); err != nil {
			c.check("fail", fmt.Sprintf("could not download the Go modules for %s (see the errors above)", module))
			continue
		}
		synced, err := c.syncRaylib(dir)
		if err != nil {
			c.check("fail", module+": "+err.Error())
			continue
		}
		c.check("ok", fmt.Sprintf("%s: modules downloaded, raylib library for raylib-go %s in .tools/raylib/", module, synced.version))
	}

	exitCode := c.summary("setup")
	if exitCode != 0 {
		fmt.Fprintln(c.stdout, "Setup incomplete. Fix the [fail] items above, then run setup again.")
	} else {
		fmt.Fprintln(c.stdout, "Setup complete.")
	}
	return exitCode
}
