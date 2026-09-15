package golib

import "testing"

// quitGame is a game that calls Quit in update number quitAt, or never if
// quitAt is 0.
type quitGame struct {
	quitAt  int
	updates int
}

func (g *quitGame) Update(input *Input, dt float32) {
	g.updates++
	if g.updates == g.quitAt {
		Quit()
	}
}

func (g *quitGame) Draw(screen *Screen) {}

// noInput leaves every key up.
func noInput(*Input) {}

func TestRunUpdatesStopsAtQuit(t *testing.T) {
	tests := []struct {
		name        string
		quitAt      int
		updates     int
		wantQuit    bool
		wantUpdates int
	}{
		{name: "no quit runs every update", quitAt: 0, updates: 3, wantQuit: false, wantUpdates: 3},
		{name: "quit stops the remaining updates", quitAt: 2, updates: 5, wantQuit: true, wantUpdates: 2},
		{name: "quit in the last update", quitAt: 3, updates: 3, wantQuit: true, wantUpdates: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quitRequested.Store(false)
			t.Cleanup(func() { quitRequested.Store(false) })
			game := &quitGame{quitAt: tt.quitAt}
			var input Input

			_, quit, err := runUpdates(game, &input, noInput, tt.updates)
			if err != nil {
				t.Fatalf("runUpdates() error = %v", err)
			}
			if quit != tt.wantQuit {
				t.Errorf("runUpdates() quit = %v, want %v", quit, tt.wantQuit)
			}
			if game.updates != tt.wantUpdates {
				t.Errorf("game got %d updates, want %d", game.updates, tt.wantUpdates)
			}
		})
	}
}
