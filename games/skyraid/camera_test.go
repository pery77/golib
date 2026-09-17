package main

import "testing"

func TestCameraCentersTheShipAheadOfItsAim(t *testing.T) {
	w := quietWorld()
	w.ship.angle = 0 // aiming right
	c := newCamera(&w)
	sx, sy := c.toScreen(w.ship.x, w.ship.y)
	if !near(sx, screenWidth/2-cameraLead, 0.01) || !near(sy, screenHeight/2, 0.01) {
		t.Errorf("the ship is at %v, %v on the screen, want %v, %v", sx, sy, screenWidth/2-cameraLead, screenHeight/2)
	}
}

func TestCameraStaysInsideTheArena(t *testing.T) {
	w := quietWorld()
	w.ship.x, w.ship.y = 0, 0
	if c := newCamera(&w); c.x != 0 || c.y != 0 {
		t.Errorf("with the ship in the top-left corner, the camera is at %v, %v, want 0, 0", c.x, c.y)
	}
	w.ship.x, w.ship.y = worldWidth, worldHeight
	if c := newCamera(&w); c.x != worldWidth-screenWidth || c.y != worldHeight-screenHeight {
		t.Errorf("with the ship in the bottom-right corner, the camera is at %v, %v", c.x, c.y)
	}
}

func TestCameraCatchesUpWithTheShip(t *testing.T) {
	w := quietWorld()
	c := newCamera(&w)
	w.ship.x += 500
	for range 120 {
		c.follow(&w, dt)
	}
	goalX, goalY := cameraGoal(&w)
	if !near(c.x, goalX, 1) || !near(c.y, goalY, 1) {
		t.Errorf("two seconds after the ship moved, the camera is at %v, %v, want %v, %v", c.x, c.y, goalX, goalY)
	}
	if c.shakeX != 0 || c.shakeY != 0 {
		t.Errorf("the camera shakes by %v, %v with no trauma", c.shakeX, c.shakeY)
	}
	w.trauma = 1
	c.follow(&w, dt)
	if c.shakeX == 0 && c.shakeY == 0 {
		t.Error("the camera doesn't shake at full trauma")
	}
	if c.shakeX < -shakeMax || c.shakeX > shakeMax || c.shakeY < -shakeMax || c.shakeY > shakeMax {
		t.Errorf("the camera shakes by %v, %v, more than %v", c.shakeX, c.shakeY, shakeMax)
	}
}

func TestScreenAndArenaPointsAgree(t *testing.T) {
	c := camera{x: 812, y: 377, shakeX: 3, shakeY: -5}
	sx, sy := c.toScreen(1000, 600)
	if x, y := c.toWorld(sx, sy); !near(x, 1000, 0.001) || !near(y, 600, 0.001) {
		t.Errorf("a point went to the screen and back as %v, %v", x, y)
	}
	if !c.sees(1000, 600, 0) || c.sees(100, 100, 10) {
		t.Error("sees is wrong about a point on the screen, or one off it")
	}
}

func TestArrowsSitOnTheScreenEdge(t *testing.T) {
	c := camera{x: 1000, y: 1000}
	if _, _, ok := c.edgePoint(1500, 1300, 20); ok {
		t.Error("a point on the screen got an arrow")
	}
	tests := []struct {
		name   string
		x, y   float32
		wantX  float32
		wantY  float32
		within bool // only check that the arrow is on the frame
	}{
		{name: "right", x: 5000, y: 1000 + screenHeight/2, wantX: screenWidth - 20, wantY: screenHeight / 2},
		{name: "above", x: 1000 + screenWidth/2, y: 0, wantX: screenWidth / 2, wantY: 20},
		{name: "top-left", x: 0, y: 0, within: true},
	}
	for _, tt := range tests {
		x, y, ok := c.edgePoint(tt.x, tt.y, 20)
		if !ok {
			t.Errorf("%s: no arrow for a point off the screen", tt.name)
			continue
		}
		if tt.within {
			onFrame := near(x, 20, 0.01) || near(y, 20, 0.01)
			if !onFrame || x < 20 || y < 20 {
				t.Errorf("%s: arrow at %v, %v, not on the frame", tt.name, x, y)
			}
			continue
		}
		if !near(x, tt.wantX, 0.01) || !near(y, tt.wantY, 0.01) {
			t.Errorf("%s: arrow at %v, %v, want %v, %v", tt.name, x, y, tt.wantX, tt.wantY)
		}
	}
}
