package main

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
)

// A macOS dist build is an app: a folder named <title>.app that Finder shows
// and opens as one file, with the game's icon, and its title in the Dock and
// the menu bar (see docs/tooling.md#dist-builds):
//
//	<title>.app/Contents/
//	  Info.plist         the details macOS reads: the title, version and icon
//	  MacOS/<fileName>   the executable, named after game.json's "fileName" or the folder
//	  Resources/icon.icns  the icon, from icon.png, when the game has one

// macIconImages are the images an .icns file holds: the type macOS reads
// each as, and its size in pixels. The @2x types serve Retina screens and
// hold the same picture as the plain type of their size, as the files Apple's
// iconutil writes do.
var macIconImages = []struct {
	kind string
	size int
}{
	{"icp4", 16},
	{"ic11", 32}, // 16@2x
	{"icp5", 32},
	{"ic12", 64}, // 32@2x
	{"ic07", 128},
	{"ic13", 256}, // 128@2x
	{"ic08", 256},
	{"ic14", 512}, // 256@2x
	{"ic09", 512},
	{"ic10", 1024}, // 512@2x
}

// macIconSizes are the sizes of macIconImages, each once, from the smallest.
var macIconSizes = []int{16, 32, 64, 128, 256, 512, 1024}

// be is the byte order of .icns files.
var be = binary.BigEndian

// macIcon returns the .icns file of icon: each of macIconImages as a PNG
// image, resized the way Windows icons are.
func macIcon(icon *image.NRGBA) []byte {
	pngs := map[int][]byte{}
	var images []byte
	for _, entry := range macIconImages {
		data, found := pngs[entry.size]
		if !found {
			data = encodePNG(resize(icon, entry.size))
			pngs[entry.size] = data
		}
		images = append(images, entry.kind...)
		images = be.AppendUint32(images, uint32(8+len(data)))
		images = append(images, data...)
	}
	file := be.AppendUint32([]byte("icns"), uint32(8+len(images)))
	return append(file, images...)
}

// appName returns the name of the app of a game called title in game.json,
// whose folder is game: the title, with the characters macOS keeps out of
// file names replaced, and .app.
func appName(title, game string) string {
	name := strings.Map(func(r rune) rune {
		if r == '/' || r == ':' {
			return '-'
		}
		return r
	}, title)
	name = strings.TrimLeft(strings.TrimSpace(name), ".") // a leading dot hides a file
	if name == "" {
		name = game
	}
	return name + ".app"
}

// bundleIdentifier returns the identifier macOS knows the game's app by, made
// from game, game.json's "fileName" or the folder's name, which golib dist
// also names the save folder after: letters, digits, hyphens and dots only,
// as macOS requires.
func bundleIdentifier(game string) string {
	name := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' {
			return r
		}
		return '-'
	}, game)
	name = strings.Trim(name, "-.")
	if name == "" {
		name = "game"
	}
	return "golib.games." + name
}

// appInfo returns the Info.plist of a game's app, whose executable is
// called exe, with the details from info, and icon.icns as its icon when
// withIcon.
func appInfo(info gameInfo, exe string, withIcon bool) []byte {
	v := info.version
	number := fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch) // macOS takes no label
	entries := []struct {
		key   string
		value any
	}{
		{"CFBundleDevelopmentRegion", "en"},
		{"CFBundleDisplayName", info.Title},
		{"CFBundleExecutable", exe},
		{"CFBundleIconFile", "icon.icns"},
		{"CFBundleIdentifier", bundleIdentifier(info.FileName)},
		{"CFBundleInfoDictionaryVersion", "6.0"},
		{"CFBundleName", info.Title},
		{"CFBundlePackageType", "APPL"},
		{"CFBundleShortVersionString", number},
		{"CFBundleVersion", number},
		{"LSApplicationCategoryType", "public.app-category.games"},
		{"NSHighResolutionCapable", true},
		{"NSHumanReadableCopyright", info.Copyright},
	}
	var plist bytes.Buffer
	plist.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
`)
	for _, entry := range entries {
		switch value := entry.value.(type) {
		case string:
			if value == "" || entry.key == "CFBundleIconFile" && !withIcon {
				continue
			}
			fmt.Fprintf(&plist, "\t<key>%s</key>\n\t<string>", entry.key)
			xml.EscapeText(&plist, []byte(value))
			plist.WriteString("</string>\n")
		case bool:
			fmt.Fprintf(&plist, "\t<key>%s</key>\n\t<%t/>\n", entry.key, value)
		}
	}
	plist.WriteString("</dict>\n</plist>\n")
	return plist.Bytes()
}

// makeApp turns the executable folder/exe into the app players open,
// folder/<title>.app, with game.json's details and icon.png as its icon, and
// GoLib's native library when the game uses rigid bodies, and signs it. It returns the app's name, or "" after reporting a failure.
func (c *cli) makeApp(game, folder, exe string, info gameInfo) string {
	name := appName(info.Title, game)
	shown := "build/" + game + "/dist/" + filepath.Base(folder) + "/" + name
	fail := func(err error) string {
		c.check("fail", fmt.Sprintf("cannot make %s: %v", shown, err))
		return ""
	}
	contents := filepath.Join(folder, name, "Contents")
	if err := os.MkdirAll(filepath.Join(contents, "MacOS"), 0o755); err != nil {
		return fail(err)
	}
	if err := os.Rename(filepath.Join(folder, exe), filepath.Join(contents, "MacOS", exe)); err != nil {
		return fail(err)
	}
	icon, withIcon, err := readIcon(filepath.Join(c.gameDir(game), iconFile))
	if err != nil {
		return fail(err)
	}
	if withIcon {
		if err := os.MkdirAll(filepath.Join(contents, "Resources"), 0o755); err != nil {
			return fail(err)
		}
		if err := os.WriteFile(filepath.Join(contents, "Resources", "icon.icns"), macIcon(icon), 0o644); err != nil {
			return fail(err)
		}
	}
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), appInfo(info, exe, withIcon), 0o644); err != nil {
		return fail(err)
	}
	// A game with rigid bodies loads golibnative from the app's Frameworks
	// folder. codesign signs the app only when what is inside it is signed,
	// so the library is signed first.
	frameworks := filepath.Join(contents, "Frameworks")
	if copied, err := c.copyNative(game, filepath.Join(contents, "MacOS", exe), frameworks); err != nil {
		return fail(err)
	} else if copied {
		if err := c.signApp(filepath.Join(frameworks, c.nativeName())); err != nil {
			c.check("warn", fmt.Sprintf("could not sign %s, so macOS may call the app damaged once players download it: %v", c.nativeName(), err))
		}
	}
	// A game with a language model loads golibllm from Frameworks too, and
	// its models from Resources/models.
	if copied, err := c.copyLLM(game, filepath.Join(contents, "MacOS", exe), frameworks); err != nil {
		return fail(err)
	} else if copied != nil {
		for _, f := range copied {
			if err := c.signApp(f); err != nil {
				c.check("warn", fmt.Sprintf("could not sign %s, so macOS may call the app damaged once players download it: %v", filepath.Base(f), err))
			}
		}
		if !c.syncModels(game, info) {
			return fail(errors.New("a language model is missing"))
		}
		if _, err := c.copyModels(game, filepath.Join(contents, "Resources", modelsFolder)); err != nil {
			return fail(err)
		}
	}
	// A downloaded app whose files don't match its signature is one macOS
	// calls damaged, with no way to open it. The linker signs the executable
	// on its own on Apple silicon, so the app is signed again as a whole.
	if err := c.signApp(filepath.Join(folder, name)); err != nil {
		c.check("warn", fmt.Sprintf("could not sign %s, so macOS may call it damaged once players download it: %v", name, err))
	}
	return name
}
