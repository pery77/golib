package golib

import (
	"errors"
	"testing"
)

// mistakeGame reports a mistake in its second update.
type mistakeGame struct {
	updates int
}

func (g *mistakeGame) Update(input *Input, dt float32) {
	g.updates++
	if g.updates == 2 {
		reportError(errors.New("a sprite is missing"))
	}
}

func (g *mistakeGame) Draw(screen *Screen) {}

func TestRunUpdatesStopsAtAMistake(t *testing.T) {
	takeError()
	game := &mistakeGame{}
	var input Input
	_, _, err := runUpdates(game, &input, noInput, 5)
	if err == nil || err.Error() != "a sprite is missing" || game.updates != 2 {
		t.Errorf("runUpdates() = %v after %d updates, want the mistake after 2", err, game.updates)
	}
	if err := takeError(); err != nil {
		t.Errorf("the mistake is still waiting: %v", err)
	}
}

func TestReportErrorKeepsTheFirst(t *testing.T) {
	takeError()
	reportError(errors.New("first"))
	reportError(errors.New("second"))
	if err := takeError(); err == nil || err.Error() != "first" {
		t.Errorf("takeError() = %v, want the first mistake", err)
	}
	if err := takeError(); err != nil {
		t.Errorf("takeError() = %v after taking it, want nil", err)
	}
}

func TestRunStopsAtAMistakeMadeBefore(t *testing.T) {
	takeError()
	// Such as a package variable asking a sprite for an animation it lacks.
	reportError(errors.New("no animation named hop"))
	if err := Run(&mistakeGame{}, Config{}); err == nil || err.Error() != "no animation named hop" {
		t.Errorf("Run() = %v, want the mistake made before it, without opening a window", err)
	}
}
