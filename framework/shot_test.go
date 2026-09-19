package golib

import (
	"reflect"
	"strings"
	"testing"
)

func TestShotPlanFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    *shotPlan
		wantErr bool
	}{
		{
			name: "normal run",
			env:  map[string]string{},
			want: nil,
		},
		{
			name: "frames are sorted and deduplicated",
			env:  map[string]string{"GOLIB_SHOT_DIR": "shots", "GOLIB_SHOT_FRAMES": "60, 1,60"},
			want: &shotPlan{dir: "shots", frames: []int{1, 60}, scale: 1},
		},
		{
			name: "pictures enlarged",
			env:  map[string]string{"GOLIB_SHOT_DIR": "shots", "GOLIB_SHOT_FRAMES": "60", "GOLIB_SHOT_SCALE": "3"},
			want: &shotPlan{dir: "shots", frames: []int{60}, scale: 3},
		},
		{
			name:    "a scale on its own",
			env:     map[string]string{"GOLIB_SHOT_SCALE": "3"},
			wantErr: true,
		},
		{
			name:    "data to start from on its own",
			env:     map[string]string{"GOLIB_SHOT_SAVE": "state.json"},
			wantErr: true,
		},
		{
			name:    "scale zero",
			env:     map[string]string{"GOLIB_SHOT_DIR": "shots", "GOLIB_SHOT_FRAMES": "60", "GOLIB_SHOT_SCALE": "0"},
			wantErr: true,
		},
		{
			name:    "scale too big",
			env:     map[string]string{"GOLIB_SHOT_DIR": "shots", "GOLIB_SHOT_FRAMES": "60", "GOLIB_SHOT_SCALE": "9"},
			wantErr: true,
		},
		{
			name:    "scale not a number",
			env:     map[string]string{"GOLIB_SHOT_DIR": "shots", "GOLIB_SHOT_FRAMES": "60", "GOLIB_SHOT_SCALE": "three"},
			wantErr: true,
		},
		{
			name:    "folder without frames",
			env:     map[string]string{"GOLIB_SHOT_DIR": "shots"},
			wantErr: true,
		},
		{
			name:    "frames without folder",
			env:     map[string]string{"GOLIB_SHOT_FRAMES": "60"},
			wantErr: true,
		},
		{
			name:    "frame zero",
			env:     map[string]string{"GOLIB_SHOT_DIR": "shots", "GOLIB_SHOT_FRAMES": "0"},
			wantErr: true,
		},
		{
			name:    "not a number",
			env:     map[string]string{"GOLIB_SHOT_DIR": "shots", "GOLIB_SHOT_FRAMES": "sixty"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := shotPlanFromEnv(func(key string) string { return tt.env[key] })
			if tt.wantErr {
				if err == nil {
					t.Fatalf("shotPlanFromEnv() = %+v, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("shotPlanFromEnv() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("shotPlanFromEnv() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestShotFileName(t *testing.T) {
	if got := shotFileName(60); got != "frame-000060.png" {
		t.Errorf("shotFileName(60) = %q, want %q", got, "frame-000060.png")
	}
}

func TestShotPlanFromEnvReadsInput(t *testing.T) {
	getenv := func(env map[string]string) func(string) string {
		return func(key string) string { return env[key] }
	}

	plan, err := shotPlanFromEnv(getenv(map[string]string{"GOLIB_SHOT_DIR": "shots", "GOLIB_SHOT_FRAMES": "60", "GOLIB_SHOT_INPUT": "Enter@1"}))
	if err != nil {
		t.Fatalf("shotPlanFromEnv() error = %v", err)
	}
	if want := (inputScript{keys: []hold[Key]{{of: KeyEnter, first: 1, last: 1}}}); !reflect.DeepEqual(plan.input, want) {
		t.Errorf("plan.input = %+v, want %+v", plan.input, want)
	}

	if _, err := shotPlanFromEnv(getenv(map[string]string{"GOLIB_SHOT_INPUT": "Enter@1"})); err == nil {
		t.Error("input without a folder and frames: want an error")
	}
	if _, err := shotPlanFromEnv(getenv(map[string]string{"GOLIB_SHOT_DIR": "shots", "GOLIB_SHOT_FRAMES": "60", "GOLIB_SHOT_INPUT": "Jump@1"})); err == nil {
		t.Error("an unknown key: want an error")
	}
}

func TestParseInputScript(t *testing.T) {
	tests := []struct {
		name    string
		script  string
		want    inputScript
		wantErr string // part of the error message; empty when no error is expected
	}{
		{name: "empty script", script: "", want: inputScript{}},
		{
			name:   "keys, any letter case",
			script: "enter@1 RIGHT@30-90 a@2 Zero@5",
			want: inputScript{keys: []hold[Key]{
				{of: KeyEnter, first: 1, last: 1}, {of: KeyRight, first: 30, last: 90},
				{of: KeyA, first: 2, last: 2}, {of: KeyZero, first: 5, last: 5},
			}},
		},
		{
			name:   "mouse buttons and moves, moves sorted by update",
			script: "Mouse@50:10,20 MouseLeft@51 mouseright@60-70 Mouse@5:640.5,360",
			want: inputScript{
				buttons: []hold[MouseButton]{{of: MouseLeft, first: 51, last: 51}, {of: MouseRight, first: 60, last: 70}},
				moves:   []mouseMove{{update: 5, x: 640.5, y: 360}, {update: 50, x: 10, y: 20}},
			},
		},
		{name: "unknown name", script: "Jump@1", wantErr: `named "Jump"`},
		{name: "no update", script: "Enter", wantErr: "Name@update"},
		{name: "update zero", script: "Enter@0", wantErr: "count from 1"},
		{name: "backwards range", script: "Right@90-30", wantErr: "count from 1"},
		{name: "not a number", script: "Right@soon", wantErr: "count from 1"},
		{name: "mouse move without a position", script: "Mouse@10", wantErr: "Mouse@update:x,y"},
		{name: "mouse move with one coordinate", script: "Mouse@10:640", wantErr: "Mouse@update:x,y"},
		{
			name:   "gamepad buttons, sticks sorted by update, and the wheel",
			script: "GamepadA@5 gamepadstart@6-8 GamepadLeftStick@10:1,0 GamepadRightStick@4:0,-0.5 MouseWheel@3:-1",
			want: inputScript{
				gamepadButtons: []hold[GamepadButton]{{of: GamepadA, first: 5, last: 5}, {of: GamepadStart, first: 6, last: 8}},
				sticks:         []stickMove{{right: true, update: 4, x: 0, y: -0.5}, {right: false, update: 10, x: 1, y: 0}},
				wheel:          []wheelTurn{{update: 3, amount: -1}},
			},
		},
		{
			name:   "fingers, one update and a range",
			script: "Touch@40:200,600 touch@50-90:1100,600.5",
			want: inputScript{touches: []touchHold{
				{first: 40, last: 40, x: 200, y: 600},
				{first: 50, last: 90, x: 1100, y: 600.5},
			}},
		},
		{name: "finger without a position", script: "Touch@40", wantErr: "Touch@update:x,y"},
		{name: "finger with one coordinate", script: "Touch@40:200", wantErr: "Touch@update:x,y"},
		{name: "finger in update zero", script: "Touch@0:200,600", wantErr: "Touch@update:x,y"},
		{name: "stick tilted too far", script: "GamepadLeftStick@10:2,0", wantErr: "from -1 to 1"},
		{name: "wheel without an amount", script: "MouseWheel@3", wantErr: "MouseWheel@update:notches"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseInputScript(tt.script)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseInputScript(%q) error = %v, want one containing %q", tt.script, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseInputScript(%q) error = %v", tt.script, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseInputScript(%q) = %+v, want %+v", tt.script, got, tt.want)
			}
		})
	}
}

func TestInputScriptKeys(t *testing.T) {
	// Enter is pressed in update 2. Right is held from update 3 to 7, written
	// as two holds that touch.
	script := inputScript{keys: []hold[Key]{
		{of: KeyEnter, first: 2, last: 2}, {of: KeyRight, first: 3, last: 5}, {of: KeyRight, first: 6, last: 7},
	}}
	tests := []struct {
		update                  int
		enterDown, enterPressed bool
		rightDown, rightPressed bool
	}{
		{update: 1},
		{update: 2, enterDown: true, enterPressed: true},
		{update: 3, rightDown: true, rightPressed: true},
		{update: 4, rightDown: true},
		{update: 6, rightDown: true}, // the second hold continues the first: no new press
		{update: 8},
	}
	for _, tt := range tests {
		in := script.at(tt.update)
		got := [4]bool{in.KeyDown(KeyEnter), in.KeyPressed(KeyEnter), in.KeyDown(KeyRight), in.KeyPressed(KeyRight)}
		want := [4]bool{tt.enterDown, tt.enterPressed, tt.rightDown, tt.rightPressed}
		if got != want {
			t.Errorf("update %d: Enter down, Enter pressed, Right down, Right pressed = %v, want %v", tt.update, got, want)
		}
	}
}

func TestInputScriptMouse(t *testing.T) {
	script, err := parseInputScript("Mouse@10:100,200 MouseLeft@12 Mouse@20:300,400")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		update        int
		x, y          float32
		down, pressed bool
	}{
		{update: 1, x: 0, y: 0},
		{update: 10, x: 100, y: 200},
		{update: 12, x: 100, y: 200, down: true, pressed: true},
		{update: 13, x: 100, y: 200},
		{update: 25, x: 300, y: 400},
	}
	for _, tt := range tests {
		in := script.at(tt.update)
		x, y := in.MousePosition()
		if x != tt.x || y != tt.y || in.MouseDown(MouseLeft) != tt.down || in.MousePressed(MouseLeft) != tt.pressed {
			t.Errorf("update %d: pointer %v, %v, left down %v, pressed %v; want %v, %v, %v, %v",
				tt.update, x, y, in.MouseDown(MouseLeft), in.MousePressed(MouseLeft), tt.x, tt.y, tt.down, tt.pressed)
		}
	}

	// The pointer moves in the updates where a move changes where it is,
	// but not in the first update.
	script, err = parseInputScript("Mouse@1:5,5 Mouse@3:6,5 Mouse@4:6,5")
	if err != nil {
		t.Fatal(err)
	}
	for update, want := range []bool{1: false, 2: false, 3: true, 4: false, 5: false} {
		in := script.at(update)
		if update > 0 && in.MouseMoved() != want {
			t.Errorf("update %d: MouseMoved() = %v, want %v", update, !want, want)
		}
	}
}

// A script puts fingers on the screen, so that shots can check a game's
// on-screen controls: each Touch item is a finger of its own, two that overlap
// are two fingers at once, and a finger works the mouse as it does in a window.
func TestInputScriptTouches(t *testing.T) {
	script, err := parseInputScript("Touch@10-30:200,600 Touch@20:1100,650")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		update  int
		fingers int
		pressed int // how many of them landed in this update
		x, y    float32
	}{
		{update: 1, fingers: 0},
		{update: 10, fingers: 1, pressed: 1, x: 200, y: 600},
		{update: 11, fingers: 1, x: 200, y: 600},
		{update: 20, fingers: 2, pressed: 1, x: 200, y: 600},
		{update: 31, fingers: 0},
	}
	for _, tt := range tests {
		in := script.at(tt.update)
		touches := in.Touches()
		pressed := 0
		for _, touch := range touches {
			if touch.Pressed {
				pressed++
			}
		}
		if len(touches) != tt.fingers || pressed != tt.pressed {
			t.Errorf("update %d: %d fingers, %d of them landing; want %d and %d",
				tt.update, len(touches), pressed, tt.fingers, tt.pressed)
		}
		if tt.fingers == 0 {
			continue
		}
		// The oldest finger moves the pointer and holds the left button.
		x, y := in.MousePosition()
		if x != tt.x || y != tt.y {
			t.Errorf("update %d: the pointer is at %v, %v, want the oldest finger at %v, %v", tt.update, x, y, tt.x, tt.y)
		}
		if !in.MouseDown(MouseLeft) {
			t.Errorf("update %d: a finger on the screen doesn't hold the left mouse button", tt.update)
		}
		if in.MousePressed(MouseLeft) != (tt.pressed > 0) {
			t.Errorf("update %d: the left button clicked = %v, want %v", tt.update, in.MousePressed(MouseLeft), tt.pressed > 0)
		}
	}

	// Fingers have ids of their own, in the script's order, so a game can
	// follow one of them.
	in := script.at(20)
	if touches := in.Touches(); touches[0].ID == touches[1].ID {
		t.Errorf("both fingers have id %d, want one each", touches[0].ID)
	}
}

func TestInputScriptGamepadAndWheel(t *testing.T) {
	if in := (inputScript{}).at(1); in.GamepadConnected(0) {
		t.Error("a script without gamepad items connected gamepad 0")
	}

	script, err := parseInputScript("GamepadA@5 GamepadLeftStick@10:1,0 GamepadLeftStick@20:0,0 MouseWheel@3:-1")
	if err != nil {
		t.Fatal(err)
	}
	first := script.at(1)
	if !first.GamepadConnected(0) || first.GamepadName(0) != scriptedGamepadName || first.GamepadConnected(1) {
		t.Errorf("gamepad 0 connected = %v, named %q; gamepad 1 connected = %v; want true, %q, false",
			first.GamepadConnected(0), first.GamepadName(0), first.GamepadConnected(1), scriptedGamepadName)
	}
	if in := script.at(5); !in.GamepadPressed(0, GamepadA) || !in.GamepadDown(0, GamepadA) {
		t.Error("update 5: A should be pressed and down")
	}
	if in := script.at(6); in.GamepadDown(0, GamepadA) {
		t.Error("update 6: A should be up again")
	}
	tilted, centered := script.at(15), script.at(20)
	if x, y := tilted.GamepadLeftStick(0); x != 1 || y != 0 {
		t.Errorf("update 15: left stick = %v, %v, want 1, 0", x, y)
	}
	if x, y := centered.GamepadLeftStick(0); x != 0 || y != 0 {
		t.Errorf("update 20: left stick = %v, %v, want 0, 0", x, y)
	}
	turned, still := script.at(3), script.at(4)
	if got := turned.MouseWheel(); got != -1 {
		t.Errorf("update 3: MouseWheel() = %v, want -1", got)
	}
	if got := still.MouseWheel(); got != 0 {
		t.Errorf("update 4: MouseWheel() = %v, want 0", got)
	}
}
