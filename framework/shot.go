package golib

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// golib shot starts a game with these environment variables set. See
// docs/tooling.md.
const (
	shotDirEnv    = "GOLIB_SHOT_DIR"    // folder to save screenshots in
	shotFramesEnv = "GOLIB_SHOT_FRAMES" // frames to capture, separated by commas, such as "1,60,300"
	shotInputEnv  = "GOLIB_SHOT_INPUT"  // input to play, such as "Enter@1 Right@30-90"; may be empty
)

// scriptedGamepadName is the name of gamepad 0 when an input script uses it.
const scriptedGamepadName = "golib shot"

// shotPlan says which frames to capture, where to save them and which input
// to play.
type shotPlan struct {
	dir    string
	frames []int // ascending, no duplicates, each at least 1
	input  inputScript
}

// shotPlanFromEnv reads the screenshot plan from the environment. It returns
// nil and no error for a normal run.
func shotPlanFromEnv(getenv func(string) string) (*shotPlan, error) {
	dir := getenv(shotDirEnv)
	list := getenv(shotFramesEnv)
	script := getenv(shotInputEnv)
	if dir == "" && list == "" && script == "" {
		return nil, nil
	}
	if dir == "" || list == "" {
		return nil, fmt.Errorf("golib.Run: set both %s and %s to take screenshots, or neither (golib shot sets them)", shotDirEnv, shotFramesEnv)
	}
	seen := make(map[int]bool)
	var frames []int
	for _, field := range strings.Split(list, ",") {
		frame, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil || frame < 1 {
			return nil, fmt.Errorf("golib.Run: invalid %s %q: use frame numbers from 1, separated by commas", shotFramesEnv, list)
		}
		if !seen[frame] {
			seen[frame] = true
			frames = append(frames, frame)
		}
	}
	sort.Ints(frames)
	input, err := parseInputScript(script)
	if err != nil {
		return nil, err
	}
	return &shotPlan{dir: dir, frames: frames, input: input}, nil
}

// hold holds a key or a button down from update first to update last, both
// included.
type hold[T comparable] struct {
	of          T
	first, last int
}

// heldAt reports whether holds keep of down in update number update.
func heldAt[T comparable](holds []hold[T], of T, update int) bool {
	for _, h := range holds {
		if h.of == of && h.first <= update && update <= h.last {
			return true
		}
	}
	return false
}

// mouseMove puts the mouse pointer at x, y from update number update on.
type mouseMove struct {
	update int
	x, y   float32
}

// stickMove tilts a stick of gamepad 0 to x, y from update number update on.
type stickMove struct {
	right  bool // the right stick; false for the left one
	update int
	x, y   float32
}

// wheelTurn turns the mouse wheel by amount notches in update number update.
type wheelTurn struct {
	update int
	amount float32
}

// inputScript is the input golib shot plays. The script
// "Enter@1 Right@30-90 Mouse@100:640,360 MouseLeft@101 GamepadA@120" holds
// Enter down in update 1, which is one press, holds Right from update 30 to
// update 90, moves the mouse pointer to 640, 360 in update 100, clicks the left
// mouse button in update 101 and presses A on gamepad 0 in update 120.
type inputScript struct {
	keys           []hold[Key]
	buttons        []hold[MouseButton]
	gamepadButtons []hold[GamepadButton]
	moves          []mouseMove // in update order
	sticks         []stickMove // in update order
	wheel          []wheelTurn
}

// parseInputScript parses an input script: items separated by spaces. Each is
// Name@update or Name@first-last to hold a key or a button down,
// Mouse@update:x,y to move the mouse pointer, MouseWheel@update:notches to
// turn the wheel, or GamepadLeftStick@update:x,y or GamepadRightStick@update:x,y
// to tilt a stick. Names are those of the Key constants without the Key
// prefix, and of the MouseButton and GamepadButton constants, in any letter
// case. Gamepad items act on gamepad 0. An empty script plays no input.
func parseInputScript(script string) (inputScript, error) {
	var s inputScript
	for _, item := range strings.Fields(script) {
		name, when, found := strings.Cut(item, "@")
		if !found {
			return inputScript{}, inputScriptError(item, "write Name@update, Name@first-last or Mouse@update:x,y")
		}
		switch {
		case strings.EqualFold(name, "Mouse"):
			update, x, y, ok := parsePointAt(when)
			if !ok {
				return inputScript{}, inputScriptError(item, "Mouse@update:x,y moves the mouse pointer to pixel x, y, such as Mouse@100:640,360")
			}
			s.moves = append(s.moves, mouseMove{update: update, x: x, y: y})
			continue
		case strings.EqualFold(name, "MouseWheel"):
			update, amount, ok := parseAmountAt(when)
			if !ok {
				return inputScript{}, inputScriptError(item, "MouseWheel@update:notches turns the mouse wheel, up when positive, such as MouseWheel@50:-1")
			}
			s.wheel = append(s.wheel, wheelTurn{update: update, amount: amount})
			continue
		case strings.EqualFold(name, "GamepadLeftStick"), strings.EqualFold(name, "GamepadRightStick"):
			update, x, y, ok := parsePointAt(when)
			if !ok || x < -1 || x > 1 || y < -1 || y > 1 {
				return inputScript{}, inputScriptError(item, "GamepadLeftStick@update:x,y tilts a stick, with x and y from -1 to 1, such as GamepadLeftStick@10:1,0")
			}
			s.sticks = append(s.sticks, stickMove{right: strings.EqualFold(name, "GamepadRightStick"), update: update, x: x, y: y})
			continue
		}

		key, isKey := byName(keyNames, name)
		mouseButton, isMouseButton := byName(mouseButtonNames, name)
		gamepadButton, isGamepadButton := byName(gamepadButtonNames, name)
		if !isKey && !isMouseButton && !isGamepadButton {
			return inputScript{}, inputScriptError(item, fmt.Sprintf("there is no key or button named %q (keys: %s; mouse buttons: %s; gamepad buttons: %s)",
				name, strings.Join(sortedNames(keyNames), ", "), strings.Join(sortedNames(mouseButtonNames), ", "), strings.Join(sortedNames(gamepadButtonNames), ", ")))
		}
		first, last, ok := parseUpdates(when)
		if !ok {
			return inputScript{}, inputScriptError(item, "updates count from 1, and a range runs from its first update to the same or a later one")
		}
		switch {
		case isKey:
			s.keys = append(s.keys, hold[Key]{of: key, first: first, last: last})
		case isMouseButton:
			s.buttons = append(s.buttons, hold[MouseButton]{of: mouseButton, first: first, last: last})
		default:
			s.gamepadButtons = append(s.gamepadButtons, hold[GamepadButton]{of: gamepadButton, first: first, last: last})
		}
	}
	sort.SliceStable(s.moves, func(i, j int) bool { return s.moves[i].update < s.moves[j].update })
	sort.SliceStable(s.sticks, func(i, j int) bool { return s.sticks[i].update < s.sticks[j].update })
	return s, nil
}

// parseUpdates parses "N", one update, or "A-B", a range of updates.
func parseUpdates(text string) (first, last int, ok bool) {
	firstText, lastText, isRange := strings.Cut(text, "-")
	first, err := strconv.Atoi(firstText)
	if err != nil {
		return 0, 0, false
	}
	last = first
	if isRange {
		if last, err = strconv.Atoi(lastText); err != nil {
			return 0, 0, false
		}
	}
	return first, last, first >= 1 && last >= first
}

// parsePointAt parses "update:x,y".
func parsePointAt(text string) (update int, x, y float32, ok bool) {
	updateText, point, hasColon := strings.Cut(text, ":")
	xText, yText, hasComma := strings.Cut(point, ",")
	update, updateErr := strconv.Atoi(updateText)
	parsedX, xErr := strconv.ParseFloat(xText, 32)
	parsedY, yErr := strconv.ParseFloat(yText, 32)
	if !hasColon || !hasComma || updateErr != nil || xErr != nil || yErr != nil || update < 1 {
		return 0, 0, 0, false
	}
	return update, float32(parsedX), float32(parsedY), true
}

// parseAmountAt parses "update:amount".
func parseAmountAt(text string) (update int, amount float32, ok bool) {
	updateText, amountText, hasColon := strings.Cut(text, ":")
	update, updateErr := strconv.Atoi(updateText)
	parsed, amountErr := strconv.ParseFloat(amountText, 32)
	if !hasColon || updateErr != nil || amountErr != nil || update < 1 {
		return 0, 0, false
	}
	return update, float32(parsed), true
}

func inputScriptError(item, reason string) error {
	return fmt.Errorf("golib.Run: invalid input %q for golib shot --input: %s. Example: --input \"Enter@1 Right@30-90 Mouse@100:640,360 MouseLeft@101\"", item, reason)
}

// at returns the input update number update sees: the keys and buttons the
// script holds down, which of those were up in the update before, where the
// latest moves so far put the mouse pointer (0, 0 before any) and the sticks,
// and how far the wheel turns in this update. Gamepad 0 is connected when the
// script uses it.
func (s inputScript) at(update int) Input {
	var in Input
	for _, h := range s.keys {
		if heldAt(s.keys, h.of, update) {
			in.down[h.of] = true
			in.pressed[h.of] = !heldAt(s.keys, h.of, update-1)
		}
	}
	for _, h := range s.buttons {
		if heldAt(s.buttons, h.of, update) {
			in.mouseDown[h.of] = true
			in.mousePressed[h.of] = !heldAt(s.buttons, h.of, update-1)
		}
	}
	for _, move := range s.moves {
		if move.update > update {
			break
		}
		in.mouseX, in.mouseY = move.x, move.y
	}
	for _, turn := range s.wheel {
		if turn.update == update {
			in.mouseWheel += turn.amount
		}
	}

	if len(s.gamepadButtons) == 0 && len(s.sticks) == 0 {
		return in
	}
	gamepad := &in.gamepads[0]
	gamepad.connected = true
	gamepad.name = scriptedGamepadName
	for _, h := range s.gamepadButtons {
		if heldAt(s.gamepadButtons, h.of, update) {
			gamepad.down[h.of] = true
			gamepad.pressed[h.of] = !heldAt(s.gamepadButtons, h.of, update-1)
		}
	}
	for _, stick := range s.sticks {
		if stick.update > update {
			break
		}
		if stick.right {
			gamepad.rightX, gamepad.rightY = stick.x, stick.y
		} else {
			gamepad.leftX, gamepad.leftY = stick.x, stick.y
		}
	}
	return in
}

// shotFileName returns the file name for a frame's screenshot, such as
// frame-000060.png.
func shotFileName(frame int) string {
	return fmt.Sprintf("frame-%06d.png", frame)
}

// runShots runs the game in a hidden window, without waiting between frames.
// Every frame runs exactly one update, so frame N always shows the game after N
// updates, on any machine. The planned frames are saved as PNG files.
func runShots(game Game, config Config, plan *shotPlan) error {
	if err := os.MkdirAll(plan.dir, 0o755); err != nil {
		return fmt.Errorf("golib.Run: cannot create the screenshot folder: %w", err)
	}
	if err := openWindow(config, true); err != nil {
		return err
	}
	defer rl.CloseWindow()

	// Draw the final picture into a texture instead of the window: a hidden or
	// covered window has no reliable pixels to read back.
	render := newRenderer(config)
	defer render.close()
	picture := render.loadTarget()
	defer rl.UnloadRenderTexture(picture)
	whole := rl.Rectangle{Width: float32(config.Width), Height: float32(config.Height)}

	screen := &Screen{width: float32(config.Width), height: float32(config.Height)}
	scene := game
	var input Input
	next := 0
	for frame := 1; next < len(plan.frames); frame++ {
		// Frame N shows the game after update N, which sees the script's input
		// for update N. Real devices are ignored, so shots repeat.
		fill := func(in *Input) { *in = plan.input.at(frame) }
		var (
			quit bool
			err  error
		)
		scene, quit, err = runUpdates(scene, &input, fill, 1)
		if err != nil {
			return err
		}
		if quit {
			return fmt.Errorf("golib.Run: the game called golib.Quit in update %d, so frame %d can't be captured: take screenshots of earlier frames", frame, plan.frames[next])
		}
		screen.time = float32(frame) * updateStep
		if err := render.drawScene(scene, screen); err != nil {
			return err
		}
		if frame == plan.frames[next] {
			// The screenshot shows the picture after post-processing, at the
			// screen's size.
			if err := render.present(&picture, whole, float32(frame)*updateStep); err != nil {
				return err
			}
			if err := saveTexture(picture.Texture, filepath.Join(plan.dir, shotFileName(frame))); err != nil {
				return err
			}
			next++
		}
		// An empty drawing pass lets raylib handle window events, as every
		// frame of a normal run does.
		rl.BeginDrawing()
		rl.EndDrawing()
	}
	return nil
}

// saveTexture writes a render texture to a PNG file.
func saveTexture(texture rl.Texture2D, path string) error {
	image := rl.LoadImageFromTexture(texture)
	defer rl.UnloadImage(image)
	rl.ImageFlipVertical(image)                  // render textures are stored bottom row first
	rl.ImageFormat(image, rl.UncompressedR8g8b8) // drop alpha, which blending leaves below 255 at soft edges
	if !rl.ExportImage(*image, path) {
		return fmt.Errorf("golib.Run: could not save the screenshot %s", path)
	}
	return nil
}
