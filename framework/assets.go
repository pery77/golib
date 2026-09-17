package golib

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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
// asset shows up on the next run. A debug build started from Explorer runs in
// build/<game>/, which has no assets folder, so it reads games/<game>/assets.
// golib dist builds read the copy embedded in the executable instead: see
// EmbedAssets.
func ReadAsset(name string) ([]byte, error) {
	files, where, err := assetSource(embeddedAssets, distBuild)
	if err != nil {
		return nil, err
	}
	return readAsset(files, where, name)
}

// EmbedAssets makes ReadAsset read from files, the game's assets folder
// embedded in the executable, instead of from disk. A golib dist build carries
// the assets inside its executable, so every game with an assets folder embeds
// it from a file named assets.go, next to main.go, with exactly this content:
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
	workDir, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("golib.ReadAsset: cannot find the working directory: %w", err)
	}
	exe, err := os.Executable()
	if err != nil {
		exe = "" // only the working directory counts
	}
	dir := debugGameDir(workDir, exe, isDir)
	return os.DirFS(dir), "in " + dir, nil
}

// debugGameDir returns the folder whose assets folder a debug build reads: the
// working directory, which golib run, golib shot, F5 and go test set to the
// game's folder. A debug build started another way, such as a double click on
// build/<game>/<game>.exe, runs somewhere else: when the working directory has
// no assets folder, it reads the one in games/<game>/, found from the
// executable's place in build/<game>/, if there is one.
func debugGameDir(workDir, exe string, isDir func(string) bool) string {
	if exe == "" || isDir(filepath.Join(workDir, assetsDir)) {
		return workDir
	}
	buildGameDir := filepath.Dir(exe)
	if filepath.Base(filepath.Dir(buildGameDir)) != "build" {
		return workDir
	}
	gameDir := filepath.Join(buildGameDir, "..", "..", "games", filepath.Base(buildGameDir))
	if !isDir(filepath.Join(gameDir, assetsDir)) {
		return workDir
	}
	return gameDir
}

// isDir reports whether path is a folder.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
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

// ListAssets returns the names of the files in a folder of the game's assets
// folder, sorted, such as every level in maps/. The names are as ReadAsset
// takes them, folder and all ("maps/level1.tmx"); folders inside are left out.
// Pass "" for the assets folder itself.
//
//	levels, err := golib.ListAssets("maps") // games/<game>/assets/maps/
//
// It reads the same files as ReadAsset: those on disk in a debug build, and
// those embedded in the executable in a golib dist build, which are the files
// that were in the assets folder when the game was built.
func ListAssets(folder string) ([]string, error) {
	files, where, err := assetSource(embeddedAssets, distBuild)
	if err != nil {
		return nil, err
	}
	return listAssets(files, where, folder)
}

// listAssets lists the files in assets/<folder> of files. where says where
// files are, for error messages.
func listAssets(files fs.FS, where, folder string) ([]string, error) {
	path := assetsDir
	if folder != "" {
		if !fs.ValidPath(folder) || folder == "." || strings.Contains(folder, `\`) {
			return nil, fmt.Errorf("golib.ListAssets: invalid folder %q: use a path relative to the assets folder, with forward slashes, such as \"maps\"", folder)
		}
		path += "/" + folder
	}
	entries, err := fs.ReadDir(files, path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("golib.ListAssets: %s not found %s: folders are relative to the game's assets folder", path, where)
	}
	if err != nil {
		return nil, fmt.Errorf("golib.ListAssets: cannot read %s %s: %w", path, where, err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if folder != "" {
			name = folder + "/" + name
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}
