package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The rigid bodies of the framework (framework/rigidbody.go) are simulated by
// Jolt Physics: on the desktop it is in GoLib's native library, golibnative,
// built from native/ by .github/workflows/native.yml and published as a
// release; in a browser it is JoltPhysics.js, the same engine built for
// WebAssembly. golib setup downloads both into .tools/, checking each file
// against the checksum written here, and the builds of a game that uses
// rigid bodies carry them, as they carry raylib. A game that doesn't use them
// never needs them: builds look for physicsMarker to tell.

// physicsMarker is in the build of every game that makes a world of rigid
// bodies, and in no other: framework/internal/device/physics.go's
// PhysicsMarker.
const physicsMarker = "golib-uses-rigid-bodies:jolt-physics"

// nativeRepository and nativeRelease name the GitHub release that setup
// downloads golibnative from. Its files are named golibnative-<os>-<arch>
// with the system's extension.
const (
	nativeRepository = "supercanocoder/golib"
	nativeRelease    = "native-2"
)

// nativeChecksums holds the SHA-256 of golibnative in nativeRelease for each
// platform, written here when the release is made (see native/README.md). A
// platform without one has no release yet: setup leaves a copy built by hand
// with native/build.sh in place, and says so.
var nativeChecksums = map[string]string{
	"darwin/amd64":  "87ccbeae15f567c160f17d07d5b5390de88b528e695d1dd40160a47fceac3b41",
	"darwin/arm64":  "42bbc82467715f7a9e9231909ac2ff1ed1b6c40a0d082f03c95b154e96e854c1",
	"linux/amd64":   "489ef66fd04378a11271fa1c01f55df0042f70971c0b2a33146b19baa09ea8c1",
	"linux/arm64":   "e385b16549eb6fcfc8a39f2e9e3aebb36e39041174ce17225981c7ce645a7845",
	"windows/amd64": "7d7a1a82fed3b4d9d56c7e7261d179cade408d11c9f4a859f4b5571ceddd4a68",
}

// joltJSVersion is the version of JoltPhysics.js, the npm package
// jolt-physics, that web builds carry, and joltJSFiles its files, from its
// dist/ folder, with their SHA-256.
const joltJSVersion = "1.1.0"

var joltJSFiles = []namedSum{
	{"jolt-physics.wasm.js", "bcebc61c2a5db94f1b155b2ce1902667f952d9d63baa003b654e9e5d1ff20eb4"},
	{"jolt-physics.wasm.wasm", "65f044b2ec57be2bbf0f84828f3948d6f3f9e17942ad80aa98923b94dd292f90"},
}

// namedSum is a file's name and its SHA-256.
type namedSum struct{ name, sha256 string }

// joltNotices holds the licenses of Jolt Physics and JoltPhysics.js, for
// THIRD-PARTY-LICENSES.txt.
//
//go:embed notices/jolt-*.txt
var joltNotices embed.FS

// nativeName is golibnative's file name on the platform the CLI builds for.
func (c *cli) nativeName() string {
	switch c.goos {
	case "windows":
		return "golibnative.dll"
	case "darwin":
		return "golibnative.dylib"
	}
	return "golibnative.so"
}

// nativeFile is where golib setup puts golibnative: .tools/native/.
func (c *cli) nativeFile() string {
	return c.path(".tools", "native", c.nativeName())
}

// syncNative makes sure .tools/native/ holds golibnative for this platform
// and .tools/jolt/ holds JoltPhysics.js, downloading what is missing or
// different, and reports each.
func (c *cli) syncNative() {
	platform := c.goos + "/" + c.goarch
	file := c.nativeFile()
	if sum, found := c.natives[platform]; found {
		ext := filepath.Ext(file)
		url := fmt.Sprintf("https://github.com/%s/releases/download/%s/golibnative-%s-%s%s", nativeRepository, nativeRelease, c.goos, c.goarch, ext)
		downloaded, err := c.syncDownload(url, sum, file)
		switch {
		case err != nil:
			c.check("fail", fmt.Sprintf("cannot download GoLib's native library %s: %v", nativeRelease, err))
		case downloaded:
			c.check("ok", fmt.Sprintf("GoLib's native library %s, for rigid bodies, downloaded into .tools/native/", nativeRelease))
		default:
			c.check("ok", fmt.Sprintf("GoLib's native library %s, for rigid bodies, in .tools/native/", nativeRelease))
		}
	} else if isFile(file) {
		c.check("ok", fmt.Sprintf("GoLib's native library, for rigid bodies, built by hand in .tools/native/ (%s has no release for %s yet)", nativeRelease, platform))
	} else {
		c.check("info", fmt.Sprintf("GoLib's native library has no release for %s yet, so games with rigid bodies won't build here: build it with native/build.sh (see native/README.md)", platform))
	}

	folder := c.path(".tools", "jolt")
	downloaded := false
	for _, f := range c.joltJS {
		got, err := c.syncDownload(c.joltJSURL(f.name), f.sha256, filepath.Join(folder, f.name))
		if err != nil {
			c.check("fail", fmt.Sprintf("cannot download JoltPhysics.js %s: %v", joltJSVersion, err))
			return
		}
		downloaded = downloaded || got
	}
	if downloaded {
		c.check("ok", fmt.Sprintf("JoltPhysics.js %s, for rigid bodies in web builds, downloaded into .tools/jolt/", joltJSVersion))
	} else {
		c.check("ok", fmt.Sprintf("JoltPhysics.js %s, for rigid bodies in web builds, in .tools/jolt/", joltJSVersion))
	}
}

// joltJSURL is where setup downloads JoltPhysics.js's file name from.
func (c *cli) joltJSURL(name string) string {
	return fmt.Sprintf("https://cdn.jsdelivr.net/npm/jolt-physics@%s/dist/%s", joltJSVersion, name)
}

// syncDownload makes sure target holds the file at url whose SHA-256 is sum,
// downloading it when target is missing or holds something else, and says
// whether it did. A download that doesn't match sum is thrown away.
func (c *cli) syncDownload(url, sum, target string) (bool, error) {
	if data, err := os.ReadFile(target); err == nil && sha256Hex(data) == sum {
		return false, nil
	}
	data, err := c.download(url)
	if err != nil {
		return false, err
	}
	if got := sha256Hex(data); got != sum {
		return false, fmt.Errorf("%s isn't the file GoLib expects: its SHA-256 is %s, not %s", url, got, sum)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return false, err
	}
	partial := target + ".part"
	if err := os.WriteFile(partial, data, 0o755); err != nil {
		return false, err
	}
	return true, os.Rename(partial, target)
}

// download fetches the file at url.
func download(url string) ([]byte, error) {
	client := http.Client{Timeout: 5 * time.Minute}
	response, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s", url, response.Status)
	}
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	return data, nil
}

// sha256Hex is the SHA-256 of data, in hexadecimal.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// usesRigidBodies says whether the program built at path makes a world of
// rigid bodies, and so needs Jolt.
func usesRigidBodies(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return bytes.Contains(data, []byte(physicsMarker)), nil
}

// errNoNative says that a game needs golibnative and .tools/native/ hasn't
// got it.
func (c *cli) errNoNative(game string) error {
	return fmt.Errorf("%s uses rigid bodies, which need GoLib's native library, and .tools/native/ has no %s: run golib setup (see native/README.md)", c.gameModule(game), c.nativeName())
}

// copyNative copies golibnative into dir when the game built at exe uses
// rigid bodies, and says whether it did.
func (c *cli) copyNative(game, exe, dir string) (bool, error) {
	uses, err := usesRigidBodies(exe)
	if err != nil || !uses {
		return false, err
	}
	if !isFile(c.nativeFile()) {
		return false, c.errNoNative(game)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	return true, syncFile(c.nativeFile(), filepath.Join(dir, c.nativeName()))
}

// copyJoltJS copies JoltPhysics.js next to a web build's .wasm when the game
// uses rigid bodies, and says whether it did.
func (c *cli) copyJoltJS(game, wasm string) (bool, error) {
	uses, err := usesRigidBodies(wasm)
	if err != nil || !uses {
		return false, err
	}
	for _, f := range c.joltJS {
		source := c.path(".tools", "jolt", f.name)
		if !isFile(source) {
			return false, fmt.Errorf("%s uses rigid bodies, which need JoltPhysics.js in a browser, and .tools/jolt/ has no %s: run golib setup", c.gameModule(game), f.name)
		}
		if err := syncFile(source, filepath.Join(filepath.Dir(wasm), f.name)); err != nil {
			return false, err
		}
	}
	return true, nil
}

// joltNotice returns the notice of Jolt for THIRD-PARTY-LICENSES.txt: Jolt
// Physics in golibnative, where, and with web, JoltPhysics.js too.
func joltNotice(where string, web bool) notice {
	n := notice{title: "Jolt Physics", url: "https://github.com/jrouwe/JoltPhysics", where: where}
	names := []string{"jolt-physics"}
	if web {
		n.title = "JoltPhysics.js " + joltJSVersion + ", with Jolt Physics"
		n.url = "https://github.com/jrouwe/JoltPhysics.js"
		names = append(names, "jolt-physics-js")
	}
	for _, name := range names {
		text, err := joltNotices.ReadFile("notices/" + name + ".txt")
		if err != nil {
			panic(errors.Join(errors.New("tools/cli/notices/"+name+".txt is missing"), err))
		}
		title, body, _ := strings.Cut(string(text), "\n")
		n.texts = append(n.texts, licenseText{name: title, text: body})
	}
	return n
}
