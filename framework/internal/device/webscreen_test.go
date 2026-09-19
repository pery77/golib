//go:build !js

package device

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A game that goes fullscreen on a phone has to fill the phone's screen. The
// first web build uploaded to itch.io did not: in fullscreen, with the phone
// turned on its side, the game sat in part of the screen instead of the middle
// of it. The backend showed the canvas fullscreen, and a fullscreen element on
// a phone keeps the shape its box had until the browser lays the page out
// again, so the drawing buffer stayed the shape the phone had before it
// turned, and the game was drawn into a corner of the screen.
//
// So the backend now shows the whole page fullscreen and, while it is the one
// that asked for it, measures the canvas against the screen instead of against
// its own box. This check plays the browser's part, because fullscreen needs a
// real tap and a browser with no window has no screen to fill: the game's page
// records what the backend asks to show fullscreen and answers as a browser
// that granted it, its stylesheet leaves the canvas's box the shape a phone in
// portrait gave it, the page around it turns the phone by resizing it, and the
// check asks how large the game is drawn.
//
// It is skipped where there is no browser to run it in.

// webScreenResult is what the page around the game reports.
type webScreenResult struct {
	Failed string  `json:"failed"` // the check could not be made, and why
	Asked  string  `json:"asked"`  // what the backend asked to show fullscreen
	Ratio  float64 `json:"ratio"`  // the display's pixels per CSS pixel

	// The game's page after the phone turned, in CSS pixels.
	InnerWidth  int `json:"innerWidth"`
	InnerHeight int `json:"innerHeight"`

	// The canvas's own box then, which a phone leaves as it was.
	BoxWidth  int `json:"boxWidth"`
	BoxHeight int `json:"boxHeight"`

	// The drawing buffer in fullscreen, and after leaving it.
	FullWidth  int `json:"fullWidth"`
	FullHeight int `json:"fullHeight"`
	BackWidth  int `json:"backWidth"`
	BackHeight int `json:"backHeight"`

	// Whether the backend noticed the player leaving fullscreen: before they
	// did, after they did, and after the game asked for it again.
	LostWhileIn    bool `json:"lostWhileIn"`
	LostAfterLeft  bool `json:"lostAfterLeft"`
	LostAfterAsked bool `json:"lostAfterAsked"`
}

// The portrait phone the game opens in, in CSS pixels.
const webScreenWidth, webScreenHeight = 412, 892

// webScreenPage is the page around the game, which turns the phone.
const webScreenPage = `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><title>The phone</title>
<style>html, body { margin: 0; } #frame { border: 0; display: block; }</style>
</head>
<body>
<iframe id="frame" src="game.html" width="412" height="892"></iframe>
<script>
	var frame = document.getElementById('frame');
	function report(result) { fetch('/result', { method: 'POST', body: JSON.stringify(result) }); }
	function game() { return frame.contentWindow; }
	function canvas() { return frame.contentDocument.getElementById('game'); }
	function tap() {
		canvas().dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, clientX: 20, clientY: 20 }));
	}
	// Every frame the game draws measures the canvas first.
	function drawFrame(then) {
		game().golib.onFrame(then);
		game().golib.askForFrame();
	}
	function whenOpen(left, then) {
		var ready = frame.contentWindow ? frame.contentWindow.ready : null;
		if (ready === 'open') { then(); return; }
		if (ready) { report({ failed: ready }); return; }
		if (left <= 0) { report({ failed: 'the game never opened in the page' }); return; }
		setTimeout(function () { whenOpen(left - 1, then); }, 100);
	}
	window.addEventListener('load', function () {
		whenOpen(100, function () {
			var result = { ratio: game().devicePixelRatio || 1 };
			// The game asks for fullscreen and the player taps it, which is
			// all a phone can answer with: there is no key to press, and a
			// browser grants fullscreen only while it handles one or the other.
			game().golib.setFullscreen(true);
			tap();
			var asked = game().askedFullscreen();
			result.asked = asked ? (asked.id || asked.tagName) : 'nothing';
			// The phone turns on its side. Its screen is landscape now, while
			// the canvas's own box is still the portrait one, as a phone
			// leaves a fullscreen element until it lays the page out again.
			frame.width = 892;
			frame.height = 412;
			drawFrame(function () {
				result.innerWidth = game().innerWidth;
				result.innerHeight = game().innerHeight;
				result.boxWidth = canvas().clientWidth;
				result.boxHeight = canvas().clientHeight;
				result.fullWidth = canvas().width;
				result.fullHeight = canvas().height;
				// The player leaves fullscreen themselves. The game has to hear
				// about it, or it would think it is still fullscreen and its own
				// fullscreen button would do nothing.
				result.lostWhileIn = game().golib.fullscreenLost();
				game().leaveFullscreen();
				result.lostAfterLeft = game().golib.fullscreenLost();
				game().golib.setFullscreen(true);
				result.lostAfterAsked = game().golib.fullscreenLost();
				// Out of fullscreen the page sizes the canvas again.
				game().golib.setFullscreen(false);
				tap();
				drawFrame(function () {
					result.backWidth = canvas().width;
					result.backHeight = canvas().height;
					report(result);
				});
			});
		});
	});
</script>
</body></html>
`

// webScreenGame is the game's own page, with a canvas its stylesheet keeps the
// size a phone in portrait gave it, and a browser's fullscreen played by hand:
// a tap dispatched by a test is not the real one a browser insists on, and a
// browser with no window has no screen to fill.
const webScreenGame = `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><title>The game</title>
<style>
	html, body { margin: 0; height: 100%; overflow: hidden; background: #000; }
	#game { display: block; width: 412px; height: 892px; outline: none; }
</style>
</head>
<body>
<canvas id="game"></canvas>
<script>
	var asked = null;
	function askedFullscreen() { return asked; }
	Element.prototype.requestFullscreen = function () {
		asked = this;
		Object.defineProperty(document, 'fullscreenElement', {
			configurable: true, get: function () { return asked; },
		});
		return Promise.resolve();
	};
	document.exitFullscreen = function () { asked = null; return Promise.resolve(); };
	// The player leaving fullscreen themselves, with Esc or a phone's gesture.
	function leaveFullscreen() {
		asked = null;
		document.dispatchEvent(new Event('fullscreenchange'));
	}
</script>
<script src="golib.js"></script>
<script>
	var ready = 'the backend never loaded';
	try {
		window.golib.open(1280, 720, 'screen check');
		ready = 'open';
	} catch (e) {
		ready = String(e && e.message ? e.message : e);
	}
</script>
</body></html>
`

func TestAGameFullscreenOnAPhoneFillsTheScreen(t *testing.T) {
	browser := findWebBrowser()
	if browser == "" {
		t.Skip("no browser on this machine to run the check in")
	}
	backend, err := os.ReadFile("web.js")
	if err != nil {
		t.Fatalf("cannot read the web backend: %v", err)
	}

	reported := make(chan webScreenResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var result webScreenResult
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
			io.WriteString(w, webScreenPage)
		case "/game.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, webScreenGame)
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

	var result webScreenResult
	select {
	case result = <-reported:
	case <-time.After(webFocusTimeout):
		t.Fatalf("the page never reported back in %v", webFocusTimeout)
	}
	if strings.Contains(result.Failed, "WebGL 2") {
		// A browser with no graphics here can say nothing about the screen.
		t.Skip("this browser has no WebGL 2 with no window: " + result.Failed)
	}
	if result.Failed != "" {
		t.Fatal("the check could not be made: " + result.Failed)
	}

	// The page goes fullscreen, not the canvas: that is what follows a phone
	// when it turns, and what keeps a message over the game in sight.
	if result.Asked != "HTML" {
		t.Errorf("the backend asked to show %q fullscreen, want the page itself, %q", result.Asked, "HTML")
	}
	// The game fills the screen the turned phone leaves, however wrong the
	// shape of the canvas's own box still is.
	wantWidth := int(math.Round(float64(result.InnerWidth) * result.Ratio))
	wantHeight := int(math.Round(float64(result.InnerHeight) * result.Ratio))
	if result.FullWidth != wantWidth || result.FullHeight != wantHeight {
		t.Errorf("fullscreen on a phone turned to %dx%d CSS pixels draws %dx%d, want %dx%d: the canvas's own box was still %dx%d, the shape the phone had before it turned",
			result.InnerWidth, result.InnerHeight, result.FullWidth, result.FullHeight,
			wantWidth, wantHeight, result.BoxWidth, result.BoxHeight)
	}
	// The backend tells the game when the player leaves the fullscreen it asked
	// for, and forgets it once the game asks again.
	if result.LostWhileIn {
		t.Error("the backend says fullscreen was lost while the game was still in it")
	}
	if !result.LostAfterLeft {
		t.Error("the player left fullscreen and the backend didn't notice: the game would think it is still fullscreen, and its own fullscreen button would do nothing")
	}
	if result.LostAfterAsked {
		t.Error("the game asked for fullscreen again and the backend still says it was lost")
	}
	// Out of fullscreen the page decides again how large the game is drawn.
	wantBackWidth := int(math.Round(webScreenWidth * result.Ratio))
	wantBackHeight := int(math.Round(webScreenHeight * result.Ratio))
	if result.BackWidth != wantBackWidth || result.BackHeight != wantBackHeight {
		t.Errorf("out of fullscreen the game draws %dx%d, want the %dx%d its page asks for: the backend kept the size it had given the canvas for the screen",
			result.BackWidth, result.BackHeight, wantBackWidth, wantBackHeight)
	}
}
