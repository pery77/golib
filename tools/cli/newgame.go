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
	// module paths and executable names. A leading _ marks a private game,
	// which .gitignore keeps out of this repository (see isPrivateGame).
	gameNamePattern = regexp.MustCompile(`^_?[a-z][a-z0-9_-]{0,31}$`)
	// reservedGameName matches golib, which would clash with the
	// framework's import path and build/golib/, and the names Windows keeps
	// for devices.
	reservedGameName = regexp.MustCompile(`^(golib|con|prn|aux|nul|com[1-9]|lpt[1-9])$`)
)

// newGame creates games/<name>/ from the templates in tools/template/game/
// (see docs/tooling.md#new-games).
func (c *cli) newGame(options []string) int {
	if len(options) != 1 {
		return c.usage("new needs one game name, for example: golib new asteroids")
	}
	name := options[0]
	if !gameNamePattern.MatchString(name) {
		return c.usage(fmt.Sprintf("invalid game name %q: use 1 to 32 lowercase letters, digits, - and _, starting with a letter, or with _ for a private game", name))
	}
	if reservedGameName.MatchString(name) {
		return c.usage(fmt.Sprintf("the game name %q is reserved: pick another one", name))
	}
	dir := c.path("games", name)
	if _, err := os.Lstat(dir); err == nil {
		return c.usage(fmt.Sprintf("games/%s already exists: pick another name, or delete that folder first", name))
	}

	if err := c.writeTemplates(name, dir); err != nil {
		os.RemoveAll(dir)
		c.check("fail", fmt.Sprintf("cannot create games/%s/: %v", name, err))
		return c.summary("new")
	}
	if err := c.goRun(dir, "mod", "tidy"); err != nil {
		os.RemoveAll(dir)
		c.check("fail", fmt.Sprintf("go mod tidy failed for games/%s (see the Go errors above); the folder was deleted, so new can run again", name))
		return c.summary("new")
	}
	c.check("ok", fmt.Sprintf("created games/%s/ from tools/template/game/", name))
	if isPrivateGame(name) {
		c.check("info", fmt.Sprintf("games/%s/ is a private game: .gitignore keeps games/_*/ out of this repository, so it can have a repository of its own", name))
	}
	c.check("info", fmt.Sprintf("next: golib run %s, and describe the game in games/%s/DESIGN.md", name, name))
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
