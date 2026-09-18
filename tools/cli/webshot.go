package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// shotWebPath is where a web build posts its screenshots, and what the web
// backend's SavePicture sends them to. A game that stops says so on
// shotWebErrorPath, so that golib shot says why instead of only that the
// screenshots never came.
const (
	shotWebPath      = "/golib-shot"
	shotWebErrorPath = "/golib-shot-error"
)

// shotWebTimeout is how long a web build may take to send every screenshot
// before the browser is stopped.
const shotWebTimeout = 120 * time.Second

// shotWeb takes screenshots of a web build: it serves the game on this
// machine, opens it in a browser with no window, and saves the pictures the
// page posts back into build/<game>/shots-web/. A page cannot write files, so
// this is how a web build is checked the way a desktop one is.
func (c *cli) shotWeb(game string, frames []int, input string, scale int) int {
	browser, err := findBrowser()
	if err != nil {
		c.check("fail", err.Error())
		return c.summary("shot")
	}
	folder := c.buildWeb(game)
	if folder == "" {
		return c.summary("shot")
	}
	shots := c.path("build", game, "shots-web")
	if err := os.RemoveAll(shots); err == nil {
		err = os.MkdirAll(shots, 0o755)
	}
	if err != nil {
		c.check("fail", fmt.Sprintf("cannot empty build/%s/shots-web/: %v", game, err))
		return c.summary("shot")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		c.check("fail", fmt.Sprintf("cannot serve the game on this machine: %v", err))
		return c.summary("shot")
	}
	arrived := make(chan string, len(frames)+8)
	stopped := make(chan string, 1)
	server := &http.Server{Handler: shotHandler(folder, shots, arrived, stopped)}
	go server.Serve(listener)
	defer server.Close()

	address := fmt.Sprintf("http://localhost:%d/?%s", listener.Addr().(*net.TCPAddr).Port,
		shotSettings(frames, input, scale))
	extras := ""
	if input != "" {
		extras += ", playing " + shortInput(input)
	}
	if scale > 1 {
		extras += fmt.Sprintf(", enlarged %d times", scale)
	}
	c.check("info", fmt.Sprintf("running %s for %d frame(s) in %s, with no window%s",
		game, frames[len(frames)-1], filepath.Base(browser), extras))

	stop, err := startBrowser(browser, address)
	if err != nil {
		c.check("fail", fmt.Sprintf("cannot start %s: %v", browser, err))
		return c.summary("shot")
	}
	defer stop()
	waited := c.waitForShots(game, len(frames), arrived, stopped)

	for _, frame := range frames {
		name := fmt.Sprintf("frame-%06d.png", frame)
		if isFile(filepath.Join(shots, name)) {
			c.check("ok", fmt.Sprintf("frame %d: build/%s/shots-web/%s", frame, game, name))
		} else if waited {
			c.check("fail", fmt.Sprintf("frame %d: no screenshot came back", frame))
		}
	}
	return c.summary("shot")
}

// waitForShots waits for the page to post every screenshot, and reports
// whether they all arrived. A game that never finishes is given up on, so
// whoever waits for shot isn't stuck.
func (c *cli) waitForShots(game string, want int, arrived <-chan string, stopped <-chan string) bool {
	deadline := time.After(shotWebTimeout)
	for got := 0; got < want; got++ {
		select {
		case <-arrived:
		case message := <-stopped:
			c.check("fail", fmt.Sprintf("%s stopped after %d of %d screenshots: %s", game, got, want, message))
			return false
		case <-deadline:
			c.check("fail", fmt.Sprintf("%s sent %d of %d screenshots within %d seconds and was stopped (does Update or Draw loop forever, or does the game stop on something a browser cannot do?)",
				game, got, want, int(shotWebTimeout.Seconds())))
			return false
		}
	}
	return true
}

// shotSettings turns the shot into the settings a web build reads, which the
// page puts where a game on the desktop finds them in its environment.
func shotSettings(frames []int, input string, scale int) string {
	list := make([]string, len(frames))
	for i, frame := range frames {
		list[i] = strconv.Itoa(frame)
	}
	settings := url.Values{}
	// The folder is where a game on the desktop would write; a web build
	// sends the pictures back instead, named after the files it would write.
	settings.Set("GOLIB_SHOT_DIR", "shots")
	settings.Set("GOLIB_SHOT_FRAMES", strings.Join(list, ","))
	if input != "" {
		settings.Set("GOLIB_SHOT_INPUT", input)
	}
	if scale > 1 {
		settings.Set("GOLIB_SHOT_SCALE", strconv.Itoa(scale))
	}
	return settings.Encode()
}

// shotHandler serves the web build and writes the screenshots it posts back
// into shots, telling arrived about each one.
func shotHandler(folder, shots string, arrived, stopped chan<- string) http.Handler {
	files := webFiles(folder)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == shotWebErrorPath {
			message, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
			select {
			case stopped <- strings.TrimSpace(string(message)):
			default:
			}
			return
		}
		if r.URL.Path != shotWebPath {
			files.ServeHTTP(w, r)
			return
		}
		name := filepath.Base(r.URL.Query().Get("name"))
		if r.Method != http.MethodPost || name == "." || name == string(filepath.Separator) ||
			!strings.HasSuffix(name, ".png") {
			http.Error(w, "post a .png named by its frame", http.StatusBadRequest)
			return
		}
		picture, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<20))
		if err == nil {
			err = os.WriteFile(filepath.Join(shots, name), picture, 0o644)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		select {
		case arrived <- name:
		default:
		}
	})
}

// browserNames are the browsers that can run a web build with no window, in
// the order they are looked for. They are Chrome's family: they take the same
// options, and every one of them has WebGL 2.
var browserNames = []string{"msedge", "chrome", "google-chrome", "chromium", "chromium-browser"}

// browserPaths are where those browsers sit when they aren't on the PATH.
var browserPaths = map[string][]string{
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

// findBrowser returns a browser that can run a web build with no window, or
// says that none was found and what to do about it.
func findBrowser() (string, error) {
	for _, name := range browserNames {
		if found, err := exec.LookPath(name); err == nil {
			return found, nil
		}
	}
	for _, path := range browserPaths[runtime.GOOS] {
		if isFile(path) {
			return path, nil
		}
	}
	return "", fmt.Errorf("shot --web found no browser to run the game in: install Microsoft Edge, Google Chrome or Chromium, which golib starts with no window to take the screenshots (Windows 10 and 11 come with Edge)")
}

// startBrowser opens address in a browser with no window, and returns a
// function that closes it and clears up after it. The browser gets a profile
// folder of its own, so that it never touches the one the person browses
// with, and starts clean every time.
func startBrowser(browser, address string) (stop func(), err error) {
	profile, err := os.MkdirTemp("", "golib-shot-")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(browser,
		"--headless=new",
		"--user-data-dir="+profile,
		"--no-first-run",
		"--no-default-browser-check",
		// A game plays its sounds; nobody is listening, and browsers wait for
		// a click before making any.
		"--autoplay-policy=no-user-gesture-required",
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
