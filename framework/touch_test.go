package golib

import "testing"

// oneFinger is a finger on the screen at x, y, with the id and the landing a
// backend would report.
func oneFinger(id int, x, y float32, pressed bool) []Touch {
	return []Touch{{ID: id, Position: Vector2{X: x, Y: y}, Pressed: pressed}}
}

func TestInputQueueDeliversEachTouchOnce(t *testing.T) {
	var queue inputQueue
	var input Input

	// A finger lands at 200, 600 during a frame that runs two updates.
	queue.readTouches(oneFinger(1, 200, 600, true))
	queue.next(&input)
	touches := input.Touches()
	if len(touches) != 1 {
		t.Fatalf("first update sees %d fingers, want 1", len(touches))
	}
	if !touches[0].Pressed {
		t.Error("first update: the finger should have landed")
	}
	if touches[0].Position != (Vector2{X: 200, Y: 600}) {
		t.Errorf("the finger is at %+v, want 200, 600", touches[0].Position)
	}

	queue.next(&input)
	touches = input.Touches()
	if len(touches) != 1 {
		t.Fatalf("second update of the same frame sees %d fingers, want the finger still down", len(touches))
	}
	if touches[0].Pressed {
		t.Error("second update of the same frame saw the finger land again")
	}
}

func TestInputQueueKeepsAQuickTapUntilAnUpdateRuns(t *testing.T) {
	var queue inputQueue
	var input Input

	// A tap: the finger lands and lifts within one frame, and that frame runs
	// no update, as one does on a display faster than 60 Hz.
	queue.readTouches(oneFinger(7, 320, 180, true))
	queue.readTouches(nil)
	queue.next(&input)
	touches := input.Touches()
	if len(touches) != 1 {
		t.Fatalf("the update sees %d fingers, want the tap that happened between updates", len(touches))
	}
	if !touches[0].Pressed || touches[0].ID != 7 {
		t.Errorf("the tap reached the update as %+v, want finger 7 landing", touches[0])
	}
	if touches[0].Position != (Vector2{X: 320, Y: 180}) {
		t.Errorf("the tap is at %+v, want where the finger was, 320, 180", touches[0].Position)
	}

	// And it is gone from the update after it.
	queue.next(&input)
	if len(input.Touches()) != 0 {
		t.Errorf("the next update still sees %d fingers, want none", len(input.Touches()))
	}
}

func TestInputQueueReadsSeveralFingersOldestFirst(t *testing.T) {
	var queue inputQueue
	var input Input

	queue.readTouches([]Touch{
		{ID: 1, Position: Vector2{X: 100, Y: 600}, Pressed: true},
		{ID: 2, Position: Vector2{X: 1100, Y: 600}, Pressed: true},
	})
	queue.next(&input)
	touches := input.Touches()
	if len(touches) != 2 || touches[0].ID != 1 || touches[1].ID != 2 {
		t.Fatalf("the update sees %+v, want fingers 1 and 2 in that order", touches)
	}

	// The first finger lifts; the second keeps its own id and place.
	queue.readTouches([]Touch{{ID: 2, Position: Vector2{X: 1100, Y: 500}}})
	queue.next(&input)
	touches = input.Touches()
	if len(touches) != 1 || touches[0].ID != 2 {
		t.Fatalf("after the first finger lifted the update sees %+v, want finger 2 alone", touches)
	}
	if touches[0].Pressed {
		t.Error("finger 2 landed again, want it only held")
	}
	if touches[0].Position.Y != 500 {
		t.Errorf("finger 2 is at y %v, want it where it moved, 500", touches[0].Position.Y)
	}
}

func TestInputQueueReadsNoMoreFingersThanItHolds(t *testing.T) {
	var queue inputQueue
	var input Input
	var many []Touch
	for i := range maxTouches + 3 {
		many = append(many, Touch{ID: i + 1, Position: Vector2{X: float32(i)}, Pressed: true})
	}
	queue.readTouches(many)
	queue.next(&input)
	if len(input.Touches()) != maxTouches {
		t.Errorf("the update sees %d fingers, want the %d GoLib holds", len(input.Touches()), maxTouches)
	}
}

// A finger works a game written for a mouse: it moves the pointer and holds the
// left button, so menus and buttons need no touch code of their own.
func TestAFingerHoldsTheLeftMouseButton(t *testing.T) {
	touches := oneFinger(1, 200, 600, true)
	down, pressed := mouseWithTouch(touches, noButton, noButton)
	if !down(MouseLeft) || !pressed(MouseLeft) {
		t.Error("a finger landing should hold and click the left mouse button")
	}
	if down(MouseRight) || pressed(MouseRight) {
		t.Error("a finger should touch no button but the left one")
	}

	// A finger still down, which landed before, holds the button without
	// clicking it again.
	down, pressed = mouseWithTouch(oneFinger(1, 200, 600, false), noButton, noButton)
	if !down(MouseLeft) {
		t.Error("a finger held down should hold the left mouse button")
	}
	if pressed(MouseLeft) {
		t.Error("a finger held down should not click again")
	}

	// With no finger, the mouse is the mouse.
	down, pressed = mouseWithTouch(nil, onlyLeftButton, onlyLeftButton)
	if !down(MouseLeft) || !pressed(MouseLeft) {
		t.Error("the mouse's own buttons should still be read")
	}
}

func TestTouchesInAnArea(t *testing.T) {
	var queue inputQueue
	var input Input
	button := Rectangle{X: 100, Y: 600, Width: 120, Height: 120}
	elsewhere := Rectangle{X: 1000, Y: 600, Width: 120, Height: 120}

	queue.readTouches(oneFinger(1, 150, 650, true))
	queue.next(&input)
	if !input.TouchDownIn(button) || !input.TouchPressedIn(button) {
		t.Error("the finger landed on the button, so it is down and pressed there")
	}
	if input.TouchDownIn(elsewhere) || input.TouchPressedIn(elsewhere) {
		t.Error("a button the finger never touched reads as pressed")
	}

	// The next update, the finger is still on it but no longer landing.
	queue.readTouches(oneFinger(1, 150, 650, false))
	queue.next(&input)
	if !input.TouchDownIn(button) {
		t.Error("the finger is still on the button")
	}
	if input.TouchPressedIn(button) {
		t.Error("the finger landed on the button twice")
	}
}

// Without a touch screen a game reads no fingers, so it can ask in every
// update without checking first.
func TestNoTouchScreenMeansNoFingers(t *testing.T) {
	var input Input
	if touches := input.Touches(); len(touches) != 0 {
		t.Errorf("an update with no touch screen sees %+v, want no fingers", touches)
	}
	if input.TouchDownIn(Rectangle{Width: 100, Height: 100}) || input.TouchPressedIn(Rectangle{Width: 100, Height: 100}) {
		t.Error("an area is touched with no touch screen")
	}
	if PlayingWithTouch() {
		t.Error("PlayingWithTouch() is true in a test, where there is no touch screen at all")
	}
}

// One build is played both ways: the controls follow what the player last
// used, so a computer with a touch screen shows no pads until a finger lands
// on it, and a phone with a keyboard plugged in stops showing them.
func TestPlayingWithTouchFollowsThePlayer(t *testing.T) {
	t.Cleanup(func() { playingWithTouch.Store(false) })
	tests := []struct {
		name                              string
		fingers, keyboard, mouse, gamepad bool
		want                              bool
	}{
		{name: "a finger lands", fingers: true, want: true},
		{name: "nothing is used, so the answer holds", want: true},
		{name: "a finger, with the mouse moving under it", fingers: true, mouse: true, want: true},
		{name: "the keyboard takes over", keyboard: true, want: false},
		{name: "nothing is used, so that holds too", want: false},
		{name: "the mouse", mouse: true, want: false},
		{name: "a gamepad", gamepad: true, want: false},
		{name: "a finger again", fingers: true, want: true},
	}
	playingWithTouch.Store(false)
	for _, tt := range tests {
		followTouchPlaying(tt.fingers, tt.keyboard, tt.mouse, tt.gamepad)
		if got := PlayingWithTouch(); got != tt.want {
			t.Errorf("%s: PlayingWithTouch() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
