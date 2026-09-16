// Cli is the part of the golib command written in Go. The golib scripts in
// tools/bootstrap/ build it into build/golib/ and start it for the commands
// that have moved here, passing their arguments on; nobody needs to start it
// by hand. Commands move here from the scripts one at a time (see
// docs/roadmap.md). So far: dist.
//
// It prints what the scripts print: one fact per line, starting with [ok],
// [info], [warn] or [fail], then a summary line. The exit code is 0 after
// success, 1 after a failure and 2 after a usage mistake, which it explains
// on standard error.
//
// It starts with the user's environment, and gives the go commands it runs
// GoLib's own (see project.goEnv). It uses only the standard library.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// commands are the golib commands implemented here, by name. The scripts
// start this program for these commands only, and list every command in
// their help.
var commands = map[string]func(c *cli, options []string) int{
	"dist": (*cli).dist,
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run runs the golib command in args and returns its exit code.
func run(args []string, stdout, stderr io.Writer) int {
	c := &cli{stdout: stdout, stderr: stderr}
	if len(args) == 0 {
		return c.usage("no command given")
	}
	command, found := commands[args[0]]
	if !found {
		return c.usage(fmt.Sprintf("unknown command %q", args[0]))
	}
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err == nil {
		c.project, err = newProject(exe, stdout, stderr)
	}
	if err != nil {
		fmt.Fprintln(stderr, "golib:", err)
		return 1
	}
	return command(c, args[1:])
}

// cli is what a command works with: the project, where its output goes, and
// how many failures and warnings it has reported.
type cli struct {
	*project
	stdout, stderr     io.Writer
	failures, warnings int
}

// check prints one fact. level is ok, info, warn or fail.
func (c *cli) check(level, message string) {
	switch level {
	case "warn":
		c.warnings++
	case "fail":
		c.failures++
	}
	fmt.Fprintf(c.stdout, "%-6s %s\n", "["+level+"]", message)
}

// summary prints the summary line of command, and returns its exit code: 1
// after a failure, 0 otherwise.
func (c *cli) summary(command string) int {
	fmt.Fprintf(c.stdout, "\n%s: %d failed, %d warning(s)\n", command, c.failures, c.warnings)
	if c.failures > 0 {
		return 1
	}
	return 0
}

// usage explains a usage mistake on standard error, and returns exit code 2.
func (c *cli) usage(message string) int {
	fmt.Fprintf(c.stderr, "golib: %s\nRun \"golib help\" for usage.\n", message)
	return 2
}
