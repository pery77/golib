package golib

import (
	"strings"
	"testing"
)

// switchGame is a scene that counts its updates and, in update number
// switchAt, calls SwitchScene with next.
type switchGame struct {
	next     Game
	switchAt int
	updates  int
}

func (g *switchGame) Update(input *Input, dt float32) {
	g.updates++
	if g.updates == g.switchAt {
		SwitchScene(g.next)
	}
}

func (g *switchGame) Draw(screen *Screen) {}

func forgetSceneRequest(t *testing.T) {
	takeSceneRequest()
	t.Cleanup(func() { takeSceneRequest() })
}

func TestRunUpdatesSwitchesScenes(t *testing.T) {
	forgetSceneRequest(t)
	play := &switchGame{}
	title := &switchGame{next: play, switchAt: 2}
	var input Input

	scene, quit, err := runUpdates(title, &input, noInput, 5)
	if err != nil || quit {
		t.Fatalf("runUpdates() quit = %v, error = %v; want neither", quit, err)
	}
	if scene != Game(play) {
		t.Error("runUpdates() didn't return the scene the game switched to")
	}
	if title.updates != 2 || play.updates != 3 {
		t.Errorf("title got %d updates and play %d, want 2 and 3: the switch happens right after the update that asks for it", title.updates, play.updates)
	}
}

func TestRunUpdatesRejectsNilScene(t *testing.T) {
	forgetSceneRequest(t)
	title := &switchGame{next: nil, switchAt: 1}
	var input Input

	_, _, err := runUpdates(title, &input, noInput, 1)
	if err == nil || !strings.Contains(err.Error(), "nil") {
		t.Errorf("runUpdates() error = %v, want one about the nil scene", err)
	}
}

func TestLastSwitchSceneWins(t *testing.T) {
	forgetSceneRequest(t)
	first, second := &switchGame{}, &switchGame{}
	SwitchScene(first)
	SwitchScene(second)
	if next, ok := takeSceneRequest(); !ok || next != Game(second) {
		t.Error("takeSceneRequest() didn't return the scene from the last SwitchScene call")
	}
	if _, ok := takeSceneRequest(); ok {
		t.Error("takeSceneRequest() returned a request twice")
	}
}
