package main

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// Every command that builds a game takes --tags, and builds with those tags
// after GoLib's own: a debug build names them again, since -tags replaces
// the ones GOFLAGS gives.
func TestTagsReachEveryBuild(t *testing.T) {
	const debug = "-tags=raylib_no_embed,ffi_no_embed,demo,steam"
	tests := []struct {
		name    string
		goos    string
		command func(c *cli, options []string) int
		options []string
		want    []string // the first go call with the tags, without "./..." or "."
	}{
		{"build", "windows", (*cli).build, []string{"--tags", "demo,steam"}, []string{"build", debug}},
		{"run", "windows", (*cli).run, []string{"--tags=demo,steam", "rocks"}, []string{"build", debug}},
		{"shot", "windows", (*cli).shot, []string{"30", "--tags", "demo,steam,demo"}, []string{"build", debug}},
		{"test", "windows", (*cli).test, []string{"rocks", "--tags", "demo,steam"}, []string{"vet", debug}},
		{"dist", "windows", (*cli).dist, []string{"--tags", "demo,steam"}, []string{"list", "-deps", "-tags=golib_dist,raylib_no_embed,ffi_no_embed,demo,steam"}},
		{"dist on macOS", "darwin", (*cli).dist, []string{"--tags", "demo,steam"}, []string{"list", "-deps", "-tags=golib_dist,demo,steam"}},
		{"run --dist", "linux", (*cli).run, []string{"--dist", "--tags", "demo,steam"}, []string{"list", "-deps", "-tags=golib_dist,raylib_no_embed,ffi_no_embed,demo,steam"}},
	}
	for _, tt := range tests {
		tp := newTestProject(t, tt.goos, "rocks")
		if tt.goos == "darwin" {
			tp.addIcon(t, "rocks")
		}
		if code := tt.command(tp.c, tt.options); code != 0 {
			t.Errorf("%s: exit code %d, output:\n%s%s", tt.name, code, tp.stdout.String(), tp.stderr.String())
			continue
		}
		if !strings.HasPrefix(tp.stdout.String(), "[info] building with the build tags demo,steam, from --tags\n") {
			t.Errorf("%s: output:\n%s\nwant it to start by naming the tags", tt.name, tp.stdout.String())
		}
		var tagged [][]string
		for _, call := range tp.calls {
			if slices.ContainsFunc(call.args, func(arg string) bool { return strings.HasPrefix(arg, "-tags=") }) {
				tagged = append(tagged, call.args)
			}
		}
		if len(tagged) == 0 || !slices.Equal(tagged[0][:len(tt.want)], tt.want) {
			t.Errorf("%s: go calls with tags %q, want the first to start with %q", tt.name, tagged, tt.want)
		}
		// Every build and test is made with the tags, not only the first.
		for _, call := range tp.calls {
			if (call.args[0] == "build" || call.args[0] == "test" || call.args[0] == "vet") && !slices.ContainsFunc(call.args, func(arg string) bool { return strings.HasSuffix(arg, ",demo,steam") }) {
				t.Errorf("%s: go %q left out the tags", tt.name, call.args)
			}
		}
	}

	// Without --tags, debug builds leave the tags to GOFLAGS, as before.
	tp := newTestProject(t, "windows", "rocks")
	if code := tp.c.build(nil); code != 0 || strings.Contains(tp.stdout.String(), "tags") {
		t.Errorf("build without --tags: exit code %d, output:\n%s", code, tp.stdout.String())
	}
	if builds := tp.builds(); len(builds) != 1 || !slices.Equal(builds[0], []string{"build", "-o", tp.c.path("build", "rocks", "rocks.exe"), "."}) {
		t.Errorf("build without --tags: go %q", builds)
	}
}

// A zip made with --tags says so in its name, so that a demo's zip is never
// taken for the full game's.
func TestTagsNameTheZip(t *testing.T) {
	tp := newTestProject(t, "windows", "rocks")
	writeFile(t, tp.c.path("games", "rocks", gameInfoFile), `{"title": "Rocks", "version": "1.0.0"}`)
	if code := tp.c.dist([]string{"rocks", "--tags", "demo"}); code != 0 {
		t.Fatalf("exit code %d, output:\n%s%s", code, tp.stdout.String(), tp.stderr.String())
	}
	if _, err := os.Stat(tp.c.path("build", "rocks", "dist", "rocks-1.0.0-demo-windows-amd64.zip")); err != nil {
		t.Errorf("dist --tags demo: %v\noutput:\n%s", err, tp.stdout.String())
	}

	tp = webProject(t, "rocks")
	writeFile(t, tp.c.path("games", "rocks", gameInfoFile), `{"title": "Rocks", "version": "1.2.0"}`)
	if code := tp.c.dist([]string{"--web", "--tags", "demo,itch"}); code != 0 {
		t.Fatalf("exit code %d, output:\n%s%s", code, tp.stdout.String(), tp.stderr.String())
	}
	if _, err := os.Stat(tp.c.path("build", "rocks", "dist", "rocks-1.2.0-demo-itch-web.zip")); err != nil {
		t.Errorf("dist --web --tags demo,itch: %v\noutput:\n%s", err, tp.stdout.String())
	}
	for _, call := range tp.calls {
		if (call.args[0] == "build" || call.args[0] == "list") && !slices.Contains(call.args, "-tags=golib_dist,demo,itch") {
			t.Errorf("dist --web --tags demo,itch: go %q", call.args)
		}
	}
}

func TestTagsUsage(t *testing.T) {
	tests := []struct {
		command string
		options []string
		want    string
	}{
		{"build", []string{"--tags"}, "build --tags needs a list of build tags, such as: golib build --tags demo"},
		{"dist", []string{"--tags="}, `dist --tags takes build tags separated by commas, each made of letters, digits, _ and . (got: "")`},
		{"run", []string{"--tags", "demo,,steam"}, `run --tags takes build tags separated by commas, each made of letters, digits, _ and . (got: "demo,,steam")`},
		{"shot", []string{"--tags", "demo steam"}, `shot --tags takes build tags separated by commas, each made of letters, digits, _ and . (got: "demo steam")`},
		{"web", []string{"--tags", "!demo"}, `web --tags takes build tags separated by commas, each made of letters, digits, _ and . (got: "!demo")`},
		{"test", []string{"--tags", "demo,golib_dist"}, "test --tags can't name golib_dist: GoLib sets it itself (see docs/tooling.md#build-tags)"},
		{"build", []string{"--tags", "raylib_no_embed"}, "build --tags can't name raylib_no_embed: GoLib sets it itself (see docs/tooling.md#build-tags)"},
		{"dist", []string{"--tags", "demo", "--tags=steam"}, "dist takes --tags once: list every tag in it, separated by commas, such as --tags demo,steam"},
		{"build", []string{"--tags", "demo", "rocks", "snake"}, "build takes at most one game name (got: rocks snake)"},
	}
	for _, tt := range tests {
		tp := newTestProject(t, "windows", "rocks", "snake")
		if code := commands[tt.command](tp.c, tt.options); code != 2 {
			t.Errorf("%s %q: exit code %d, want 2", tt.command, tt.options, code)
		}
		if want := "golib: " + tt.want + "\nRun \"golib help\" for usage.\n"; tp.stderr.String() != want {
			t.Errorf("%s %q: stderr %q, want %q", tt.command, tt.options, tp.stderr.String(), want)
		}
		if len(tp.calls) > 0 || len(tp.games) > 0 || tp.stdout.Len() > 0 {
			t.Errorf("%s %q: go or the game ran, or a check was printed, after a usage mistake:\n%s", tt.command, tt.options, tp.stdout.String())
		}
	}
}

// A tag game.json keeps to dist builds stops build and run without --dist,
// before anything is built, saying how to make that build instead; dist,
// run --dist, shot and test still take it.
func TestDistOnlyTags(t *testing.T) {
	for _, tt := range []struct {
		name    string
		command func(c *cli, options []string) int
		options []string
		refused bool
	}{
		{"build", (*cli).build, []string{"--tags", "steam,demo"}, true},
		{"run", (*cli).run, []string{"--tags", "demo"}, true},
		{"run --dist", (*cli).run, []string{"--dist", "--tags", "demo"}, false},
		{"dist", (*cli).dist, []string{"--tags", "demo"}, false},
		{"shot", (*cli).shot, []string{"--tags", "demo"}, false},
		{"test", (*cli).test, []string{"rocks", "--tags", "demo"}, false},
		{"build, another tag", (*cli).build, []string{"--tags", "steam"}, false},
	} {
		tp := newTestProject(t, "windows", "rocks")
		writeFile(t, tp.c.path("games", "rocks", gameInfoFile), `{"title": "Rocks", "buildTags": {"demo": "dist", "steam": "any"}}`)
		code := tt.command(tp.c, tt.options)
		refusal := "[fail] games/rocks/game.json keeps the build tag demo to dist builds"
		if !tt.refused {
			if code != 0 || strings.Contains(tp.stdout.String(), refusal) {
				t.Errorf("%s: exit code %d, output:\n%s%s", tt.name, code, tp.stdout.String(), tp.stderr.String())
			}
			continue
		}
		if code != 1 || !strings.Contains(tp.stdout.String(), refusal) || !strings.Contains(tp.stdout.String(), "golib dist rocks --tags") {
			t.Errorf("%s: exit code %d, output:\n%s", tt.name, code, tp.stdout.String())
		}
		if len(tp.builds()) > 0 || len(tp.games) > 0 {
			t.Errorf("%s: built %q or ran the game after refusing", tt.name, tp.builds())
		}
	}
}
