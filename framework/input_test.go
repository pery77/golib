package golib

import (
	"math"
	"testing"
)

func onlySpace(key Key) bool { return key == KeySpace }

func noKey(Key) bool { return false }

func onlyLeftButton(button MouseButton) bool { return button == MouseLeft }

func noButton(MouseButton) bool { return false }

func noGamepad(int) gamepadFrame { return gamepadFrame{} }

func TestInputQueueDeliversEachPressOnce(t *testing.T) {
	var queue inputQueue
	var input Input

	// Space goes down during a frame that runs two updates.
	queue.readKeyboard(onlySpace, onlySpace)
	queue.next(&input)
	if !input.KeyPressed(KeySpace) || !input.KeyDown(KeySpace) {
		t.Fatal("first update: Space should be pressed and down")
	}
	queue.next(&input)
	if input.KeyPressed(KeySpace) {
		t.Error("second update of the same frame saw the press again")
	}
	if !input.KeyDown(KeySpace) {
		t.Error("second update: Space should still be down")
	}
}

func TestInputQueueKeepsPressesUntilAnUpdateRuns(t *testing.T) {
	var queue inputQueue
	var input Input

	// A quick tap: Space goes down and up within one frame, which runs no update.
	queue.readKeyboard(noKey, onlySpace)
	queue.readKeyboard(noKey, noKey)
	queue.next(&input)
	if !input.KeyPressed(KeySpace) {
		t.Error("the press was lost")
	}
	if input.KeyDown(KeySpace) {
		t.Error("a released key is reported down")
	}
}

func TestInputIgnoresUnknownKeys(t *testing.T) {
	var input Input
	if input.KeyDown(Key(-1)) || input.KeyPressed(Key(100000)) {
		t.Error("unknown keys must read as up")
	}
}

func TestInputQueueDeliversEachClickOnce(t *testing.T) {
	var queue inputQueue
	var input Input

	// The left button goes down at 100, 200 during a frame that runs two updates.
	queue.readMouse(100, 200, 0, onlyLeftButton, onlyLeftButton)
	queue.next(&input)
	if x, y := input.MousePosition(); x != 100 || y != 200 {
		t.Errorf("MousePosition() = %v, %v, want 100, 200", x, y)
	}
	if !input.MousePressed(MouseLeft) || !input.MouseDown(MouseLeft) {
		t.Fatal("first update: the left button should be pressed and down")
	}
	if input.MouseDown(MouseRight) {
		t.Error("the right button is reported down")
	}
	queue.next(&input)
	if input.MousePressed(MouseLeft) {
		t.Error("second update of the same frame saw the click again")
	}
	if !input.MouseDown(MouseLeft) {
		t.Error("second update: the left button should still be down")
	}
}

func TestInputQueueKeepsClicksAndTheLatestPosition(t *testing.T) {
	var queue inputQueue
	var input Input

	// A quick click during a frame that runs no update, then the pointer moves.
	queue.readMouse(10, 20, 0, noButton, onlyLeftButton)
	queue.readMouse(30, 40, 0, noButton, noButton)
	queue.next(&input)
	if !input.MousePressed(MouseLeft) {
		t.Error("the click was lost")
	}
	if x, y := input.MousePosition(); x != 30 || y != 40 {
		t.Errorf("MousePosition() = %v, %v, want the latest position, 30, 40", x, y)
	}
}

func TestInputQueueAddsUpTheWheelUntilAnUpdateRuns(t *testing.T) {
	var queue inputQueue
	var input Input

	// Two frames without an update turn the wheel up twice.
	queue.readMouse(0, 0, 1, noButton, noButton)
	queue.readMouse(0, 0, 1, noButton, noButton)
	queue.next(&input)
	if got := input.MouseWheel(); got != 2 {
		t.Errorf("MouseWheel() = %v, want 2", got)
	}
	queue.next(&input)
	if got := input.MouseWheel(); got != 0 {
		t.Errorf("second update: MouseWheel() = %v, want 0: each turn goes to one update", got)
	}
}

func TestInputIgnoresUnknownMouseButtons(t *testing.T) {
	var input Input
	if input.MouseDown(MouseButton(-1)) || input.MousePressed(MouseButton(99)) {
		t.Error("unknown mouse buttons must read as up")
	}
}

func TestInputQueueReadsGamepads(t *testing.T) {
	var queue inputQueue
	var input Input

	// Pad 1 is connected: A goes down and the left stick is pushed right, with
	// the right stick resting slightly off center.
	frame := func(pad int) gamepadFrame {
		if pad != 1 {
			return gamepadFrame{}
		}
		read := gamepadFrame{connected: true, name: "Test Pad", leftX: 1, rightX: 0.1, rightY: -0.1}
		read.down[GamepadA] = true
		read.pressed[GamepadA] = true
		return read
	}
	queue.readGamepads(frame)
	queue.next(&input)

	if input.GamepadConnected(0) || !input.GamepadConnected(1) {
		t.Errorf("connected: pad 0 = %v, pad 1 = %v; want false, true", input.GamepadConnected(0), input.GamepadConnected(1))
	}
	if got := input.GamepadName(1); got != "Test Pad" {
		t.Errorf("GamepadName(1) = %q, want %q", got, "Test Pad")
	}
	if !input.GamepadPressed(1, GamepadA) || !input.GamepadDown(1, GamepadA) || input.GamepadDown(1, GamepadB) {
		t.Error("pad 1: A should be pressed and down, and B up")
	}
	if x, y := input.GamepadLeftStick(1); x != 1 || y != 0 {
		t.Errorf("GamepadLeftStick(1) = %v, %v, want 1, 0", x, y)
	}
	if x, y := input.GamepadRightStick(1); x != 0 || y != 0 {
		t.Errorf("GamepadRightStick(1) = %v, %v, want 0, 0: inside the dead zone", x, y)
	}

	queue.readGamepads(frame)
	queue.next(&input)
	queue.next(&input)
	if input.GamepadPressed(1, GamepadA) {
		t.Error("an update without a new frame saw the press again")
	}
}

func TestInputIgnoresUnknownGamepads(t *testing.T) {
	var queue inputQueue
	var input Input
	queue.readGamepads(noGamepad)
	queue.next(&input)
	if input.GamepadConnected(-1) || input.GamepadConnected(maxGamepads) || input.GamepadDown(9, GamepadA) || input.GamepadPressed(0, GamepadButton(99)) {
		t.Error("unknown gamepads and buttons must read as disconnected and up")
	}
	if x, y := input.GamepadLeftStick(-1); x != 0 || y != 0 {
		t.Error("an unknown gamepad's stick must read 0, 0")
	}
}

func TestApplyDeadZone(t *testing.T) {
	tests := []struct {
		name         string
		x, y         float32
		wantX, wantY float32
	}{
		{name: "resting off center", x: 0.1, y: -0.1, wantX: 0, wantY: 0},
		{name: "full tilt stays full", x: 1, y: 0, wantX: 1, wantY: 0},
		{name: "rescaled from the edge of the dead zone", x: 0.6, y: 0, wantX: 0.5, wantY: 0},
		{name: "a corner is clamped to full tilt", x: 1, y: 1, wantX: float32(math.Sqrt(0.5)), wantY: float32(math.Sqrt(0.5))},
	}
	for _, tt := range tests {
		x, y := applyDeadZone(tt.x, tt.y)
		if math.Abs(float64(x-tt.wantX)) > 1e-4 || math.Abs(float64(y-tt.wantY)) > 1e-4 {
			t.Errorf("%s: applyDeadZone(%v, %v) = %v, %v, want %v, %v", tt.name, tt.x, tt.y, x, y, tt.wantX, tt.wantY)
		}
	}
}

func TestInputQueueReportsPointerMoves(t *testing.T) {
	var queue inputQueue
	var input Input
	steps := []struct {
		frames [][2]float32 // pointer positions read before the update
		moved  bool
	}{
		{[][2]float32{{50, 50}}, false},           // the first update has nothing to compare with
		{[][2]float32{{50, 50}}, false},           // resting
		{nil, false},                              // no frame between two updates
		{[][2]float32{{51, 50}}, true},            // moved
		{[][2]float32{{60, 60}, {51, 50}}, false}, // moved and came back within a frame's updates
		{[][2]float32{{51, 50}}, false},
	}
	for i, step := range steps {
		for _, p := range step.frames {
			queue.readMouse(p[0], p[1], 0, noButton, noButton)
		}
		queue.next(&input)
		if input.MouseMoved() != step.moved {
			t.Errorf("update %d: MouseMoved() = %v, want %v", i, input.MouseMoved(), step.moved)
		}
	}
}
