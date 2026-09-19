package golib

import (
	"bytes"
	"strings"
	"testing"
)

// browserPlays is what a browser decodes, which is what the web backend's
// device.PlayableFormats returns. The tests below set playableFormats to it to
// try, on a desktop, what a game does there.
var browserPlays = []string{".ogg", ".mp3", ".wav"}

// playsHere makes this machine play formats and collects what GoLib says about
// music it cannot play, for the length of one test.
func playsHere(t *testing.T, formats []string) *bytes.Buffer {
	t.Helper()
	said := &bytes.Buffer{}
	wasPlayable, wasWarnings := playableFormats, warnings
	playableFormats, warnings = formats, said
	t.Cleanup(func() { playableFormats, warnings = wasPlayable, wasWarnings })
	return said
}

// A machine that plays the music's own format plays the file the game named,
// and says nothing.
func TestMusicPlaysTheFileTheGameNamed(t *testing.T) {
	useAssets(t, map[string][]byte{
		"music/theme.xm":  []byte("tracker"),
		"music/theme.ogg": []byte("vorbis"),
	})
	said := playsHere(t, musicFormats)
	file, err := NewMusic("music/theme.xm").playableFile()
	if file != "music/theme.xm" || err != nil {
		t.Errorf("playableFile = %q, %v, want music/theme.xm and no error", file, err)
	}
	if said.Len() != 0 {
		t.Errorf("it said %q, want nothing", said)
	}
}

// Where the music's own format doesn't play, a file beside it with the same
// name in a format that does play takes its place: this is what a web build
// does with the .xm music a game plays on the desktop.
func TestMusicPlaysAnotherFileWhereItsFormatDoesNot(t *testing.T) {
	useAssets(t, map[string][]byte{
		"music/theme.xm":  []byte("tracker"),
		"music/theme.mp3": []byte("mpeg"),
		"music/theme.ogg": []byte("vorbis"),
		"theme.xm":        []byte("tracker"),
		"theme.wav":       []byte("riff"),
	})
	said := playsHere(t, browserPlays)
	// .ogg comes first in what the machine plays, so it wins over the .mp3.
	file, err := NewMusic("music/theme.xm").playableFile()
	if file != "music/theme.ogg" || err != nil {
		t.Errorf("playableFile = %q, %v, want music/theme.ogg and no error", file, err)
	}
	// Music at the top of the assets folder is looked up there.
	file, err = NewMusic("theme.xm").playableFile()
	if file != "theme.wav" || err != nil {
		t.Errorf("playableFile = %q, %v, want theme.wav and no error", file, err)
	}
	for _, want := range []string{`golib.NewMusic("music/theme.xm")`, ".xm files don't play", "music/theme.ogg", "theme.wav"} {
		if !strings.Contains(said.String(), want) {
			t.Errorf("it said:\n%s\nwant a line naming %q", said, want)
		}
	}
}

// With nothing beside it to play instead, the music is simply not heard: the
// game runs on, and GoLib says what to do about it.
func TestMusicIsSilentWhereNothingBesideItPlays(t *testing.T) {
	useAssets(t, map[string][]byte{"music/theme.xm": []byte("tracker")})
	said := playsHere(t, browserPlays)
	theme := NewMusic("music/theme.xm")
	if theme.load() || !theme.silent {
		t.Errorf("load = %v, silent %v, want false and silent", theme.load(), theme.silent)
	}
	if err := theme.Err(); err != nil {
		t.Errorf("Err = %v, want none: music this machine cannot play doesn't stop the game", err)
	}
	if err := takeError(); err != nil {
		t.Errorf("reported error: %v, want none", err)
	}
	for _, want := range []string{"music/theme.ogg", "music/theme.mp3", "music/theme.wav", "runs without this music"} {
		if !strings.Contains(said.String(), want) {
			t.Errorf("it said:\n%s\nwant a line naming %q", said, want)
		}
	}
	// It says so once, not every frame.
	lines := said.Len()
	theme.Play()
	theme.load()
	if said.Len() != lines {
		t.Errorf("it said it again:\n%s", said)
	}
	theme.unload()
	if theme.silent {
		t.Error("unload left the music silent: a game that runs again reads it again")
	}
}

// A format GoLib plays nowhere, and a folder that isn't there, are still
// mistakes that stop Run.
func TestMusicStopsRunForRealMistakes(t *testing.T) {
	useAssets(t, map[string][]byte{"music/theme.xm": []byte("tracker")})
	playsHere(t, browserPlays)
	tests := map[string]string{
		"music/theme.it": `golib.NewMusic("music/theme.it"): GoLib cannot play ".it" files: use one of .ogg, .mp3, .wav, .qoa, .xm, .mod`,
		"tracks/old.xm":  `golib.NewMusic("tracks/old.xm"): golib.ListAssets: assets/tracks not found`,
	}
	for name, want := range tests {
		file, err := NewMusic(name).playableFile()
		if file != "" || err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("playableFile(%q) = %q, %v, want an error containing %q", name, file, err, want)
		}
	}
}

// The whole path, with a sound device: a game asks for music this machine
// doesn't play, and hears the file beside it.
func TestMusicPlaysAnotherFileWithADevice(t *testing.T) {
	if audio.isReady() {
		t.Skip("a sound device is open")
	}
	SetVolume(0) // silent
	audio.open()
	t.Cleanup(func() {
		audio.close()
		SetVolume(1)
	})
	if !audio.isReady() {
		t.Skip("this machine has no sound device")
	}
	useAssets(t, map[string][]byte{
		"music/theme.xm":  []byte("not a tracker module at all"),
		"music/theme.wav": wav(SoundSpec{Duration: 0.2}.resolve().samples()),
	})
	playsHere(t, browserPlays)
	theme := NewMusic("music/theme.xm")
	theme.SetVolume(0.5)
	theme.Play()
	t.Cleanup(theme.unload)
	if !theme.Playing() || theme.Err() != nil {
		t.Fatalf("playing %v, error %v, want it playing", theme.Playing(), theme.Err())
	}
	if err := audio.updateMusic(); err != nil {
		t.Errorf("updating the music: %v", err)
	}
	if err := takeError(); err != nil {
		t.Errorf("reported error: %v", err)
	}
}
