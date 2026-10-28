package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	// gameNamePattern is what game names look like: they are folder names,
	// module paths and executable names.
	gameNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	// reservedGameName matches golib, which would clash with the
	// framework's import path and build/golib/, and the names Windows keeps
	// for devices.
	reservedGameName = regexp.MustCompile(`^(golib|con|prn|aux|nul|com[1-9]|lpt[1-9])$`)
)

// newGame creates games/<name>/ from the templates in tools/template/game/,
// or examples/<name>/ with --example, for GoLib's own examples (see
// docs/tooling.md#new-games).
func (c *cli) newGame(options []string) int {
	folder := gamesFolder
	var names []string
	for _, option := range options {
		switch {
		case option == "--example":
			folder = examplesFolder
		case strings.HasPrefix(option, "-"):
			return c.usage("unknown option for new: " + option)
		default:
			names = append(names, option)
		}
	}
	if len(names) != 1 {
		return c.usage("new needs one game name, for example: golib new moonshot")
	}
	name := names[0]
	if !gameNamePattern.MatchString(name) {
		return c.usage(fmt.Sprintf("invalid game name %q: use 1 to 32 lowercase letters, digits, - and _, starting with a letter", name))
	}
	if reservedGameName.MatchString(name) {
		return c.usage(fmt.Sprintf("the game name %q is reserved: pick another one", name))
	}
	// A game and an example can't share a name: build/<name>/ and the
	// commands go by the name alone.
	for _, taken := range []string{gamesFolder, examplesFolder} {
		if _, err := os.Lstat(c.path(taken, name)); err == nil {
			return c.usage(fmt.Sprintf("%s/%s already exists: pick another name, or delete that folder first", taken, name))
		}
	}
	shown := folder + "/" + name
	dir := c.path(folder, name)

	if err := c.writeTemplates(name, dir); err != nil {
		os.RemoveAll(dir)
		c.check("fail", fmt.Sprintf("cannot create %s/: %v", shown, err))
		return c.summary("new")
	}
	if err := c.goRun(dir, "mod", "tidy"); err != nil {
		os.RemoveAll(dir)
		c.check("fail", fmt.Sprintf("go mod tidy failed for %s (see the Go errors above); the folder was deleted, so new can run again", shown))
		return c.summary("new")
	}
	c.check("ok", fmt.Sprintf("created %s/ from tools/template/game/", shown))
	if folder == gamesFolder {
		c.check("info", fmt.Sprintf("%s/ stays out of GoLib's repository, as all of games/ does: to keep its history, give it a git repository of its own (git init in its folder)", shown))
	}
	c.check("info", fmt.Sprintf("next: golib run %s, and describe the game in %s/DESIGN.md", name, shown))
	return c.summary("new")
}

// writeTemplates writes the game called name into dir: each *.tmpl file in
// tools/template/game/ without its suffix, with {{name}}, {{go}} and
// {{date}} replaced, and the framework's go.sum, whose checksums cover the
// modules a new game needs, so go mod tidy doesn't have to look them up.
func (c *cli) writeTemplates(name, dir string) error {
	goVersion, err := c.goVersion()
	if err != nil {
		return err
	}
	templates, err := filepath.Glob(c.path("tools", "template", "game", "*.tmpl"))
	if err != nil || len(templates) == 0 {
		return fmt.Errorf("tools/template/game/ has no *.tmpl files")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	replacer := strings.NewReplacer("{{name}}", name, "{{go}}", goVersion, "{{date}}", time.Now().Format("2006-01-02"))
	for _, template := range templates {
		content, err := os.ReadFile(template)
		if err != nil {
			return err
		}
		target := filepath.Join(dir, strings.TrimSuffix(filepath.Base(template), ".tmpl"))
		if err := os.WriteFile(target, []byte(replacer.Replace(string(content))), 0o644); err != nil {
			return err
		}
	}
	return copyFile(c.path("framework", "go.sum"), filepath.Join(dir, "go.sum"))
}

// goVersion returns the version of the Go toolchain in .tools/go/, such as
// 1.27.1. The scripts check that it is the version GoLib pins before they
// start this program.
func (p *project) goVersion() (string, error) {
	data, err := os.ReadFile(p.path(".tools", "go", "VERSION"))
	if err != nil {
		return "", fmt.Errorf("cannot read the Go version (run: golib setup): %w", err)
	}
	first, _, _ := strings.Cut(string(data), "\n")
	return strings.TrimPrefix(strings.TrimSpace(first), "go"), nil
}
