package main

import (
	"testing"

	"golib"
)

// The camera is golib.Camera; these tests check what the game asks of it: the
// lead ahead of the aim, the arena's edges, the catch-up and the shake. Its
// Update, Snap, View, ToScreen and ToWorld all work without a window.

func TestCameraCentersTheShipAheadOfItsAim(t *testing.T) {
	w := quietWorld()
	w.ship.angle = 0 // aiming right
	on := newArenaCamera(&w).ToScreen(w.ship.position)
	// The view sits on whole pixels, so half a pixel either way is right.
	if !near(on.X, screenWidth/2-cameraLead, 0.5) || !near(on.Y, screenHeight/2, 0.5) {
		t.Errorf("the ship is at %v, %v on the screen, want %v, %v", on.X, on.Y, screenWidth/2-cameraLead, screenHeight/2)
	}
}

func TestCameraStaysInsideTheArena(t *testing.T) {
	w := quietWorld()
	w.ship.position, w.ship.angle = golib.Vector2{}, -90
	if view := newArenaCamera(&w).View(); view.X != 0 || view.Y != 0 {
		t.Errorf("with the ship in the top-left corner, the view starts at %v, %v, want 0, 0", view.X, view.Y)
	}
	w.ship.position, w.ship.angle = golib.Vector2{X: worldWidth, Y: worldHeight}, 90
	view := newArenaCamera(&w).View()
	if view.X != worldWidth-screenWidth || view.Y != worldHeight-screenHeight {
		t.Errorf("with the ship in the bottom-right corner, the view starts at %v, %v", view.X, view.Y)
	}
}

func TestCameraCatchesUpWithTheShip(t *testing.T) {
	p := &playScene{world: quietWorld()}
	p.camera = newArenaCamera(&p.world)
	p.world.ship.position.X += 500
	for range 120 {
		p.followCamera(dt)
	}
	want := cameraTarget(&p.world)
	if got := p.camera.Center(); !near(got.X, want.X, 1) || !near(got.Y, want.Y, 1) {
		t.Errorf("two seconds after the ship moved, the view is on %v, %v, want %v, %v", got.X, got.Y, want.X, want.Y)
	}
}

func TestCameraShakesWithTrauma(t *testing.T) {
	p := &playScene{world: quietWorld()}
	p.camera = newArenaCamera(&p.world)
	still := p.camera.Center()
	p.followCamera(dt)
	if p.camera.Center() != still {
		t.Errorf("with no trauma the view moved from %v to %v", still, p.camera.Center())
	}

	p.world.trauma = 1 // the world only loses trauma in step, which this test skips
	shaken := false
	for range 20 {
		p.followCamera(dt)
		off := p.camera.Center().Sub(still)
		if off != (golib.Vector2{}) {
			shaken = true
		}
		// One pixel of slack: the view sits on whole pixels.
		if abs(off.X) > shakeMax+1 || abs(off.Y) > shakeMax+1 {
			t.Fatalf("the view shook by %v, %v, more than %v", off.X, off.Y, shakeMax)
		}
	}
	if !shaken {
		t.Error("the view doesn't shake at full trauma")
	}
}

func TestArrowsSitOnTheScreenEdge(t *testing.T) {
	w := quietWorld()
	c := newArenaCamera(&w)
	if !c.View().Contains(w.ship.position.X, w.ship.position.Y) {
		t.Error("the ship isn't in the camera's view, so it would get an arrow")
	}
	const inset = 20
	tests := []struct {
		name   string
		on     golib.Vector2 // the enemy's place on the screen, outside it
		want   golib.Vector2
		within bool // only check that the arrow is on the frame
	}{
		{name: "right", on: golib.Vector2{X: 4000, Y: screenHeight / 2}, want: golib.Vector2{X: screenWidth - inset, Y: screenHeight / 2}},
		{name: "above", on: golib.Vector2{X: screenWidth / 2, Y: -1000}, want: golib.Vector2{X: screenWidth / 2, Y: inset}},
		{name: "top-left", on: golib.Vector2{X: -1000, Y: -1000}, within: true},
	}
	for _, tt := range tests {
		edge := edgePoint(tt.on, inset)
		if tt.within {
			onFrame := near(edge.X, inset, 0.01) || near(edge.Y, inset, 0.01)
			if !onFrame || edge.X < inset || edge.Y < inset {
				t.Errorf("%s: arrow at %v, %v, not on the frame", tt.name, edge.X, edge.Y)
			}
			continue
		}
		if !near(edge.X, tt.want.X, 0.01) || !near(edge.Y, tt.want.Y, 0.01) {
			t.Errorf("%s: arrow at %v, %v, want %v, %v", tt.name, edge.X, edge.Y, tt.want.X, tt.want.Y)
		}
	}
}
