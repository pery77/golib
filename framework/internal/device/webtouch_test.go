//go:build !js

package device

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A game in a browser is played on a phone with fingers, so the web backend
// turns the browser's touch events into the fingers Input.Touches reports. This
// check puts the real web.js in a page in a browser with no window, touches the
// canvas the way a phone does, and reads the input buffer the backend fills: the
// fingers' ids, where they are in canvas pixels, which of them landed since the
// last frame, and that a finger that lands and lifts between two frames is still
// reported once.
//
// The page reads the buffer at the places weblayout.go gives it, so the check
// fails if web.js and the Go side ever disagree about where the fingers are.
//
// It is skipped where there is no browser to run it in.

// webTouchResult is what the page reports: one snapshot of the buffer per step.
type webTouchResult struct {
	Failed string             `json:"failed"`
	Steps  []webTouchSnapshot `json:"steps"`

	// What the backend makes of the machine the page is open on, with the
	// browser's answers played by the page: a phone, a computer with a touch
	// screen and a mouse, and a computer with no touch at all.
	PhoneIsTouch   bool `json:"phoneIsTouch"`
	TouchPCIsTouch bool `json:"touchPCIsTouch"`
	PlainPCIsTouch bool `json:"plainPCIsTouch"`
}

// webTouchSnapshot is what one frame of the game would have seen.
type webTouchSnapshot struct {
	What    string           `json:"what"` // what the page did before reading
	Count   int              `json:"count"`
	Fingers []webTouchFinger `json:"fingers"`
}

type webTouchFinger struct {
	ID  int     `json:"id"`
	X   float64 `json:"x"`
	Y   float64 `json:"y"`
	New bool    `json:"new"`
}

// webTouchPage touches the game and reads the buffer after each step. The
// canvas is 640 by 360 CSS pixels, so a touch at client 64, 36 is at 64, 36 in
// canvas pixels on a display with one pixel per CSS pixel, and at twice that on
// one with two: the check works out which from the first finger instead of
// assuming the display it runs on.
//
// The mouse pointer is not in this: package golib is what moves it with the
// oldest finger, above the backend, and input_test.go checks that.
const webTouchPage = `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><title>Fingers</title>
<style>
	html, body { margin: 0; height: 100%; overflow: hidden; background: #000; }
	#game { display: block; width: 640px; height: 360px; outline: none; touch-action: none; }
</style>
</head>
<body>
<canvas id="game"></canvas>
<script src="golib.js"></script>
<script>
	// Where the fingers are in the input buffer, as weblayout.go says.
	const IN_TOUCH_COUNT = {{touchCount}}, IN_TOUCHES = {{touches}};
	const IN_TOUCHES_NEW = {{touchesNew}}, IN_SIZE = {{size}};

	const steps = [];
	function report(result) { fetch('/result', { method: 'POST', body: JSON.stringify(result) }); }

	var buffer = null, floats = null;
	function read(what) {
		window.golib.snapshotInput();
		const count = floats[IN_TOUCH_COUNT];
		const fingers = [];
		for (var i = 0; i < count; i++) {
			const at = IN_TOUCHES + i * 3;
			fingers.push({ id: floats[at], x: floats[at + 1], y: floats[at + 2], new: buffer[IN_TOUCHES_NEW + i] !== 0 });
		}
		steps.push({ what: what, count: count, fingers: fingers });
	}

	// The browser's own events, as a phone sends them. A mouse is left out of
	// every one: it is not a finger.
	function touch(kind, id, x, y) {
		const event = new PointerEvent(kind, {
			bubbles: true, cancelable: true, pointerId: id, pointerType: 'touch',
			clientX: x, clientY: y,
		});
		document.getElementById('game').dispatchEvent(event);
	}

	try {
		window.golib.open(640, 360, 'touch check');
		buffer = window.golib.inputBuffer(IN_SIZE);
		floats = new Float32Array(buffer.buffer, 0, IN_TOUCHES_NEW / 4);

		read('before any touch');
		touch('pointerdown', 11, 64, 36);
		read('one finger down');
		read('the frame after it landed');
		touch('pointermove', 11, 128, 72);
		read('that finger moved');
		touch('pointerdown', 12, 320, 180);
		read('a second finger down');
		touch('pointerup', 11, 128, 72);
		read('the first finger lifted');
		// A quick tap: the finger lands and lifts between two reads.
		touch('pointerdown', 13, 600, 300);
		touch('pointerup', 13, 600, 300);
		read('a tap between two frames');
		read('the frame after the tap');
		// A mouse is not a finger.
		document.getElementById('game').dispatchEvent(new PointerEvent('pointerdown', {
			bubbles: true, pointerId: 1, pointerType: 'mouse', clientX: 10, clientY: 10,
		}));
		read('a mouse click');

		// What kind of machine is this? The backend asks the browser how many
		// fingers it takes and whether its main pointer is coarse, so the page
		// answers for each machine in turn.
		function machine(fingers, coarse) {
			Object.defineProperty(navigator, 'maxTouchPoints', { configurable: true, get: function () { return fingers; } });
			window.matchMedia = function (query) {
				return { matches: query.indexOf('coarse') >= 0 ? coarse : !coarse, media: query };
			};
			return window.golib.touchScreen();
		}
		const wasMatchMedia = window.matchMedia;
		const phoneIsTouch = machine(5, true);       // a phone: fingers, coarse
		const touchPCIsTouch = machine(10, false);   // a laptop with a touch screen and a mouse
		const plainPCIsTouch = machine(0, false);    // a computer with no touch at all
		window.matchMedia = wasMatchMedia;
		delete navigator.maxTouchPoints;

		report({
			steps: steps,
			phoneIsTouch: phoneIsTouch,
			touchPCIsTouch: touchPCIsTouch,
			plainPCIsTouch: plainPCIsTouch,
		});
	} catch (e) {
		report({ failed: String(e && e.message ? e.message : e) });
	}
</script>
</body></html>
`

func TestTheWebBackendReadsFingers(t *testing.T) {
	browser := findWebBrowser()
	if browser == "" {
		t.Skip("no browser on this machine to run the check in")
	}
	backend, err := os.ReadFile("web.js")
	if err != nil {
		t.Fatalf("cannot read the web backend: %v", err)
	}
	page := strings.NewReplacer(
		"{{touchCount}}", fmt.Sprint(inTouchCount),
		"{{touches}}", fmt.Sprint(inTouches),
		"{{touchesNew}}", fmt.Sprint(inTouchesNew),
		"{{size}}", fmt.Sprint(inSize),
	).Replace(webTouchPage)

	reported := make(chan webTouchResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var result webTouchResult
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
			io.WriteString(w, page)
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

	var result webTouchResult
	select {
	case result = <-reported:
	case <-time.After(webFocusTimeout):
		t.Fatalf("the page never reported back in %v", webFocusTimeout)
	}
	if strings.Contains(result.Failed, "WebGL 2") {
		t.Skip("this browser has no WebGL 2 with no window: " + result.Failed)
	}
	if result.Failed != "" {
		t.Fatal("the check could not be made: " + result.Failed)
	}
	const steps = 9
	if len(result.Steps) != steps {
		t.Fatalf("the page reported %d steps, want %d: %+v", len(result.Steps), steps, result.Steps)
	}

	// How many pixels the display has per CSS pixel, worked out from the finger
	// the page put at client 64, 36.
	first := result.Steps[1]
	if first.Count != 1 {
		t.Fatalf("one finger down reads %d fingers: %+v", first.Count, first)
	}
	scale := first.Fingers[0].X / 64
	if scale <= 0 {
		t.Fatalf("a finger at client 64, 36 reads x %v, want it in canvas pixels", first.Fingers[0].X)
	}
	at := func(x, y float64) (float64, float64) { return x * scale, y * scale }

	wantX, wantY := at(64, 36)
	if got := first.Fingers[0]; got.X != wantX || got.Y != wantY || !got.New {
		t.Errorf("one finger down reads %+v, want it landing at %v, %v", got, wantX, wantY)
	}
	held := result.Steps[2]
	if held.Count != 1 || held.Fingers[0].New {
		t.Errorf("the frame after it landed reads %+v, want the finger still down and not landing again", held)
	}
	if held.Fingers[0].ID != first.Fingers[0].ID {
		t.Errorf("the finger's id changed from %d to %d while it stayed down", first.Fingers[0].ID, held.Fingers[0].ID)
	}

	moved := result.Steps[3]
	wantX, wantY = at(128, 72)
	if moved.Count != 1 || moved.Fingers[0].X != wantX || moved.Fingers[0].Y != wantY {
		t.Errorf("the finger moved reads %+v, want it at %v, %v", moved, wantX, wantY)
	}

	second := result.Steps[4]
	if second.Count != 2 {
		t.Fatalf("a second finger down reads %d fingers: %+v", second.Count, second)
	}
	if second.Fingers[0].ID != first.Fingers[0].ID || second.Fingers[1].ID == second.Fingers[0].ID {
		t.Errorf("two fingers read as %+v, want the first one first, with an id of its own each", second.Fingers)
	}
	if second.Fingers[0].New || !second.Fingers[1].New {
		t.Errorf("two fingers read as %+v, want only the second one landing", second.Fingers)
	}

	lifted := result.Steps[5]
	if lifted.Count != 1 || lifted.Fingers[0].ID != second.Fingers[1].ID {
		t.Errorf("after the first finger lifted, the fingers are %+v, want the second one alone", lifted.Fingers)
	}

	tap := result.Steps[6]
	if tap.Count != 2 {
		t.Fatalf("a tap between two frames reads %d fingers, want the finger still down and the tap: %+v", tap.Count, tap)
	}
	wantX, wantY = at(600, 300)
	if got := tap.Fingers[1]; !got.New || got.X != wantX || got.Y != wantY {
		t.Errorf("the tap reads %+v, want it landing at %v, %v", got, wantX, wantY)
	}
	after := result.Steps[7]
	if after.Count != 1 {
		t.Errorf("the frame after the tap reads %d fingers, want only the finger still down: %+v", after.Count, after)
	}

	mouse := result.Steps[8]
	if mouse.Count != after.Count {
		t.Errorf("a mouse click added a finger: %+v, want the %d fingers before it", mouse, after.Count)
	}

	// Whether a game starts with its on-screen controls showing is this
	// answer: yes on a phone, no on a computer, even one with a touch screen,
	// where a finger on the game turns them on instead.
	if !result.PhoneIsTouch {
		t.Error("a phone, which takes fingers and has a coarse pointer, is not played with touch: a game there would open with no controls to tap")
	}
	if result.TouchPCIsTouch {
		t.Error("a computer with a touch screen and a mouse is played with touch: every game would cover itself with on-screen controls there")
	}
	if result.PlainPCIsTouch {
		t.Error("a computer with no touch at all is played with touch")
	}
}
