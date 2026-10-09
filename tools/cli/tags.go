package main

import (
	"fmt"
	"slices"
	"strings"
)

// debugTags are the build tags of a debug build: it loads raylib and libffi
// from files golib provides. goEnv gives them to every go command through
// GOFLAGS, and a command given --tags names them again, since -tags replaces
// the tags in GOFLAGS.
const debugTags = "raylib_no_embed,ffi_no_embed"

// golibTags are the build tags GoLib sets itself, which --tags can't name.
var golibTags = []string{"golib_dist", "raylib_no_embed", "ffi_no_embed"}

// takeTags takes --tags and its list out of the options of command, one of
// the commands that build a game, and keeps the list in c.tags, for withTags.
// It returns the other options, or exit code 2 after explaining a usage
// mistake. The list is Go build tags, separated by commas, such as
// "demo,steam": a game's files can say //go:build demo, so one game builds
// in more than one way (see docs/tooling.md#build-tags).
func (c *cli) takeTags(command string, options []string) ([]string, int) {
	var rest []string
	given := false
	for i := 0; i < len(options); i++ {
		option := options[i]
		value := ""
		switch {
		case option == "--tags":
			if i+1 >= len(options) {
				return nil, c.usage(command + " --tags needs a list of build tags, such as: golib " + command + " --tags demo")
			}
			i++
			value = options[i]
		case strings.HasPrefix(option, "--tags="):
			value = strings.TrimPrefix(option, "--tags=")
		default:
			rest = append(rest, option)
			continue
		}
		if given {
			return nil, c.usage(command + " takes --tags once: list every tag in it, separated by commas, such as --tags demo,steam")
		}
		given = true
		var tags []string
		for _, tag := range strings.Split(value, ",") {
			if !validTag(tag) {
				return nil, c.usage(fmt.Sprintf("%s --tags takes build tags separated by commas, each made of letters, digits, _ and . (got: %q)", command, value))
			}
			if slices.Contains(golibTags, tag) {
				return nil, c.usage(fmt.Sprintf("%s --tags can't name %s: GoLib sets it itself (see docs/tooling.md#build-tags)", command, tag))
			}
			if !slices.Contains(tags, tag) {
				tags = append(tags, tag)
			}
		}
		c.tags = strings.Join(tags, ",")
	}
	return rest, 0
}

// validTag reports whether tag can be a build tag: letters, digits, _ and .
func validTag(tag string) bool {
	return tag != "" && strings.Trim(tag, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_.") == ""
}

// distOnlyTag returns a tag --tags gave that the game's game.json keeps to
// dist builds ("buildTags"), or "". A game.json with a mistake gives none:
// the build reports the mistake.
func (c *cli) distOnlyTag(game string) string {
	if c.tags == "" {
		return ""
	}
	info, _, err := readGameInfo(c.path("games", game))
	if err != nil {
		return ""
	}
	for _, tag := range strings.Split(c.tags, ",") {
		if info.BuildTags[tag] == tagDistOnly {
			return tag
		}
	}
	return ""
}

// refuseDistOnly reports, with a [fail] line, whether --tags names a tag the
// game keeps to dist builds, which command, a debug build, doesn't make.
func (c *cli) refuseDistOnly(command, game string) bool {
	tag := c.distOnlyTag(game)
	if tag == "" {
		return false
	}
	c.check("fail", fmt.Sprintf("games/%s/%s keeps the build tag %s to dist builds, and %s makes a debug build: make it with golib dist %s --tags %s, or play it with golib run %s --dist --tags %s", game, gameInfoFile, tag, command, game, c.tags, game, c.tags))
	return true
}

// withTags returns the build tags base, GoLib's own for a build, with the
// ones --tags gave after them.
func (c *cli) withTags(base string) string {
	switch {
	case c.tags == "":
		return base
	case base == "":
		return c.tags
	}
	return base + "," + c.tags
}

// tagsArgs returns the -tags argument a debug build's go command needs for
// the tags --tags gave, with the debug build's own, or none without them,
// when GOFLAGS has the debug build's tags already.
func (c *cli) tagsArgs() []string {
	if c.tags == "" {
		return nil
	}
	return []string{"-tags=" + c.withTags(debugTags)}
}

// tagsInName returns what goes in the name of a zip made with the tags
// --tags gave, so that a demo's zip, say, is never taken for the full
// game's: "-demo" for --tags demo, "-demo-steam" for --tags demo,steam, and
// nothing without them.
func (c *cli) tagsInName() string {
	if c.tags == "" {
		return ""
	}
	return "-" + strings.ReplaceAll(c.tags, ",", "-")
}

// reportTags says which tags --tags gave, if any, before a command builds.
func (c *cli) reportTags() {
	if c.tags != "" {
		c.check("info", "building with the build tags "+c.tags+", from --tags")
	}
}
