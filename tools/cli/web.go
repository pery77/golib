package main

import (
	"fmt"
	"html"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// The files a web build is made of, next to the game's .wasm:
//
//   - index.html   the page, written by writePage
//   - wasm_exec.js Go's own, from .tools/go/lib/wasm/
//   - golib.js     the other half of the web backend,
//     framework/internal/device/web.js
const (
	webPageFile    = "index.html"
	wasmExecFile   = "wasm_exec.js"
	webGlueFile    = "golib.js"
	defaultWebPort = 8080
)

// web builds a game for the browser and serves it on this machine, so that
// the game can be played at a http://localhost address. The build is stage 1
// of the web build: no sound and no post-processing shaders yet (see
// docs/roadmap.md).
func (c *cli) web(options []string) int {
	var names []string
	port := defaultWebPort
	open, lan := true, false
	for i := 0; i < len(options); i++ {
		option := options[i]
		switch {
		case option == "--no-open":
			open = false
		case option == "--lan":
			lan = true
		case option == "--port":
			i++
			if i >= len(options) {
				return c.usage("--port needs a number, such as: golib web --port 9000")
			}
			chosen, err := strconv.Atoi(options[i])
			if err != nil || chosen < 0 || chosen > 65535 {
				return c.usage(fmt.Sprintf("--port got %q: pass a port number from 0 to 65535, or 0 to pick a free one", options[i]))
			}
			port = chosen
		case strings.HasPrefix(option, "-"):
			return c.usage("unknown option for web: " + option)
		default:
			names = append(names, option)
		}
	}
	game, exitCode := c.resolveGame("web", names)
	if game == "" {
		return exitCode
	}
	folder := c.buildWeb(game)
	if folder == "" {
		return c.summary("web")
	}
	return c.serveWeb(game, folder, port, open, lan)
}

// webTags is what a web build is built with: its assets go inside it, as a
// dist build's do, because a page has no folder to read them from.
const webTags = "golib_dist"

// buildWeb builds game for the browser into build/<game>/web/ and returns
// that folder, or "" after reporting a failure.
func (c *cli) buildWeb(game string) string {
	folder := c.path("build", game, "web")
	if _, ok := c.buildWebInto(game, folder); !ok {
		return ""
	}
	return folder
}

// buildWebInto builds game for the browser into folder, with the page and the
// two JavaScript files next to the .wasm. It returns what game.json says and
// false after reporting a failure.
func (c *cli) buildWebInto(game, folder string) (gameInfo, bool) {
	dir := c.path("games", game)
	shown := "games/" + game
	const tags = webTags

	info, _, err := readGameInfo(dir)
	if err != nil {
		c.check("fail", err.Error())
		return info, false
	}
	packages, err := c.listWebPackages(dir)
	if err != nil {
		c.check("fail", "could not inspect "+shown+" (see the Go errors above)")
		return info, false
	}
	if isDir(filepath.Join(dir, "assets")) && !embedsAssets(packages) {
		c.check("fail", shown+"/assets/ would be missing from the web build: add "+shown+"/assets.go, as the golib.EmbedAssets documentation shows")
		return info, false
	}
	c.reportSilentOnWeb(unplayableOnWeb(filepath.Join(dir, "assets")))

	shownFolder := c.shown(folder) + "/"
	if err := os.MkdirAll(folder, 0o755); err != nil {
		c.check("fail", fmt.Sprintf("cannot create %s: %v", shownFolder, err))
		return info, false
	}
	wasm := filepath.Join(folder, game+".wasm")
	_, err = c.runGo(goCall{
		dir:  dir,
		args: []string{"build", "-trimpath", "-tags=" + tags, "-o", wasm, "."},
		env:  webEnv,
	})
	if err != nil {
		c.check("fail", fmt.Sprintf("web build failed for %s (see the Go errors above)", shown))
		return info, false
	}
	size, err := os.Stat(wasm)
	if err != nil {
		c.check("fail", fmt.Sprintf("cannot read %s: %v", c.shown(wasm), err))
		return info, false
	}
	c.check("ok", fmt.Sprintf("built %s into %s (%s)", shown, c.shown(wasm), fileSize(size.Size())))

	goWasmExec := c.path(".tools", "go", "lib", "wasm", wasmExecFile)
	if err := syncFile(goWasmExec, filepath.Join(folder, wasmExecFile)); err != nil {
		c.check("fail", fmt.Sprintf("cannot copy %s from the Go toolchain: %v", wasmExecFile, err))
		return info, false
	}
	glue := c.path("framework", "internal", "device", "web.js")
	if err := syncFile(glue, filepath.Join(folder, webGlueFile)); err != nil {
		c.check("fail", fmt.Sprintf("cannot copy the web backend's %s: %v", webGlueFile, err))
		return info, false
	}
	if err := c.writePage(game, info, folder); err != nil {
		c.check("fail", fmt.Sprintf("cannot write %s%s: %v", shownFolder, webPageFile, err))
		return info, false
	}
	c.check("ok", fmt.Sprintf("wrote %s, %s and %s next to it", webPageFile, wasmExecFile, webGlueFile))
	return info, true
}

// webEnv builds for the browser instead of this machine.
var webEnv = []string{"GOOS=js", "GOARCH=wasm"}

// webSilentFormats are the sound files GoLib plays on the desktop that no
// browser decodes: the tracker music raylib reads, and QOA. A game that wants
// to be played in a browser keeps its music as .ogg or .mp3.
var webSilentFormats = []string{".xm", ".mod", ".qoa"}

// webTrackerFormats are the ones among them that golib.NewMusic plays. Where
// a game's music is one of these, a web build plays a file beside it with the
// same name in a format the browser reads, and runs without the music when
// there is none. A .qoa sound has no such way out and stops the game.
var webTrackerFormats = []string{".xm", ".mod"}

// browserFormats are the sound files browsers decode, in the order package
// golib prefers them, which is the order it looks for music to play in place
// of a tracker module.
var browserFormats = []string{".ogg", ".mp3", ".wav"}

// silentOnWeb is a file in the game's assets folder that no browser can play,
// named from that folder, and what a web build plays in its place.
type silentOnWeb struct {
	name    string // "assets/music/tune.xm"
	instead string // "assets/music/tune.ogg", or "" when there is no such file
	music   bool   // tracker music, which package golib replaces or skips
}

// unplayableOnWeb returns the files in the game's assets folder that no
// browser can play, sorted, each with the file a web build plays instead.
func unplayableOnWeb(assets string) []silentOnWeb {
	var found []silentOnWeb
	filepath.WalkDir(assets, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		format := strings.ToLower(filepath.Ext(path))
		if !slices.Contains(webSilentFormats, format) {
			return nil
		}
		name, err := filepath.Rel(assets, path)
		if err != nil {
			return nil
		}
		file := silentOnWeb{name: "assets/" + filepath.ToSlash(name), music: slices.Contains(webTrackerFormats, format)}
		if file.music {
			file.instead = insteadOnWeb(assets, name)
		}
		found = append(found, file)
		return nil
	})
	slices.SortFunc(found, func(a, b silentOnWeb) int { return strings.Compare(a.name, b.name) })
	return found
}

// insteadOnWeb returns the file, named from the assets folder, that a web
// build plays in place of the music named name, which is a path inside that
// folder: the same name in the first browser format that is there, as package
// golib looks for it. It returns "" when there is none.
func insteadOnWeb(assets, name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	for _, format := range browserFormats {
		if isFile(filepath.Join(assets, base+format)) {
			return "assets/" + filepath.ToSlash(base+format)
		}
	}
	return ""
}

// reportSilentOnWeb says what a browser will do with the sound files it can't
// decode: play another file, run without the music, or stop the game.
func (c *cli) reportSilentOnWeb(files []silentOnWeb) {
	var quiet, stopping []string
	for _, file := range files {
		switch {
		case file.instead != "":
			// One line each: they are separate files, with a fix each.
			c.check("info", "a browser cannot play "+file.name+", so a web build plays "+file.instead+" instead")
		case file.music:
			quiet = append(quiet, file.name)
		default:
			stopping = append(stopping, file.name)
		}
	}
	if len(quiet) > 0 {
		c.check("warn", fmt.Sprintf("a browser cannot play %s, and there is no %s file of the same name beside %s, so the game runs in a browser without that music: save %s as .ogg for the web",
			joinWords(quiet), joinEither(browserFormats), pluralThem(len(quiet)), pluralThem(len(quiet))))
	}
	if len(stopping) > 0 {
		c.check("warn", fmt.Sprintf("a browser cannot play %s, so the game stops with a message when it reaches %s: save %s as .ogg or .mp3 for the web",
			joinWords(stopping), pluralThem(len(stopping)), pluralThem(len(stopping))))
	}
}

// joinEither lists choices, where joinWords lists things that go together.
func joinEither(words []string) string {
	if len(words) <= 1 {
		return strings.Join(words, "")
	}
	return strings.Join(words[:len(words)-1], ", ") + " or " + words[len(words)-1]
}

// pluralThem names one file or several, for a sentence about them.
func pluralThem(count int) string {
	if count == 1 {
		return "it"
	}
	return "them"
}

// listWebPackages lists what a web build is made of, which is not what a
// desktop build is made of: raylib and the libraries it calls stay out.
func (c *cli) listWebPackages(dir string) ([]goPackage, error) {
	return c.listPackagesWith(dir, webTags, webEnv)
}

// distWeb builds game for the browser to share: the page and its files in
// build/<game>/dist/web/, with the licenses that go with them, and a zip of
// what is in that folder, which is what itch.io takes.
func (c *cli) distWeb(game string) bool {
	distDir := c.path("build", game, "dist")
	folder := filepath.Join(distDir, "web")
	if err := os.RemoveAll(folder); err != nil {
		c.check("fail", fmt.Sprintf("cannot empty %s (is a browser holding it open?): %v", c.shown(folder), err))
		return false
	}
	info, ok := c.buildWebInto(game, folder)
	if !ok {
		return false
	}

	// A web build carries Go's runtime and GoLib's framework, and nothing of
	// raylib: the browser draws and sounds instead.
	packages, err := c.listWebPackages(c.path("games", game))
	if err != nil {
		c.check("fail", "could not inspect games/"+game+" (see the Go errors above)")
		return false
	}
	modules := thirdPartyModules(packages)
	notices := c.gameNotices(game, game+".wasm", modules, nil, false)
	text, err := thirdPartyNotices(info.Title, notices)
	if err != nil {
		c.check("fail", fmt.Sprintf("cannot write %s: %v", noticesFile, err))
		return false
	}
	if err := os.WriteFile(filepath.Join(folder, noticesFile), text, 0o644); err != nil {
		c.check("fail", fmt.Sprintf("cannot write %s: %v", noticesFile, err))
		return false
	}
	titles := make([]string, 0, len(notices))
	for _, n := range notices {
		titles = append(titles, n.title)
	}
	c.check("ok", fmt.Sprintf("wrote %s next to it, with the licenses of %s", noticesFile, joinWords(titles)))

	zipName := fmt.Sprintf("%s-%s-web.zip", game, info.Version)
	zipPath := filepath.Join(distDir, zipName)
	if err := os.Remove(zipPath); err != nil && !os.IsNotExist(err) {
		c.check("fail", fmt.Sprintf("cannot replace %s: %v", c.shown(zipPath), err))
		return false
	}
	size, err := zipInside(folder, zipPath)
	if err != nil {
		c.check("fail", fmt.Sprintf("cannot zip %s: %v", c.shown(folder), err))
		return false
	}
	c.check("ok", fmt.Sprintf("zipped what is in %s/ into build/%s/dist/%s (%s): share this file", c.shown(folder), game, zipName, fileSize(size)))
	c.check("info", fmt.Sprintf("upload it to itch.io as a web game, or serve the files anywhere: %s is at the top of the zip, where a page host looks for it", webPageFile))
	return true
}

// writePage writes the page that loads the game: a canvas that fills the
// window, the backend, Go's loader, and the game itself.
func (c *cli) writePage(game string, info gameInfo, folder string) error {
	page := strings.NewReplacer(
		"{{title}}", html.EscapeString(info.Title),
		"{{wasm}}", html.EscapeString(game+".wasm"),
		"{{glue}}", webGlueFile,
		"{{exec}}", wasmExecFile,
	).Replace(webPageTemplate)
	return os.WriteFile(filepath.Join(folder, webPageFile), []byte(page), 0o644)
}

// serveWeb serves folder on this machine until the terminal stops it, and
// opens the game in a browser unless open is false. With lan, it serves to the
// whole network the machine is on, and says at which address, so that a phone
// or a tablet on the same Wi-Fi can play the game: a touch screen is the one
// thing this machine cannot try for itself.
func (c *cli) serveWeb(game, folder string, port int, open, lan bool) int {
	host := "127.0.0.1"
	if lan {
		host = "0.0.0.0"
	}
	listener, err := net.Listen("tcp", host+":"+strconv.Itoa(port))
	if err != nil {
		c.check("fail", fmt.Sprintf("cannot serve on port %d: %v", port, err))
		if port != 0 {
			c.check("info", "another program may be using that port: try golib web --port 0 to pick a free one")
		}
		return c.summary("web")
	}
	served := listener.Addr().(*net.TCPAddr).Port
	address := fmt.Sprintf("http://localhost:%d/", served)
	c.check("info", fmt.Sprintf("serving %s at %s (press Ctrl+C to stop)", c.shown(folder), address))
	if lan {
		if onNetwork := networkAddress(); onNetwork != "" {
			c.check("info", fmt.Sprintf("open http://%s:%d/ on a phone or tablet on the same network, to play the game with a touch screen", onNetwork, served))
		} else {
			c.check("warn", "this machine has no network address to give: it may not be on a network, and only this machine can open the game")
		}
		c.check("info", "anyone on this network can open the game while --lan serves it")
	}
	if open {
		c.openBrowser(address)
	}
	c.summary("web")
	server := &http.Server{Handler: webFiles(folder)}
	if err := server.Serve(listener); err != nil {
		fmt.Fprintf(c.stderr, "golib: the web server stopped: %v\n", err)
		return 1
	}
	return 0
}

// networkAddress returns the address this machine has on the network it is on,
// for another device to open, or "" when it has none. A machine can have
// several, one per adapter, and the first one that is neither a loopback nor a
// link-local address is the one a phone on the same Wi-Fi reaches.
func networkAddress() string {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addresses {
		network, ok := address.(*net.IPNet)
		if !ok {
			continue
		}
		ip := network.IP.To4()
		if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			continue
		}
		return ip.String()
	}
	return ""
}

// webFiles serves the web build. Browsers only run WebAssembly served as
// application/wasm, which Go's own table doesn't always have, so it is set
// here, and nothing is cached, so a rebuild shows on the next reload.
func webFiles(folder string) http.Handler {
	files := http.FileServer(http.Dir(folder))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".wasm") {
			w.Header().Set("Content-Type", "application/wasm")
		}
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	})
}

// openBrowser shows address in the machine's browser. A machine with none,
// such as a server, is no reason to stop: the address is on the terminal.
func (c *cli) openBrowser(address string) {
	var cmd *exec.Cmd
	switch c.goos {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", address)
	case "darwin":
		cmd = exec.Command("open", address)
	default:
		cmd = exec.Command("xdg-open", address)
	}
	if err := cmd.Start(); err != nil {
		c.check("info", "could not open a browser: open the address above yourself")
		return
	}
	go cmd.Wait() // let the browser go without leaving it behind
}

// fileSize says how large a file is, in the units people read.
func fileSize(bytes int64) string {
	switch {
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(bytes)/(1<<10))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

// webPageTemplate is the page a web build is played in. It is plain HTML on
// purpose: someone sharing the game can read it, change the background or
// wrap it in a page of their own.
const webPageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no, viewport-fit=cover">
<title>{{title}}</title>
<style>
	/* The game fills the page, and nothing on a phone treats it as a page:
	   a tap is the game's, not a zoom, a text selection or a scroll, and
	   100dvh is the screen a phone really leaves, without the part its
	   address bar covers. */
	html, body { margin: 0; height: 100%; height: 100dvh; background: #000; overflow: hidden; }
	body {
		touch-action: none; overscroll-behavior: none;
		-webkit-user-select: none; user-select: none; -webkit-tap-highlight-color: transparent;
	}
	#game { display: block; width: 100%; height: 100%; outline: none; touch-action: none; }
	#message {
		position: absolute; inset: 0; display: flex; align-items: center; justify-content: center;
		color: #eee; font: 16px system-ui, sans-serif; text-align: center; padding: 1em;
	}
</style>
</head>
<body>
<canvas id="game"></canvas>
<div id="message">Loading&hellip;</div>
<script src="{{glue}}"></script>
<script src="{{exec}}"></script>
<script>
	const message = document.getElementById('message');
	function fail(text) { message.textContent = text; }
	if (!window.WebAssembly) {
		fail('This browser cannot run WebAssembly, which the game is made of.');
	} else if (!document.createElement('canvas').getContext('webgl2')) {
		fail('This browser has no WebGL 2, which the game draws with.');
	} else {
		const go = new Go();
		// golib shot --web opens this page with the settings the terminal
		// gives a game on the desktop, and the game reads them the same way.
		new URLSearchParams(location.search).forEach(function (value, name) {
			if (name.indexOf('GOLIB_') === 0) go.env[name] = value;
		});
		WebAssembly.instantiateStreaming(fetch('{{wasm}}'), go.importObject).then(function (loaded) {
			message.remove();
			return go.run(loaded.instance);
		}).catch(function (err) {
			fail('The game could not start: ' + err);
			console.error(err);
		});
	}
</script>
</body>
</html>
`
