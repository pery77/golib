package golib

import "sync"

// sceneRequest holds the scene passed to SwitchScene until Run switches to it.
var sceneRequest struct {
	sync.Mutex
	next      Game
	requested bool
}

// SwitchScene makes next the scene that Run updates and draws, from the update
// after the current one. A scene is any value that implements Game: split a
// game into scenes, such as a title screen, the game itself, a pause screen
// and a game over screen, pass the first one to Run, and switch from Update:
//
//	func (s *titleScene) Update(input *golib.Input, dt float32) {
//		if input.KeyPressed(golib.KeyEnter) {
//			golib.SwitchScene(newPlayScene())
//		}
//	}
//
// Run stops calling the old scene, but the scene keeps its state. A pause
// screen can hold on to the scene it paused, draw it under its own text and
// switch back to it:
//
//	type pauseScene struct {
//		paused golib.Game
//	}
//
//	func (s *pauseScene) Update(input *golib.Input, dt float32) {
//		if input.KeyPressed(golib.KeyEscape) {
//			golib.SwitchScene(s.paused) // carries on where it stopped
//		}
//	}
//
//	func (s *pauseScene) Draw(screen *golib.Screen) {
//		s.paused.Draw(screen)
//		screen.DrawText("Paused", 40, 40, 40, golib.White)
//	}
//
// Call SwitchScene from Update. When one update calls it more than once, the
// last call wins. Run returns an error if next is nil.
func SwitchScene(next Game) {
	sceneRequest.Lock()
	defer sceneRequest.Unlock()
	sceneRequest.next = next
	sceneRequest.requested = true
}

// takeSceneRequest returns the scene passed to SwitchScene since the last
// call, and whether there was one.
func takeSceneRequest() (Game, bool) {
	sceneRequest.Lock()
	defer sceneRequest.Unlock()
	next, requested := sceneRequest.next, sceneRequest.requested
	sceneRequest.next, sceneRequest.requested = nil, false
	return next, requested
}
