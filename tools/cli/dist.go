package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// dist builds a game for players (see docs/tooling.md#dist-builds): a folder
// in build/<game>/dist/ with the executable, the libraries it loads and the
// licenses of the third-party software in it, and a zip of that folder to
// share. On Windows the executable carries the game's icon and version
// information.
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
	// Windows and Linux games load raylib and libffi from the executable's
	// folder, as debug builds do. macOS looks for them in system folders and
	// the working directory only, so there the executable carries them and
	// copies them into the player's cache folder, as raylib-go and ffi do by
	// default.
	beside := c.goos != "darwin"
	tags := "golib_dist"
	if beside {
		tags += ",raylib_no_embed,ffi_no_embed"
	}

	packages, err := c.listPackages(dir, tags)
	if err != nil {
		c.check("fail", "could not inspect "+shown+" (see the Go errors above)")
		return false
	}
	if isDir(filepath.Join(dir, "assets")) && !embedsAssets(packages) {
		// A game embeds its assets from assets.go (see golib.EmbedAssets).
		// Without it, the executable would build fine and fail on the
		// player's machine.
		c.check("fail", shown+"/assets/ would be missing from the dist build: add "+shown+"/assets.go, as the golib.EmbedAssets documentation shows")
		return false
	}

	distDir := c.path("build", game, "dist")
	folder := filepath.Join(distDir, game)
	shownFolder := "build/" + game + "/dist/" + game + "/"
	exe := c.executable(game)
	if err := os.RemoveAll(distDir); err != nil {
		c.check("fail", fmt.Sprintf("cannot empty build/%s/dist/ (is the game still running?): %v", game, err))
		return false
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		c.check("fail", fmt.Sprintf("cannot create %s: %v", shownFolder, err))
		return false
	}

	info, ok := c.buildExecutable(game, tags, filepath.Join(folder, exe))
	if !ok {
		return false
	}
	c.check("ok", fmt.Sprintf("built %s into %s%s", shown, shownFolder, exe))

	modules := thirdPartyModules(packages)
	libraries, err := findLibraries(c.goos+"/"+c.goarch, modules)
	if err != nil {
		c.check("fail", "cannot find the libraries the game loads: "+err.Error())
		return false
	}
	var names []string
	for _, l := range libraries {
		names = append(names, l.name)
	}
	if beside {
		for _, l := range libraries {
			if err := l.copyTo(folder); err != nil {
				c.check("fail", fmt.Sprintf("cannot copy %s next to the executable: %v", l.name, err))
				return false
			}
		}
		c.check("ok", fmt.Sprintf("copied %s next to it: the game loads them when it starts", joinWords(names)))
		if !slices.ContainsFunc(libraries, func(l library) bool { return l.from.Path == ffiModule }) && c.goos != "linux" {
			c.check("warn", fmt.Sprintf("%s ships no libffi for %s/%s, so the game stops as soon as it starts (see docs/roadmap.md)", ffiModule, c.goos, c.goarch))
		}
	}

	notices := c.gameNotices(game, exe, modules, libraries, beside)
	text, err := thirdPartyNotices(info.Title, notices)
	if err == nil {
		err = os.WriteFile(filepath.Join(folder, noticesFile), text, 0o644)
	}
	if err != nil {
		c.check("fail", "cannot write "+noticesFile+": "+err.Error())
		return false
	}
	var titles []string
	for _, n := range notices {
		titles = append(titles, n.title)
	}
	c.check("ok", fmt.Sprintf("wrote %s next to it, with the licenses of %s", noticesFile, joinWords(titles)))

	osName := c.goos
	if osName == "darwin" {
		osName = "macos"
	}
	zipName := fmt.Sprintf("%s-%s-%s-%s.zip", game, info.Version, osName, c.goarch)
	size, err := zipFolder(folder, filepath.Join(distDir, zipName))
	if err != nil {
		c.check("fail", "cannot zip "+shownFolder+": "+err.Error())
		return false
	}
	c.check("ok", fmt.Sprintf("zipped %s into build/%s/dist/%s (%.1f MB): share this file", shownFolder, game, zipName, float64(size)/1e6))

	switch c.goos {
	case "windows":
		c.check("info", fmt.Sprintf("players unzip it and start %s, which needs the files next to it and writes nothing to their machine", exe))
	case "linux":
		c.check("info", fmt.Sprintf("players unzip it and start %s, which needs the files next to it, and libX11.so.6, libGL.so.1 and libffi.so.8 from their system", exe))
	default:
		c.check("info", fmt.Sprintf("players unzip it and start %s. On macOS it carries %s inside, and copies them into the player's ~/Library/Caches folder when it first starts", exe, joinWords(names)))
	}
	return true
}

// buildExecutable builds game's executable into output with the build tags
// tags, and returns what game.json says. On Windows it adds the game's
// icon and version information. It returns false after reporting a failure.
func (c *cli) buildExecutable(game, tags, output string) (gameInfo, bool) {
	dir := c.path("games", game)
	ldflags := "-s -w"
	var info gameInfo
	var ok bool
	switch c.goos {
	case "windows":
		// -H=windowsgui makes a program that opens no console window.
		ldflags += " -H=windowsgui"
		// Go links the .syso files in a package's folder into the program,
		// and only from there, so the resources sit next to main.go for the
		// length of the build. .gitignore lists the name.
		resources := filepath.Join(dir, "golib_dist_windows_"+c.goarch+".syso")
		defer func() {
			if err := os.Remove(resources); err != nil && !errors.Is(err, fs.ErrNotExist) {
				c.check("warn", fmt.Sprintf("could not delete games/%s/%s after the build: delete it by hand (%v)", game, filepath.Base(resources), err))
			}
		}()
		info, ok = c.writeWindowsResources(game, resources)
	case "linux":
		// The dynamic linker looks for the libraries in the executable's
		// folder too.
		ldflags += " -r $ORIGIN"
		info, ok = c.checkGameFiles(game)
	default:
		info, ok = c.checkGameFiles(game)
	}
	if !ok {
		return info, false
	}

	// -tags replaces raylib_no_embed and ffi_no_embed from GOFLAGS, so dist
	// builds embed both libraries unless tags names them again.
	err := c.goRun(dir, "build", "-trimpath", "-tags="+tags, "-ldflags="+ldflags, "-o", output, ".")
	if err != nil {
		c.check("fail", "dist build failed for games/"+game+" (see the Go errors above)")
		return info, false
	}
	return info, true
}

// gameNotices returns what THIRD-PARTY-LICENSES.txt lists for game, whose
// executable is called exe: Go, the modules, the libraries, which are beside
// the executable or inside it, and the files that the game's
// assets/ATTRIBUTION.md lists.
func (c *cli) gameNotices(game, exe string, modules []goModule, libraries []library, beside bool) []notice {
	goVersion := "(unknown version)"
	if data, err := os.ReadFile(c.path(".tools", "go", "VERSION")); err == nil {
		first, _, _ := strings.Cut(string(data), "\n")
		goVersion = strings.TrimPrefix(strings.TrimSpace(first), "go")
	}
	notices := []notice{{
		title: "Go " + goVersion,
		url:   "https://go.dev",
		where: "Built into " + exe + ": the Go runtime and standard library",
		files: []string{c.path(".tools", "go", "LICENSE")},
	}}
	for _, m := range modules {
		files := moduleLicenseFiles(m.Dir)
		if len(files) == 0 {
			c.check("warn", fmt.Sprintf("%s has no license file in its folder: find its license and add its notice to %s by hand", m.Path, noticesFile))
		}
		notices = append(notices, notice{
			title: m.Path + " " + m.Version,
			url:   "https://pkg.go.dev/" + m.Path + "@" + m.Version,
			where: "Built into " + exe,
			files: files,
		})
	}
	for _, l := range libraries {
		where := l.name + ", next to " + exe
		if !beside {
			where = "Built into " + exe
		}
		notices = append(notices, notice{
			title: l.title,
			url:   l.url,
			where: fmt.Sprintf("%s, from %s %s", where, l.from.Path, l.from.Version),
			files: []string{l.license},
		})
	}
	// Games write where the files in their assets folder come from, and under
	// which license, in assets/ATTRIBUTION.md (see framework/README.md).
	if attribution := c.path("games", game, "assets", attributionFile); isFile(attribution) {
		notices = append(notices, notice{
			title: "assets/" + attributionFile,
			where: "Built into " + exe + ": files in the game's assets folder that were not made for it",
			files: []string{attribution},
		})
	}
	return notices
}

// writeWindowsResources writes the Windows resources of game, its icon from
// icon.png and the details Explorer shows from game.json, into the .syso file
// at path, reports what it found, and returns what game.json says. It
// returns false after reporting a failure.
func (c *cli) writeWindowsResources(game, path string) (gameInfo, bool) {
	info, icon, ok := c.readGameFiles(game, true)
	if !ok {
		return info, false
	}
	data, err := windowsResources(info, icon, game, c.goarch)
	if err == nil {
		err = os.WriteFile(path, data, 0o644)
	}
	if err != nil {
		c.check("fail", "cannot make the Windows resources of games/"+game+": "+err.Error())
		return info, false
	}
	return info, true
}

// checkGameFiles checks game.json and icon.png on platforms that don't use
// them yet, so a mistake shows up wherever the game is built, and returns
// what game.json says. It returns false after reporting a failure.
func (c *cli) checkGameFiles(game string) (gameInfo, bool) {
	info, _, ok := c.readGameFiles(game, false)
	return info, ok
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

// goPackage is what go list says about a package.
type goPackage struct {
	ImportPath    string
	DepOnly       bool // a dependency, rather than the package go list was asked about
	Module        *goModule
	EmbedPatterns []string
}

// listPackages returns the package in dir and every package it is built
// from, with the build tags tags.
func (c *cli) listPackages(dir, tags string) ([]goPackage, error) {
	output, err := c.goOutput(dir, "list", "-deps", "-tags="+tags, "-json=ImportPath,DepOnly,Module,EmbedPatterns", ".")
	if err != nil {
		return nil, err
	}
	var packages []goPackage
	decoder := json.NewDecoder(bytes.NewReader(output))
	for {
		var p goPackage
		err := decoder.Decode(&p)
		if errors.Is(err, io.EOF) {
			return packages, nil
		}
		if err != nil {
			fmt.Fprintln(c.stderr, "go list:", err)
			return nil, err
		}
		packages = append(packages, p)
	}
}

// embedsAssets reports whether the main package of packages embeds the
// game's assets folder.
func embedsAssets(packages []goPackage) bool {
	for _, p := range packages {
		if !p.DepOnly && (slices.Contains(p.EmbedPatterns, "assets") || slices.Contains(p.EmbedPatterns, "all:assets")) {
			return true
		}
	}
	return false
}

// thirdPartyModules returns the modules that packages come from, sorted by
// path, apart from the game's own and GoLib's: GoLib's license, zlib, asks
// for no notice in games.
func thirdPartyModules(packages []goPackage) []goModule {
	var modules []goModule
	for _, p := range packages {
		m := p.Module
		if m == nil || m.Main || m.Path == "golib" || slices.ContainsFunc(modules, func(seen goModule) bool { return seen.Path == m.Path }) {
			continue
		}
		modules = append(modules, *m)
	}
	slices.SortFunc(modules, func(a, b goModule) int { return strings.Compare(a.Path, b.Path) })
	return modules
}

// zipFolder writes the folder dir and everything in it into a new zip file
// at path, and returns the zip file's size.
func zipFolder(dir, path string) (int64, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return 0, err
	}
	archive := zip.NewWriter(file)
	top := filepath.Dir(dir)
	err = filepath.WalkDir(dir, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(top, name)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		if entry.IsDir() {
			header.Name += "/"
			_, err = archive.CreateHeader(header)
			return err
		}
		header.Method = zip.Deflate
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		content, err := os.Open(name)
		if err != nil {
			return err
		}
		defer content.Close()
		_, err = io.Copy(writer, content)
		return err
	})
	err = errors.Join(err, archive.Close())
	var size int64
	if err == nil {
		var stat os.FileInfo
		if stat, err = file.Stat(); err == nil {
			size = stat.Size()
		}
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		os.Remove(path)
		return 0, err
	}
	return size, nil
}

// joinWords joins words as a list in a sentence: "a", "a and b", "a, b and c".
func joinWords(words []string) string {
	if len(words) <= 1 {
		return strings.Join(words, "")
	}
	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
}
