package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Steam's achievements, its language and the rest of framework/steam.go are
// Valve's Steamworks API, a library of Valve's that a game sold on Steam
// carries. Valve's license lets games ship the library and nothing else of
// the Steamworks SDK, so the SDK stays out of GoLib's repository: each
// developer downloads it from partner.steamgames.com and golib steam-sdk
// copies its libraries into .tools/steamworks/, as the SDK lays them out.
// The builds of a game made with the build tag steam carry this platform's,
// and the framework loads it from beside the executable. Without the tag a
// game neither carries nor needs it (see docs/tooling.md#steamworks).

// steamTag is the build tag that puts Steam in a game.
const steamTag = "steam"

// steamLibraries are the files golib steam-sdk takes from the SDK's
// redistributable_bin/ folder: the library of each platform GoLib builds
// for, in the SDK's own folder for it, which .tools/steamworks/ keeps.
var steamLibraries = []struct{ dir, name, platform string }{
	{"win64", "steam_api64.dll", "windows/amd64"},
	{"linux64", "libsteam_api.so", "linux/amd64"},
	{"linuxarm64", "libsteam_api.so", "linux/arm64"},
	{"osx", "libsteam_api.dylib", "darwin/amd64 and darwin/arm64"},
}

// steamVersionFile is the file of .tools/steamworks/ that says which SDK
// the libraries come from, such as "1.64".
const steamVersionFile = "VERSION"

// steamSDK copies the Steamworks API's libraries from the SDK at the path
// it is given, its zip or its sdk folder, into .tools/steamworks/, in place
// of what was there: how GoLib gets the SDK, and how it moves to a newer one.
func (c *cli) steamSDK(options []string) int {
	if len(options) != 1 || strings.HasPrefix(options[0], "-") {
		return c.usage("steam-sdk needs the Steamworks SDK, its zip or its sdk folder, such as: golib steam-sdk ~/Downloads/steamworks_sdk_164.zip")
	}
	source := options[0]
	sdk, err := openSteamSDK(source)
	if err != nil {
		c.check("fail", fmt.Sprintf("%s: %v", source, err))
		return c.summary("steam-sdk")
	}
	defer sdk.close()
	version := sdk.version
	if version == "" {
		version = sdkVersionFromName(source)
	}

	target := c.steamDir()
	partial := target + ".new"
	if err := os.RemoveAll(partial); err != nil {
		c.check("fail", fmt.Sprintf("cannot empty %s: %v", c.shown(partial), err))
		return c.summary("steam-sdk")
	}
	var copied []string
	for _, l := range steamLibraries {
		data, found, err := sdk.read(path.Join("redistributable_bin", l.dir, l.name))
		if err == nil && found {
			file := filepath.Join(partial, l.dir, l.name)
			err = os.MkdirAll(filepath.Dir(file), 0o755)
			if err == nil {
				err = os.WriteFile(file, data, 0o755)
			}
		}
		switch {
		case err != nil:
			c.check("fail", fmt.Sprintf("cannot copy redistributable_bin/%s/%s: %v", l.dir, l.name, err))
			os.RemoveAll(partial)
			return c.summary("steam-sdk")
		case found:
			copied = append(copied, l.dir+"/"+l.name)
		default:
			c.check("warn", fmt.Sprintf("the SDK has no redistributable_bin/%s/%s, so games built with --tags steam on %s won't have Steam", l.dir, l.name, l.platform))
		}
	}
	if len(copied) == 0 {
		c.check("fail", fmt.Sprintf("%s holds none of the Steamworks API's libraries: give the SDK's zip, such as steamworks_sdk_164.zip, or its sdk folder", source))
		os.RemoveAll(partial)
		return c.summary("steam-sdk")
	}
	if version == "" {
		c.check("warn", "found no version in the SDK's Readme.txt nor in its zip's name: .tools/steamworks/VERSION says unknown")
		version = "unknown"
	}
	err = os.WriteFile(filepath.Join(partial, steamVersionFile), []byte(version+"\n"), 0o644)
	if err == nil {
		err = os.RemoveAll(target)
	}
	if err == nil {
		err = os.Rename(partial, target)
	}
	if err != nil {
		c.check("fail", fmt.Sprintf("cannot put the libraries in .tools/steamworks/: %v", err))
		os.RemoveAll(partial)
		return c.summary("steam-sdk")
	}
	c.check("ok", fmt.Sprintf("copied the Steamworks API %s from the SDK into .tools/steamworks/: %s", version, joinWords(copied)))
	if name, ok := c.steamLibrary(); ok && isFile(c.steamFile()) {
		c.check("ok", fmt.Sprintf("builds made here with --tags steam carry %s", name))
	} else if !ok {
		c.check("info", fmt.Sprintf("Steam has no library for %s/%s: builds made here with --tags steam fail", c.goos, c.goarch))
	}
	c.check("info", "the rest of the SDK stays where it is, out of the project: Valve's license lets games ship only these libraries")
	return c.summary("steam-sdk")
}

// steamSDKFiles reads files of the Steamworks SDK, from its zip or its folder.
type steamSDKFiles struct {
	read    func(name string) (data []byte, found bool, err error) // name is from the sdk folder, with slashes
	close   func() error                                           // closes the zip
	version string                                                 // from Readme.txt, such as "1.64", or ""
}

// openSteamSDK opens the Steamworks SDK at source: its zip, the sdk folder
// inside it, or the folder that holds that sdk folder.
func openSteamSDK(source string) (*steamSDKFiles, error) {
	info, err := os.Stat(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("no such file or folder: give the Steamworks SDK's zip, such as steamworks_sdk_164.zip, or its sdk folder")
	}
	if err != nil {
		return nil, err
	}
	sdk := &steamSDKFiles{close: func() error { return nil }}
	if info.IsDir() {
		dir := source
		if !isDir(filepath.Join(dir, "redistributable_bin")) && isDir(filepath.Join(dir, "sdk", "redistributable_bin")) {
			dir = filepath.Join(dir, "sdk")
		}
		if !isDir(filepath.Join(dir, "redistributable_bin")) {
			return nil, errors.New("has no redistributable_bin folder: give the Steamworks SDK's sdk folder, or its zip")
		}
		sdk.read = func(name string) ([]byte, bool, error) {
			data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
			if errors.Is(err, os.ErrNotExist) {
				return nil, false, nil
			}
			return data, err == nil, err
		}
	} else {
		archive, err := zip.OpenReader(source)
		if err != nil {
			return nil, fmt.Errorf("isn't a zip nor a folder: give the Steamworks SDK's zip, such as steamworks_sdk_164.zip, or its sdk folder (%v)", err)
		}
		// The SDK's zip holds one folder, sdk/, but any folder will do: the
		// files are found by what follows the one that holds
		// redistributable_bin/.
		prefix, found := "", false
		for _, f := range archive.File {
			if before, _, ok := strings.Cut(f.Name, "redistributable_bin/"); ok && (before == "" || strings.HasSuffix(before, "/")) {
				prefix, found = before, true
				break
			}
		}
		if !found {
			archive.Close()
			return nil, errors.New("holds no redistributable_bin folder: give the Steamworks SDK's zip, such as steamworks_sdk_164.zip")
		}
		files := map[string]*zip.File{}
		for _, f := range archive.File {
			if name, ok := strings.CutPrefix(f.Name, prefix); ok {
				files[name] = f
			}
		}
		sdk.read = func(name string) ([]byte, bool, error) {
			f, found := files[name]
			if !found {
				return nil, false, nil
			}
			r, err := f.Open()
			if err != nil {
				return nil, true, err
			}
			defer r.Close()
			data, err := io.ReadAll(r)
			return data, true, err
		}
		sdk.close = archive.Close
	}
	if err := sdk.readVersion(); err != nil {
		sdk.close()
		return nil, err
	}
	return sdk, nil
}

// readmeVersion finds the newest version of the SDK that its Readme.txt
// describes: each starts a section with a line such as "v1.64 11th March
// 2026", the newest first.
var readmeVersion = regexp.MustCompile(`(?m)^v(\d+\.\d+)\b`)

// readVersion reads the SDK's version from its Readme.txt, if it has one.
func (sdk *steamSDKFiles) readVersion() error {
	data, found, err := sdk.read("Readme.txt")
	if err != nil || !found {
		return err
	}
	if match := readmeVersion.FindSubmatch(data); match != nil {
		sdk.version = string(match[1])
	}
	return nil
}

// sdkName is the name Valve gives the SDK's zip: steamworks_sdk_164.zip for
// version 1.64.
var sdkName = regexp.MustCompile(`steamworks_sdk_(\d)(\d+)`)

// sdkVersionFromName returns the SDK's version from the name of its zip or
// folder, such as "1.64" for steamworks_sdk_164.zip, or "".
func sdkVersionFromName(source string) string {
	match := sdkName.FindStringSubmatch(filepath.Base(source))
	if match == nil {
		return ""
	}
	return match[1] + "." + match[2]
}

// steamDir is where golib steam-sdk puts the libraries: .tools/steamworks/.
func (c *cli) steamDir() string {
	return c.path(".tools", "steamworks")
}

// steamLibrary returns the folder of .tools/steamworks/ that holds the
// Steamworks API's library for the platform the CLI builds for, and its
// name there, such as "osx/libsteam_api.dylib"; ok is false when Steam has
// none for it, as on Windows on ARM.
func (c *cli) steamLibrary() (name string, ok bool) {
	switch c.goos + "/" + c.goarch {
	case "windows/amd64":
		return "win64/steam_api64.dll", true
	case "linux/amd64":
		return "linux64/libsteam_api.so", true
	case "linux/arm64":
		return "linuxarm64/libsteam_api.so", true
	case "darwin/amd64", "darwin/arm64":
		return "osx/libsteam_api.dylib", true
	}
	return "", false
}

// steamFile is the library of steamLibrary, in .tools/steamworks/.
func (c *cli) steamFile() string {
	name, _ := c.steamLibrary()
	return filepath.Join(c.steamDir(), filepath.FromSlash(name))
}

// steamFileName is the library's file name, as the game loads it.
func (c *cli) steamFileName() string {
	name, _ := c.steamLibrary()
	return path.Base(name)
}

// steamVersion is the SDK's version that .tools/steamworks/VERSION gives,
// or "".
func (c *cli) steamVersion() string {
	data, err := os.ReadFile(filepath.Join(c.steamDir(), steamVersionFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// withSteam says whether --tags put Steam in the build.
func (c *cli) withSteam() bool {
	return c.tags != "" && slices.Contains(strings.Split(c.tags, ","), steamTag)
}

// checkSteam reports, with a [fail] line, whether a desktop build of game
// made with the build tag steam can't be made: game.json gives no
// "steamAppID", or .tools/steamworks/ has no library for this platform. It
// returns what game.json says, and false after a failure. Without the tag it
// checks nothing.
func (c *cli) checkSteam(game string) bool {
	if !c.withSteam() {
		return true
	}
	info, _, err := readGameInfo(c.gameDir(game))
	if err != nil {
		c.check("fail", fmt.Sprintf("%s/%s: %v", c.gameModule(game), gameInfoFile, err))
		return false
	}
	if info.SteamAppID == 0 {
		c.check("fail", fmt.Sprintf(`the build tag steam needs the game's Steam app ID: add it to %s/%s, such as "steamAppID": 480 (480 is Spacewar, Valve's app for trying the API)`, c.gameModule(game), gameInfoFile))
		return false
	}
	if _, ok := c.steamLibrary(); !ok {
		c.check("fail", fmt.Sprintf("Steam has no library for %s/%s, so a game can't be built with the build tag steam here", c.goos, c.goarch))
		return false
	}
	if !isFile(c.steamFile()) {
		name, _ := c.steamLibrary()
		c.check("fail", fmt.Sprintf("the build tag steam needs the Steamworks API, and .tools/steamworks/ has no %s: download the Steamworks SDK from partner.steamgames.com and run golib steam-sdk <path to steamworks_sdk_XXX.zip>", name))
		return false
	}
	return true
}

// copySteam copies the Steamworks API's library into dir, for a build made
// with the build tag steam, and says whether it did. Without the tag it
// copies nothing.
func (c *cli) copySteam(dir string) (bool, error) {
	if !c.withSteam() {
		return false, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	return true, syncFile(c.steamFile(), filepath.Join(dir, c.steamFileName()))
}

// steamAppIDFile is the file that tells Steam which game a program is when
// Steam didn't start it: debug builds write it next to the executable, and
// dist builds never carry it, so a player's copy starts through Steam.
const steamAppIDFile = "steam_appid.txt"

// writeSteamAppID writes steam_appid.txt into dir, next to a debug build
// made with the build tag steam.
func (c *cli) writeSteamAppID(game, dir string) error {
	if !c.withSteam() {
		return nil
	}
	info, _, err := readGameInfo(c.gameDir(game))
	if err != nil {
		return err
	}
	content := []byte(fmt.Sprintf("%d\n", info.SteamAppID))
	file := filepath.Join(dir, steamAppIDFile)
	if old, err := os.ReadFile(file); err == nil && bytes.Equal(old, content) {
		return nil
	}
	return os.WriteFile(file, content, 0o644)
}

// steamNotice is the notice of the Steamworks API for THIRD-PARTY-LICENSES.txt.
func (c *cli) steamNotice(where string) notice {
	title := "Steamworks API"
	if v := c.steamVersion(); v != "" && v != "unknown" {
		title += ", from the Steamworks SDK " + v
	}
	return notice{title: title, url: "https://partner.steamgames.com", where: where, texts: []licenseText{{
		name: "Steamworks SDK Access Agreement",
		text: "The Steamworks API library is Valve Corporation's, copyright Valve Corporation. It is\n" +
			"redistributed with this game under the Steamworks SDK Access Agreement,\n" +
			"https://partner.steamgames.com/documentation/sdk_access_agreement, which lets\n" +
			"games on Steam carry it. It is not under the licenses above.\n",
	}}}
}
