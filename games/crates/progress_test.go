package main

import (
	"strings"
	"testing"

	"golib"
)

func TestProgressRecordsBests(t *testing.T) {
	first := &layout{name: "maps/one.tmx"}
	var p progress
	if !p.record(first, 30) {
		t.Error("the first finish isn't a best")
	}
	if p.record(first, 31) {
		t.Error("a finish in more moves is a best")
	}
	if !p.record(first, 25) {
		t.Error("a finish in fewer moves isn't a best")
	}
	if !p.finished(first) || p.Best[first.name] != 25 {
		t.Errorf("progress %+v, want level one's best at 25", p)
	}
	if p.finished(&layout{name: "maps/two.tmx"}) {
		t.Error("a level never played is finished")
	}
}

func TestNoProgressYet(t *testing.T) {
	if err := golib.DeleteData(progressName); err != nil {
		t.Fatal(err)
	}
	s := newSession(nil)
	if len(s.progress.Best) != 0 || s.progress.MusicOff || s.saveErr != nil {
		t.Errorf("with nothing saved: %+v, %v; want no progress, music on and no error", s.progress, s.saveErr)
	}
}

func TestDamagedProgress(t *testing.T) {
	// Progress saved as something else can't be read as progress.
	if err := golib.SaveData(progressName, []string{"not", "progress"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { golib.DeleteData(progressName) })
	s := newSession(nil)
	if s.saveErr == nil || !strings.Contains(s.saveErr.Error(), "damaged") {
		t.Errorf("loading damaged progress: %v; want an error that says so", s.saveErr)
	}
	if len(s.progress.Best) != 0 {
		t.Errorf("damaged progress gave %+v", s.progress)
	}
}
