//go:build ignore

// Makes the WAV files that jfxr_test.go compares GoLib's sounds with, using
// jfxr's own synthesizer, in a browser with no window:
//
//   - preset-<name>.jfxr: a sound from each of jfxr's presets, from a fixed
//     random seed, saved as jfxr saves sounds.
//   - <name>.wav: the sound jfxr makes from each .jfxr file in this folder,
//     as jfxr exports it.
//
// Run it from the framework folder:
//
//	golib go run testdata/jfxr/make.go
//
// It downloads jfxr's code from GitHub, at the commit framework/jfxr.go
// follows, and needs Microsoft Edge, Google Chrome or Chromium. The browser's
// profile goes in build/jfxr-testdata/.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"
)

const (
	dir    = "testdata/jfxr"
	commit = "e971e4c183beddaff3abcebb77f2eaae17a2b83b"
)

// sources are jfxr's files, in the order the page needs them, with their
// SHA-256 checksums.
var sources = []struct{ name, sum string }{
	{"math.js", "227bb6d7a11ffb09abdaccce2ce524ed688984882718f4343f8a6fd4d51e472d"},
	{"random.js", "a21555124818044410bb5d89576a9fa0075eee12c6a48af9fdda74cbd107e61a"},
	{"clip.js", "ef0ceacdf24876756755f4f337883224352e3054e06dbab2c66164af3e6b1e2b"},
	{"sound.js", "7bd627d9459255f7b99af4155cb015cbb97b9efc201c640ff11565b311418333"},
	{"synth.js", "96283ad712b7ebb16a1516b9821645a30bfc6225c9d599c96c95cc893fbf886e"},
	{"presets.js", "5dd8ac8c046473448ef37ff759f4489c792d354cab8e13169bef6a937cf141e0"},
}

// presets are the presets to save a sound from: the name of the file, the
// name jfxr gives the preset, and how long the sound may last, in seconds,
// to keep the WAV files small. The first seed that makes a short enough
// sound wins.
var presets = []struct {
	File    string  `json:"file"`
	Name    string  `json:"name"`
	Longest float64 `json:"longest"`
}{
	{"coin", "Pickup/coin", 0.2},
	{"laser", "Laser/shoot", 0.2},
	{"explosion", "Explosion", 0.4},
	{"powerup", "Powerup", 0.2},
	{"hit", "Hit/hurt", 0.2},
	{"jump", "Jump", 0.2},
	{"blip", "Blip/select", 0.1},
	{"random", "Random", 0.3},
}

// page runs jfxr's code, then writes what it made into the page, where the
// browser's --dump-dom finds it.
const page = `<!doctype html>
<meta charset="utf-8">
<pre id="out"></pre>
<script>
%s
var files = %s;
var presets = %s;

function render(text) {
  var wav;
  // A setTimeout that runs at once makes the synthesizer finish here.
  new Synth(text, function(f) { f(); }).run(function(clip) {
    var bytes = clip.toWavBytes(), binary = '';
    for (var i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]);
    wav = btoa(binary);
  });
  return wav;
}

var out = document.getElementById('out');
try {
  var results = [];
  presets.forEach(function(p) {
    var preset = ALL_PRESETS.filter(function(q) { return q.name == p.name; })[0];
    for (var seed = 1; ; seed++) {
      var sound = new Sound();
      preset.random = new Random(seed);
      preset.applyTo(sound);
      if (sound.duration() <= p.longest) break;
    }
    sound.name = p.name + ' ' + seed;
    var text = sound.serialize();
    results.push({name: 'preset-' + p.file, jfxr: text, wav: render(text)});
  });
  Object.keys(files).forEach(function(name) {
    results.push({name: name, wav: render(files[name])});
  });
  out.textContent = 'RESULTS ' + JSON.stringify(results) + ' END';
} catch (e) {
  out.textContent = 'ERROR ' + e.stack + ' END';
}
</script>
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "make.go:", err)
		os.Exit(1)
	}
}

func run() error {
	if _, err := os.Stat(dir); err != nil {
		return errors.New("run it from the framework folder: golib go run testdata/jfxr/make.go")
	}
	browser, err := findBrowser()
	if err != nil {
		return err
	}
	code, err := jfxrCode()
	if err != nil {
		return err
	}

	// The .jfxr files written by hand; the preset files are made again.
	names, _ := filepath.Glob(filepath.Join(dir, "*.jfxr"))
	files := map[string]string{}
	for _, name := range names {
		base := strings.TrimSuffix(filepath.Base(name), ".jfxr")
		if strings.HasPrefix(base, "preset-") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		files[base] = string(data)
	}
	filesJSON, _ := json.Marshal(files)
	presetsJSON, _ := json.Marshal(presets)

	work, err := filepath.Abs(filepath.Join("..", "build", "jfxr-testdata"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	pagePath := filepath.Join(work, "page.html")
	if err := os.WriteFile(pagePath, fmt.Appendf(nil, page, code, filesJSON, presetsJSON), 0o644); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pageURL := url.URL{Scheme: "file", Path: filepath.ToSlash(pagePath)}
	if runtime.GOOS == "windows" {
		pageURL.Path = "/" + pageURL.Path
	}
	cmd := exec.CommandContext(ctx, browser, "--headless", "--disable-gpu", "--no-first-run",
		"--no-default-browser-check", "--user-data-dir="+filepath.Join(work, "profile"),
		"--dump-dom", pageURL.String())
	cmd.Stderr = io.Discard
	dom, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("%s: %w", browser, err)
	}

	found := regexp.MustCompile(`(?s)<pre id="out">(RESULTS|ERROR) (.*?) END</pre>`).FindSubmatch(dom)
	if found == nil {
		return fmt.Errorf("the page wrote no results; the browser printed:\n%s", dom)
	}
	text := html.UnescapeString(string(found[2]))
	if string(found[1]) == "ERROR" {
		return errors.New("jfxr failed in the browser: " + text)
	}
	var results []struct{ Name, Jfxr, Wav string }
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		return err
	}
	for _, r := range results {
		wav, err := base64.StdEncoding.DecodeString(r.Wav)
		if err != nil {
			return fmt.Errorf("%s: %w", r.Name, err)
		}
		if r.Jfxr != "" {
			if err := os.WriteFile(filepath.Join(dir, r.Name+".jfxr"), []byte(r.Jfxr), 0o644); err != nil {
				return err
			}
		}
		if err := os.WriteFile(filepath.Join(dir, r.Name+".wav"), wav, 0o644); err != nil {
			return err
		}
		fmt.Printf("%s.wav: %.3f seconds\n", r.Name, float64(len(wav)-44)/2/44100)
	}
	return nil
}

// jfxrCode returns jfxr's files, downloaded and checked, as one script: their
// imports and exports are left out, since they share the page's scope.
func jfxrCode() (string, error) {
	var code strings.Builder
	for _, source := range sources {
		address := "https://raw.githubusercontent.com/ttencate/jfxr/" + commit + "/lib/src/" + source.name
		response, err := http.Get(address)
		if err != nil {
			return "", err
		}
		data, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			return "", err
		}
		if response.StatusCode != http.StatusOK {
			return "", fmt.Errorf("%s: %s", address, response.Status)
		}
		if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != source.sum {
			return "", fmt.Errorf("%s doesn't have the expected checksum", address)
		}
		for line := range strings.Lines(string(data)) {
			if strings.HasPrefix(line, "import ") {
				continue
			}
			code.WriteString(strings.TrimPrefix(line, "export "))
		}
		code.WriteString("\n")
	}
	return code.String(), nil
}

// findBrowser returns the path of a Chromium browser: Edge, Chrome or
// Chromium.
func findBrowser() (string, error) {
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		for _, root := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), os.Getenv("LocalAppData")} {
			if root != "" {
				candidates = append(candidates,
					filepath.Join(root, "Microsoft", "Edge", "Application", "msedge.exe"),
					filepath.Join(root, "Google", "Chrome", "Application", "chrome.exe"))
			}
		}
	case "darwin":
		candidates = []string{
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	}
	for _, name := range []string{"microsoft-edge", "google-chrome", "chromium", "chromium-browser"} {
		if path, err := exec.LookPath(name); err == nil {
			candidates = append(candidates, path)
		}
	}
	if i := slices.IndexFunc(candidates, func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && !info.IsDir()
	}); i >= 0 {
		return candidates[i], nil
	}
	return "", errors.New("found no Microsoft Edge, Google Chrome or Chromium")
}
