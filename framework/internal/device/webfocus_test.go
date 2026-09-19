//go:build !js

package device

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The web backend has to take the keyboard focus itself. A site that shows a
// game inside an <iframe>, as itch.io does, sends key events to whichever
// document holds the focus, and a click on the game doesn't move it there:
// the backend cancels that click's default action, which is what would have
// moved it. A game built before it did drew and played its sounds on itch.io
// but never saw a key, and nothing on this machine showed it, because a page
// served on its own already holds the keyboard.
//
// So this check puts the real web.js in a page inside another page, in a
// browser with no window, and asks the outer page what happened, the way the
// bug was found. It needs no game and no web build: the backend is plain
// JavaScript, and golib.open is what a game's first frame calls. The test
// reads web.js from this folder, so go test runs it again whenever the
// backend changes.
//
// It is skipped where there is no browser to run it in.

// webFocusTimeout is how long the browser may take to answer.
const webFocusTimeout = 60 * time.Second

// webFocusResult is what the page around the game reports.
type webFocusResult struct {
	Failed string `json:"failed"` // the check could not be made, and why
	Opened bool   `json:"opened"` // the game opened its window in the page
	Held   bool   `json:"held"`   // the game held the keyboard when it opened
	Stolen bool   `json:"stolen"` // the page around it could take the keyboard away
	Back   bool   `json:"back"`   // a click on the game brought the keyboard back
	Holder string `json:"holder"` // what holds the keyboard inside the game's page
}

// webFocusPage is the page around the game, as itch.io's page is.
const webFocusPage = `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><title>Around the game</title></head>
<body>
<button id="outside">Not the game</button>
<iframe id="frame" src="game.html" width="640" height="360" style="border:0"></iframe>
<script>
	var frame = document.getElementById('frame');
	function report(result) { fetch('/result', { method: 'POST', body: JSON.stringify(result) }); }
	function game() { return frame.contentDocument; }
	function holder() {
		var active = game().activeElement;
		return active ? (active.id || active.tagName) : 'none';
	}
	// The game's page says when it has opened, or why it couldn't.
	function whenOpen(left, then) {
		var ready = frame.contentWindow ? frame.contentWindow.ready : null;
		if (ready === 'open') { then(); return; }
		if (ready) { report({ failed: ready }); return; }
		if (left <= 0) { report({ failed: 'the game never opened in the page' }); return; }
		setTimeout(function () { whenOpen(left - 1, then); }, 100);
	}
	window.addEventListener('load', function () {
		whenOpen(100, function () {
			var result = { opened: true, held: game().hasFocus() };
			// The page takes the keyboard, as the page around a game holds it
			// while the player reads what is on it.
			document.getElementById('outside').focus();
			result.stolen = !game().hasFocus();
			// Then the player clicks the game, which is all they do on
			// itch.io before pressing a key.
			var canvas = game().getElementById('game');
			canvas.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, clientX: 20, clientY: 20 }));
			setTimeout(function () {
				result.back = game().hasFocus();
				result.holder = holder();
				report(result);
			}, 200);
		});
	});
</script>
</body></html>
`

// webFocusGame is the game's own page, as golib dist --web writes it, with
// the backend opened by hand instead of by a game's first frame.
const webFocusGame = `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><title>The game</title>
<style>html, body { margin: 0; height: 100%; } #game { display: block; width: 100%; height: 100%; outline: none; }</style>
</head>
<body>
<canvas id="game"></canvas>
<script src="golib.js"></script>
<script>
	var ready = 'the backend never loaded';
	try {
		window.golib.open(320, 180, 'focus check');
		ready = 'open';
	} catch (e) {
		ready = String(e && e.message ? e.message : e);
	}
</script>
</body></html>
`

func TestAGameInsideAPageTakesTheKeyboard(t *testing.T) {
	browser := findWebBrowser()
	if browser == "" {
		t.Skip("no browser on this machine to run the check in")
	}
	backend, err := os.ReadFile("web.js")
	if err != nil {
		t.Fatalf("cannot read the web backend: %v", err)
	}

	reported := make(chan webFocusResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var result webFocusResult
		if err := json.Unmarshal(body, &result); err != nil {
			result.Failed = "the page sent " + string(body)
		}
		select {
		case reported <- result:
		default:
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, webFocusPage)
		case "/game.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, webFocusGame)
		case "/golib.js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Write(backend)
		default:
			http.NotFound(w, r)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	stop, err := startWebBrowser(browser, server.URL+"/")
	if err != nil {
		t.Skipf("cannot start %s with no window: %v", filepath.Base(browser), err)
	}
	defer stop()

	var result webFocusResult
	select {
	case result = <-reported:
	case <-time.After(webFocusTimeout):
		t.Fatalf("the page never reported back in %v", webFocusTimeout)
	}

	switch {
	case strings.Contains(result.Failed, "WebGL 2"):
		// A browser with no graphics here can say nothing about the keyboard.
		t.Skip("this browser has no WebGL 2 with no window: " + result.Failed)
	case result.Failed != "":
		t.Fatal("the check could not be made: " + result.Failed)
	case !result.Opened:
		t.Fatal("the game never opened in the page")
	}
	if !result.Held {
		t.Error("the game did not hold the keyboard when it opened: the page around it keeps it, and the game sees no key until it is clicked")
	}
	if !result.Stolen {
		// Without this the click proves nothing: the game would have held the
		// keyboard all along.
		t.Skip("the page around the game could not take the keyboard away, so a click on the game proves nothing here")
	}
	if !result.Back {
		t.Error("a click on the game did not bring the keyboard back: a game inside a page, as itch.io shows it, draws and sounds but never sees a key")
	}
	if result.Holder != "game" {
		t.Errorf("the keyboard is held by %q inside the game's page, want the canvas, \"game\"", result.Holder)
	}
}

// findWebBrowser returns a browser that runs a page with no window, or "".
// tools/cli/webshot.go looks in the same places for golib shot --web; the two
// are apart because the CLI and the framework are separate modules, so change
// them together.
func findWebBrowser() string {
	for _, name := range []string{"msedge", "chrome", "chromium", "google-chrome", "chromium-browser"} {
		if found, err := exec.LookPath(name); err == nil {
			return found
		}
	}
	paths := map[string][]string{
		"windows": {
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		},
		"darwin": {
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		},
		"linux": {
			"/usr/bin/google-chrome",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/usr/bin/microsoft-edge",
		},
	}
	for _, path := range paths[runtime.GOOS] {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

// startWebBrowser opens address in a browser with no window, with a profile
// of its own so that it never touches the one the person browses with, and
// returns a function that closes it and clears up after it.
func startWebBrowser(browser, address string) (stop func(), err error) {
	profile, err := os.MkdirTemp("", "golib-webfocus-")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(browser,
		"--headless=new",
		"--user-data-dir="+profile,
		"--no-first-run",
		"--no-default-browser-check",
		"--mute-audio",
		"--window-size=1280,720",
		address,
	)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(profile)
		return nil, err
	}
	return func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		cmd.Wait()
		os.RemoveAll(profile)
	}, nil
}
