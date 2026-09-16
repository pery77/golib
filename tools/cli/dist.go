package main

import (
	"errors"
	"fmt"
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// dist builds a game for players (see docs/tooling.md#dist-builds): one
// executable in build/<game>/dist/, with raylib, libffi and the game's assets
// inside and, on Windows, its icon and version information.
func (c *cli) dist(options []string) int {
	game, exitCode := c.resolveGame("dist", options)
	if game == "" {
		return exitCode
	}
	c.distGame(game)
	return c.summary("dist")
}

// distGame makes the dist build of game, reporting each step. It returns
// false after reporting a failure.
func (c *cli) distGame(game string) bool {
	dir := c.path("games", game)
	shown := "games/" + game // how messages name the game's folder
	if isDir(filepath.Join(dir, "assets")) {
		// A game embeds its assets from assets.go (see golib.EmbedAssets).
		// Without it, the executable would build fine and fail on the
		// player's machine.
		patterns, err := c.goOutput(dir, "list", "-tags=golib_dist", "-f", "{{range .EmbedPatterns}}{{println .}}{{end}}", ".")
		if err != nil {
			c.check("fail", "could not inspect "+shown+" (see the Go errors above)")
			return false
		}
		lines := strings.Fields(string(patterns))
		if !slices.Contains(lines, "assets") && !slices.Contains(lines, "all:assets") {
			c.check("fail", shown+"/assets/ would be missing from the dist build: add "+shown+"/assets.go, as the golib.EmbedAssets documentation shows")
			return false
		}
	}

	outDir := c.path("build", game, "dist")
	exe := c.executable(game)
	if err := os.RemoveAll(outDir); err != nil {
		c.check("fail", fmt.Sprintf("cannot empty build/%s/dist/ (is the game still running?): %v", game, err))
		return false
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		c.check("fail", fmt.Sprintf("cannot create build/%s/dist/: %v", game, err))
		return false
	}

	ldflags := "-s -w"
	if c.goos == "windows" {
		// -H=windowsgui makes a program that opens no console window.
		ldflags += " -H=windowsgui"
		// Go links the .syso files in a package's folder into the program,
		// and only from there, so the resources sit next to main.go for the
		// length of the build. .gitignore lists the name.
		resources := filepath.Join(dir, "golib_dist_windows_"+c.goarch+".syso")
		if !c.writeWindowsResources(game, resources) {
			return false
		}
		defer func() {
			if err := os.Remove(resources); err != nil && !errors.Is(err, fs.ErrNotExist) {
				c.check("warn", fmt.Sprintf("could not delete %s/%s after the build: delete it by hand (%v)", shown, filepath.Base(resources), err))
			}
		}()
	} else if !c.checkGameFiles(game) {
		return false
	}

	// -tags replaces raylib_no_embed and ffi_no_embed from GOFLAGS, so both
	// libraries are embedded.
	err := c.goRun(dir, "build", "-trimpath", "-tags=golib_dist", "-ldflags="+ldflags, "-o", filepath.Join(outDir, exe), ".")
	if err != nil {
		c.check("fail", "dist build failed for "+shown+" (see the Go errors above)")
		return false
	}
	c.check("ok", fmt.Sprintf("built %s into build/%s/dist/%s", shown, game, exe))
	switch c.goos {
	case "windows":
		c.check("info", "one file with raylib, libffi and the assets inside; it copies raylib and libffi into the player's %LOCALAPPDATA% folder when it first starts")
	case "darwin":
		c.check("info", "one file with raylib, libffi and the assets inside; it copies raylib and libffi into the player's ~/Library/Caches folder when it first starts")
	default:
		c.check("info", "one file with raylib and the assets inside; it copies raylib into the player's ~/.cache folder when it first starts. Players need libX11.so.6, libGL.so.1 and libffi.so.8")
	}
	return true
}

// writeWindowsResources writes the Windows resources of game, its icon from
// icon.png and the details Explorer shows from game.json, into the .syso file
// at path, and reports what it found. It returns false after reporting a
// failure.
func (c *cli) writeWindowsResources(game, path string) bool {
	info, icon, ok := c.readGameFiles(game, true)
	if !ok {
		return false
	}
	data, err := windowsResources(info, icon, game, c.goarch)
	if err == nil {
		err = os.WriteFile(path, data, 0o644)
	}
	if err != nil {
		c.check("fail", "cannot make the Windows resources of games/"+game+": "+err.Error())
		return false
	}
	return true
}

// checkGameFiles checks game.json and icon.png on platforms that don't use
// them yet, so a mistake shows up wherever the game is built. It returns
// false after reporting a failure.
func (c *cli) checkGameFiles(game string) bool {
	_, _, ok := c.readGameFiles(game, false)
	return ok
}

// readGameFiles reads game.json and icon.png from game's folder. With
// describe, it reports what the executable's details and icon will be;
// without, it only says that this platform leaves them out. icon is nil when
// the game has no icon.png. ok is false after reporting a failure.
func (c *cli) readGameFiles(game string, describe bool) (info gameInfo, icon *image.NRGBA, ok bool) {
	dir := c.path("games", game)
	shown := "games/" + game

	info, infoFound, err := readGameInfo(dir)
	if err != nil {
		c.check("fail", fmt.Sprintf("%s/%s: %v", shown, gameInfoFile, err))
		return info, nil, false
	}
	if describe {
		details := fmt.Sprintf("%q, version %s", info.Title, info.Version)
		if info.Author != "" {
			details += ", by " + info.Author
		}
		if infoFound {
			c.check("ok", fmt.Sprintf("%s/%s: the executable's details say %s", shown, gameInfoFile, details))
		} else {
			c.check("info", fmt.Sprintf("%s has no %s, so the executable's details say %s: add one to set the title, version and author (see docs/tooling.md)", shown, gameInfoFile, details))
		}
	}

	icon, iconFound, err := readIcon(filepath.Join(dir, iconFile))
	if err != nil {
		c.check("fail", fmt.Sprintf("%s/%s: %v", shown, iconFile, err))
		return info, nil, false
	}
	switch {
	case describe && iconFound:
		size := icon.Bounds().Dx()
		c.check("ok", fmt.Sprintf("%s/%s (%d by %d pixels): the game's icon, in %d sizes from %d to %d pixels", shown, iconFile, size, size, len(iconSizes), iconSizes[0], iconSizes[len(iconSizes)-1]))
	case describe:
		c.check("info", fmt.Sprintf("%s has no %s, so the game shows Windows' default icon: add a square PNG, ideally 256 by 256 pixels", shown, iconFile))
	case infoFound || iconFound:
		c.check("info", "only Windows builds carry the icon from icon.png and the details from game.json so far")
	}
	return info, icon, true
}

// windowsResources returns a .syso file with the Windows resources of the
// game called name, built for arch processors: the version information from
// info, and the icon, unless icon is nil.
func windowsResources(info gameInfo, icon *image.NRGBA, name, arch string) ([]byte, error) {
	target, found := targets[arch]
	if !found {
		return nil, fmt.Errorf("GoLib can't make Windows resources for %q processors: use amd64 or arm64", arch)
	}
	resources := []resource{{kind: rtVersion, id: 1, data: versionInfo(info, name)}}
	if icon != nil {
		resources = append(resources, iconResources(icon)...)
	}
	section, addressFields := resourceSection(resources)
	return objectFile(target, section, addressFields), nil
}
