package main

import (
	"errors"
	"testing"

	"golib"
)

// newTestSession returns a session with three one-crate levels and nothing
// saved: tests keep saved data in memory, shared by the tests of the game.
func newTestSession(t *testing.T) *session {
	t.Helper()
	var levels []*layout
	for _, name := range []string{"a", "b", "c"} {
		l := parseLayout(t, "#####", "#@$.#", "#####")
		l.name, l.par = name, 2
		levels = append(levels, l)
	}
	if err := golib.DeleteData(progressName); err != nil {
		t.Fatal(err)
	}
	return newSession(levels)
}

func TestLevelsUnlockInOrder(t *testing.T) {
	s := newTestSession(t)
	if !s.unlocked(0) || s.unlocked(1) || s.unlocked(2) {
		t.Errorf("with no progress, unlocked: %v %v %v; want only the first", s.unlocked(0), s.unlocked(1), s.unlocked(2))
	}
	if next := s.nextLevel(); next != 0 {
		t.Errorf("with no progress, the next level is %d, want 0", next)
	}
	s.finish(s.levels[0], 3)
	if !s.unlocked(1) || s.unlocked(2) {
		t.Errorf("after finishing the first level, unlocked: %v %v; want the second only", s.unlocked(1), s.unlocked(2))
	}
	if next := s.nextLevel(); next != 1 {
		t.Errorf("after finishing the first level, the next level is %d, want 1", next)
	}
	if finished, atPar := s.finishedLevels(); finished != 1 || atPar != 0 {
		t.Errorf("finished %d, %d at par; want 1 and 0", finished, atPar)
	}
}

func TestProgressSurvivesARestart(t *testing.T) {
	s := newTestSession(t)
	s.finish(s.levels[0], 2)
	s.finish(s.levels[1], 2)
	s.toggleMusic()
	if s.saveErr != nil {
		t.Fatal(s.saveErr)
	}

	again := newSession(s.levels)
	if next := again.nextLevel(); next != 2 {
		t.Errorf("after a restart, the game continues from level %d, want 2", next)
	}
	if finished, atPar := again.finishedLevels(); finished != 2 || atPar != 2 {
		t.Errorf("after a restart: finished %d, %d at par; want 2 and 2", finished, atPar)
	}
	if again.musicOn() {
		t.Error("the music is on again after a restart")
	}
}

func TestEverythingFinishedStartsOver(t *testing.T) {
	s := newTestSession(t)
	for _, l := range s.levels {
		s.finish(l, 2)
	}
	if next := s.nextLevel(); next != 0 {
		t.Errorf("with every level finished, the next level is %d, want 0", next)
	}
}

func TestSaveFailureIsKept(t *testing.T) {
	s := newTestSession(t)
	s.saveData = func(string, any) error { return errors.New("the disk is full") }
	s.finish(s.levels[0], 2)
	if s.saveErr == nil {
		t.Fatal("the failed save wasn't kept")
	}
	if !s.progress.finished(s.levels[0]) {
		t.Error("the progress was lost with the failed save")
	}
}
