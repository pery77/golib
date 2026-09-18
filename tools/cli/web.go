package main

import (
	"fmt"
	"html"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	open := true
	for i := 0; i < len(options); i++ {
		option := options[i]
		switch {
		case option == "--no-open":
			open = false
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
	return c.serveWeb(game, folder, port, open)
}

// buildWeb builds game for the browser into build/<game>/web/ and puts the
// page and the two JavaScript files next to it. It returns that folder, or ""
// after reporting a failure. The build carries its assets inside, as a dist
// build does: a page has no folder to read them from.
func (c *cli) buildWeb(game string) string {
	dir := c.path("games", game)
	shown := "games/" + game
	const tags = "golib_dist"

	packages, err := c.listPackages(dir, tags)
	if err != nil {
		c.check("fail", "could not inspect "+shown+" (see the Go errors above)")
		return ""
	}
	if isDir(filepath.Join(dir, "assets")) && !embedsAssets(packages) {
		c.check("fail", shown+"/assets/ would be missing from the web build: add "+shown+"/assets.go, as the golib.EmbedAssets documentation shows")
		return ""
	}

	folder := c.path("build", game, "web")
	shownFolder := "build/" + game + "/web/"
	if err := os.MkdirAll(folder, 0o755); err != nil {
		c.check("fail", fmt.Sprintf("cannot create %s: %v", shownFolder, err))
		return ""
	}
	wasm := filepath.Join(folder, game+".wasm")
	_, err = c.runGo(goCall{
		dir:  dir,
		args: []string{"build", "-trimpath", "-tags=" + tags, "-o", wasm, "."},
		env:  []string{"GOOS=js", "GOARCH=wasm"},
	})
	if err != nil {
		c.check("fail", fmt.Sprintf("web build failed for %s (see the Go errors above)", shown))
		return ""
	}
	size, err := os.Stat(wasm)
	if err != nil {
		c.check("fail", fmt.Sprintf("cannot read %s: %v", c.shown(wasm), err))
		return ""
	}
	c.check("ok", fmt.Sprintf("built %s into %s (%s)", shown, c.shown(wasm), fileSize(size.Size())))

	goWasmExec := c.path(".tools", "go", "lib", "wasm", wasmExecFile)
	if err := syncFile(goWasmExec, filepath.Join(folder, wasmExecFile)); err != nil {
		c.check("fail", fmt.Sprintf("cannot copy %s from the Go toolchain: %v", wasmExecFile, err))
		return ""
	}
	glue := c.path("framework", "internal", "device", "web.js")
	if err := syncFile(glue, filepath.Join(folder, webGlueFile)); err != nil {
		c.check("fail", fmt.Sprintf("cannot copy the web backend's %s: %v", webGlueFile, err))
		return ""
	}
	if err := c.writePage(game, folder); err != nil {
		c.check("fail", fmt.Sprintf("cannot write %s%s: %v", shownFolder, webPageFile, err))
		return ""
	}
	c.check("ok", fmt.Sprintf("wrote %s, %s and %s next to it", webPageFile, wasmExecFile, webGlueFile))
	return folder
}

// writePage writes the page that loads the game: a canvas that fills the
// window, the backend, Go's loader, and the game itself.
func (c *cli) writePage(game, folder string) error {
	info, _, err := readGameInfo(c.path("games", game))
	if err != nil {
		info.Title = game
	}
	page := strings.NewReplacer(
		"{{title}}", html.EscapeString(info.Title),
		"{{wasm}}", html.EscapeString(game+".wasm"),
		"{{glue}}", webGlueFile,
		"{{exec}}", wasmExecFile,
	).Replace(webPageTemplate)
	return os.WriteFile(filepath.Join(folder, webPageFile), []byte(page), 0o644)
}

// serveWeb serves folder on this machine until the terminal stops it, and
// opens the game in a browser unless open is false.
func (c *cli) serveWeb(game, folder string, port int, open bool) int {
	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		c.check("fail", fmt.Sprintf("cannot serve on port %d: %v", port, err))
		if port != 0 {
			c.check("info", "another program may be using that port: try golib web --port 0 to pick a free one")
		}
		return c.summary("web")
	}
	address := fmt.Sprintf("http://localhost:%d/", listener.Addr().(*net.TCPAddr).Port)
	c.check("info", fmt.Sprintf("serving %s at %s (press Ctrl+C to stop)", c.shown(folder), address))
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
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{title}}</title>
<style>
	html, body { margin: 0; height: 100%; background: #000; overflow: hidden; }
	#game { display: block; width: 100%; height: 100%; }
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
