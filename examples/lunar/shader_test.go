//go:build !js && !golib_dist

package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golib"
)

var shaderReloadCheck = flag.Bool("shader-reload-check", false, "check live F5 reload in a hidden graphics window")

func TestShaderSourceLocations(t *testing.T) {
	root := t.TempDir()
	game := filepath.Join(root, "games", "lunar")
	if err := os.MkdirAll(filepath.Join(game, "shaders"), 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(game, "shaders", "cinema.fs")
	if err := os.WriteFile(file, []byte("first edit"), 0644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "build", "lunar", "lunar.exe")
	for _, cwd := range []string{game, root, filepath.Dir(exe)} {
		got, err := readShaderFrom(cwd, exe)
		if err != nil || got != "first edit" {
			t.Fatalf("read from %s: %q, %v", cwd, got, err)
		}
	}
	if err := os.WriteFile(file, []byte("second edit"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, err := readShaderFrom(game, exe); err != nil || got != "second edit" {
		t.Fatalf("stale source: %q %v", got, err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if _, err := readShaderFrom(game, exe); err == nil {
		t.Fatal("missing shader did not return an error")
	}
}

// This uses real GLSL compilation and the real scripted F5 key, not a mock of
// the reload path. Source edits stay inside the test's temporary directory.
// Run with: golib go -C examples/lunar test -run TestLiveShaderReload -args -shader-reload-check
func TestLiveShaderReload(t *testing.T) {
	if !*shaderReloadCheck {
		t.Skip("use -shader-reload-check to open a hidden graphics window")
	}
	// Tests run in their own goroutines; keep the graphics context on one OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir("shaders", 0755); err != nil {
		t.Fatal(err)
	}
	oldCinema := cinema
	cinema = newCinemaShader(probeShader("1.0, 0.0, 0.0"))
	t.Cleanup(func() { cinema = oldCinema; golib.SetPostProcess() })
	a := &session{progress: newCampaign()}
	a.progress.Credits = 12345
	a.progress.Deliveries = 9
	a.applySettings()
	probe := &shaderProbe{app: a}
	t.Setenv("GOLIB_SHOT_DIR", dir)
	t.Setenv("GOLIB_SHOT_FRAMES", "1,2,4,6,8,9,10")
	t.Setenv("GOLIB_SHOT_INPUT", "F5@2 F5@4 F5@6 F2@7 F5@8 F2@9 F5@10")
	t.Setenv("GOLIB_SHOT_SAVE", "")
	t.Setenv("GOLIB_SHOT_SCALE", "1")
	if err := golib.Run(probe, golib.Config{Title: "Shader reload check", Width: 32, Height: 32}); err != nil {
		t.Fatal(err)
	}
	if probe.err != nil {
		t.Fatal(probe.err)
	}
	if a.progress.Credits != 12345 || a.progress.Deliveries != 9 {
		t.Fatal("F5 reset campaign progress")
	}
	if !strings.Contains(probe.failureNotice, "Previous effect kept") {
		t.Fatalf("missing compile error feedback: %q", probe.failureNotice)
	}
	if !strings.Contains(probe.disabledNotice, "Press F2") {
		t.Fatalf("reload enabled an intentionally disabled effect: %q", probe.disabledNotice)
	}
	if !strings.Contains(a.notice, "Cannot read") {
		t.Fatalf("missing source did not report an error: %q", a.notice)
	}
	want := map[int][3]uint32{1: {255, 0, 0}, 2: {0, 255, 0}, 4: {0, 255, 0}, 6: {0, 0, 255}, 8: {255, 255, 255}, 9: {0, 255, 0}, 10: {0, 255, 0}}
	for frame, c := range want {
		f, err := os.Open(filepath.Join(dir, fmt.Sprintf("frame-%06d.png", frame)))
		if err != nil {
			t.Fatal(err)
		}
		im, err := png.Decode(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		r, g, b, _ := im.At(16, 16).RGBA()
		if got := ([3]uint32{r >> 8, g >> 8, b >> 8}); got != c {
			t.Errorf("frame %d: got %v, want %v", frame, got, c)
		}
	}
}

func probeShader(color string) string {
	return "#version 330\nuniform vec2 frameSizeRCP;\nout vec4 finalColor;\nvoid main(){finalColor=vec4(vec3(" + color + ")*(frameSizeRCP.x*1280.0),1.0);}"
}

type shaderProbe struct {
	app                           *session
	frame                         int
	err                           error
	failureNotice, disabledNotice string
}

func (p *shaderProbe) Update(in *golib.Input, dt float32) {
	p.frame++
	p.app.update(in, dt)
	var source string
	switch p.frame {
	case 1:
		source = probeShader("0.0, 1.0, 0.0")
	case 3:
		source = "#version 330\nTHIS IS INTENTIONALLY INVALID GLSL"
	case 4:
		p.failureNotice = p.app.notice
	case 5:
		source = probeShader("0.0, 0.0, 1.0")
	case 7:
		source = probeShader("0.0, 1.0, 0.0")
	case 8:
		p.disabledNotice = p.app.notice
	case 9:
		p.err = os.Remove(filepath.Join("shaders", "cinema.fs"))
	}
	if source != "" {
		p.err = os.WriteFile(filepath.Join("shaders", "cinema.fs"), []byte(source), 0644)
	}
}
func (p *shaderProbe) Draw(screen *golib.Screen) { screen.Clear(golib.White) }
