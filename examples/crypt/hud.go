package main

import (
	"fmt"

	"golib"
)

var (
	centered    = golib.TextOptions{Align: golib.AlignCenter}
	rightJusted = golib.TextOptions{Align: golib.AlignRight}
)

// drawText draws text with a dark shadow under it, so it reads over the
// dark stone and the torchlight.
func drawText(screen *golib.Screen, text string, x, y, size float32, color golib.Color, options ...golib.TextOptions) {
	offset := size / 10
	screen.DrawText(text, x+offset, y+offset, size, textShadowColor, options...)
	screen.DrawText(text, x, y, size, color, options...)
}

// drawHUD draws the hearts and keys, the time, a "!" over every guard after
// the player, the message of the moment, and a prompt by a crate.
func drawHUD(screen *golib.Screen, w *world, camera *golib.Camera3D, input hudInput) {
	screen.DrawRectangle(golib.Rectangle{X: 16, Y: 16, Width: 272, Height: 96}, panelColor)
	for i := range startHearts {
		drawHeart(screen, float32(44+i*42), 44, i < w.hearts)
	}
	total := w.level.keysInLevel()
	for i := range total {
		drawKeyIcon(screen, float32(36+i*44), 84, i < w.keysFound, i >= w.keysFound-w.keysHeld && i < w.keysFound)
	}

	right := screen.Width() - 16
	screen.DrawRectangle(golib.Rectangle{X: right - 250, Y: 16, Width: 250, Height: 72}, panelColor)
	room := w.level.checkpoints[w.checkpoint].room
	drawText(screen, roomTitle(room), right-16, 26, 20, accentColor, rightJusted)
	drawText(screen, clock(w.time), right-16, 54, 20, textColor, rightJusted)

	for _, g := range w.guards {
		if g.state != chasing {
			continue
		}
		over, ok := camera.ToScreen(g.body.Position.Add(golib.Vector3{Y: playerHeight + 0.35}))
		if !ok {
			continue
		}
		size := float32(40)
		if g.alert.Running() {
			size = 40 + 30*(1-g.alert.Progress())
		}
		drawText(screen, "!", over.X, over.Y-size/2, size, alertColor, centered)
	}

	if w.messageFor.Running() {
		opacity := golib.Clamp(w.messageFor.Left()*3, 0, 1)
		drawText(screen, w.message, screen.Width()/2, screen.Height()-120, 30, golib.WithOpacity(textColor, opacity), centered)
	} else if !w.player.punch.Running() && w.nearestCrate() >= 0 {
		drawText(screen, input.breakPrompt, screen.Width()/2, screen.Height()-120, 20, golib.WithOpacity(accentColor, 0.9), centered)
	}
	drawText(screen, input.pausePrompt, screen.Width()/2, screen.Height()-36, 20, golib.WithOpacity(textColor, 0.6), centered)
	if !golib.WindowFocused() {
		drawText(screen, "Paused: click the window to carry on", screen.Width()/2, screen.Height()/2, 30, textColor, centered)
	}
}

// hudInput is what the prompts say, for the keyboard and mouse or for the
// gamepad, as the player plays now.
type hudInput struct {
	breakPrompt string
	pausePrompt string
}

// prompts are the HUD's prompts for how the player plays now.
func prompts(input *golib.Input) hudInput {
	if !golib.PlayingWithGamepad() {
		return hudInput{breakPrompt: "Click the crate, or press F, to break it", pausePrompt: "Esc: pause"}
	}
	switch input.GamepadType(0) {
	case golib.GamepadTypePlayStation:
		return hudInput{breakPrompt: "Square: break the crate", pausePrompt: "Options: pause"}
	case golib.GamepadTypeNintendo:
		return hudInput{breakPrompt: "Y: break the crate", pausePrompt: "+: pause"}
	}
	return hudInput{breakPrompt: "X: break the crate", pausePrompt: "Menu: pause"}
}

// roomTitle is a room's name for the HUD: "The great hall".
func roomTitle(room string) string {
	if room == "" {
		return ""
	}
	return "The " + room
}

// clock writes seconds as minutes and seconds: 2:05.
func clock(seconds float32) string {
	s := int(seconds)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// drawHeart draws a heart centered on x, y, filled or empty.
func drawHeart(screen *golib.Screen, x, y float32, full bool) {
	color := heartColor
	if !full {
		color = golib.WithOpacity(golib.White, 0.2)
	}
	screen.DrawCircle(x-7, y-5, 9, color)
	screen.DrawCircle(x+7, y-5, 9, color)
	screen.DrawTriangle(x-15.8, y-2, x+15.8, y-2, x, y+15, color)
}

// drawKeyIcon draws a key lying on its side, its bow at x, y: gold once
// found, and outlined while it is still held, not yet used on a door.
func drawKeyIcon(screen *golib.Screen, x, y float32, found, held bool) {
	color := accentColor
	if !found {
		color = golib.WithOpacity(golib.White, 0.2)
	}
	screen.DrawCircle(x, y, 8, color)
	screen.DrawCircle(x, y, 3.5, panelColor)
	screen.DrawRectangle(golib.Rectangle{X: x + 6, Y: y - 2, Width: 20, Height: 4}, color)
	screen.DrawRectangle(golib.Rectangle{X: x + 18, Y: y + 2, Width: 3, Height: 6}, color)
	screen.DrawRectangle(golib.Rectangle{X: x + 23, Y: y + 2, Width: 3, Height: 5}, color)
	if held {
		screen.DrawCircleOutline(x+10, y+1, 17, 2, golib.WithOpacity(accentColor, 0.7))
	}
}

// drawMessage darkens the screen and writes a title and lines under it, in the middle.
func drawMessage(screen *golib.Screen, title string, lines ...string) {
	screen.DrawRectangle(golib.Rectangle{Width: screen.Width(), Height: screen.Height()}, overlayColor)
	y := screen.Height()/2 - float32(40+30*len(lines))/2 - 30
	drawText(screen, title, screen.Width()/2, y, 60, accentColor, centered)
	for i, line := range lines {
		drawText(screen, line, screen.Width()/2, y+90+float32(i)*36, 20, textColor, centered)
	}
}
