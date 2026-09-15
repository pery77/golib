package golib

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// assetsDir is the folder, inside the game's folder, that holds its assets.
const assetsDir = "assets"

// embeddedAssets holds the files passed to EmbedAssets, rooted at the game's
// folder, or nil.
var embeddedAssets fs.FS

// ReadAsset returns the contents of a file in the game's assets folder. name
// is relative to that folder and uses forward slashes:
//
//	data, err := golib.ReadAsset("levels/intro.txt") // games/<game>/assets/levels/intro.txt
//
// Debug builds (golib build, run, shot and test) read the file from disk, from
// the assets folder in the working directory. golib run and golib shot start
// the game in its folder, and go test runs a game's tests there, so an edited
// asset shows up on the next run. golib dist builds read the copy embedded in
// the executable instead: see EmbedAssets.
func ReadAsset(name string) ([]byte, error) {
	files, where, err := assetSource(embeddedAssets, distBuild)
	if err != nil {
		return nil, err
	}
	return readAsset(files, where, name)
}

// EmbedAssets makes ReadAsset read from files, the game's assets folder
// embedded in the executable, instead of from disk. A golib dist build ships
// as a single file, so every game with an assets folder embeds it from a file
// named assets.go, next to main.go, with exactly this content:
//
//	//go:build golib_dist
//
//	package main
//
//	import (
//		"embed"
//
//		"golib"
//	)
//
//	//go:embed all:assets
//	var assets embed.FS
//
//	func init() { golib.EmbedAssets(assets) }
//
// The golib_dist build tag leaves the file out of debug builds, which read
// assets from disk. golib dist stops with an error if a game has an assets
// folder but doesn't embed it.
func EmbedAssets(files embed.FS) {
	embeddedAssets = files
}

// assetSource returns the files ReadAsset reads from, rooted at the game's
// folder, and says where they are, for error messages.
func assetSource(embedded fs.FS, dist bool) (fs.FS, string, error) {
	if embedded != nil {
		return embedded, "in the executable", nil
	}
	if dist {
		return nil, "", errors.New("golib.ReadAsset: this golib dist build has no embedded assets: add assets.go to the game, as the golib.EmbedAssets documentation shows")
	}
	dir, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("golib.ReadAsset: cannot find the working directory: %w", err)
	}
	return os.DirFS(dir), "in " + dir, nil
}

// readAsset reads assets/<name> from files. where says where files are, for
// error messages.
func readAsset(files fs.FS, where, name string) ([]byte, error) {
	if name == "." || !fs.ValidPath(name) || strings.Contains(name, `\`) {
		return nil, fmt.Errorf("golib.ReadAsset: invalid asset name %q: use a path relative to the assets folder, with forward slashes, such as \"sprites/player.png\"", name)
	}
	path := assetsDir + "/" + name
	data, err := fs.ReadFile(files, path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("golib.ReadAsset: %s not found %s: asset names are relative to the game's assets folder", path, where)
	}
	if err != nil {
		return nil, fmt.Errorf("golib.ReadAsset: cannot read %s %s: %w", path, where, err)
	}
	return data, nil
}
