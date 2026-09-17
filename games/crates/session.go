package main

import (
	"fmt"
	"log"

	"golib"
)

// session is what every scene shares: the levels and the player's progress,
// which says whether the music plays. main makes one, and each scene keeps a
// pointer to it.
type session struct {
	levels   []*layout
	progress progress
	saveData func(name string, value any) error // golib.SaveData; tests replace it
	saveErr  error                              // the last failure to load or save the progress, shown on the title
	gamepad  bool                               // gamepad 0 is connected, so the scenes show its controls
}

// newSession loads the saved progress. Progress that can't be read is a
// message on the title, not the end of the game.
func newSession(levels []*layout) *session {
	var p progress
	_, err := golib.LoadData(progressName, &p)
	if err != nil {
		p = progress{} // damaged: start over
		err = fmt.Errorf("the saved progress is damaged, so the game starts over: %w", err)
		log.Print(err)
	}
	return &session{levels: levels, progress: p, saveData: golib.SaveData, saveErr: err}
}

// update reads the keys that work in every scene, fullscreen and music, and
// keeps the tune playing. Every scene calls it first thing in its Update.
func (s *session) update(input *golib.Input) {
	s.gamepad = input.GamepadConnected(0)
	if input.KeyPressed(golib.KeyF11) || (altDown(input) && input.KeyPressed(golib.KeyEnter)) {
		golib.SetFullscreen(!golib.IsFullscreen())
	}
	if input.KeyPressed(golib.KeyM) || input.GamepadPressed(0, golib.GamepadBack) {
		s.toggleMusic()
	}
	if s.musicOn() {
		theme.Play() // safe every update: the tune keeps playing
	} else {
		theme.Pause()
	}
}

func (s *session) musicOn() bool {
	return !s.progress.MusicOff
}

// toggleMusic turns the music on or off, and remembers it.
func (s *session) toggleMusic() {
	s.progress.MusicOff = !s.progress.MusicOff
	s.save()
}

// save writes the progress. A failure goes to the console and is kept for the
// title to show, and the game goes on.
func (s *session) save() {
	err := s.saveData(progressName, s.progress)
	if err != nil && (s.saveErr == nil || err.Error() != s.saveErr.Error()) {
		log.Print(err)
	}
	s.saveErr = err
}

// finish records that level l was finished in moves, and saves. It reports
// whether that is the level's best.
func (s *session) finish(l *layout, moves int) bool {
	best := s.progress.record(l, moves)
	s.save()
	return best
}

// unlocked reports whether level i, counted from 0, can be played: the first
// level, every finished level, and the level after a finished one.
func (s *session) unlocked(i int) bool {
	return i == 0 || s.progress.finished(s.levels[i]) || s.progress.finished(s.levels[i-1])
}

// nextLevel returns the level to continue from: the first one not finished
// yet that can be played, or the first level when every one is finished.
func (s *session) nextLevel() int {
	for i, l := range s.levels {
		if !s.progress.finished(l) && s.unlocked(i) {
			return i
		}
	}
	return 0
}

// finishedLevels returns how many levels have been finished, and how many of
// those in par moves.
func (s *session) finishedLevels() (finished, atPar int) {
	for _, l := range s.levels {
		if best, ok := s.progress.Best[l.name]; ok {
			finished++
			if best <= l.par {
				atPar++
			}
		}
	}
	return finished, atPar
}

// altDown reports whether an Alt key is held, for Alt+Enter.
func altDown(input *golib.Input) bool {
	return input.KeyDown(golib.KeyLeftAlt) || input.KeyDown(golib.KeyRightAlt)
}
