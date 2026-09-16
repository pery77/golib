package main

import (
	"math"

	"golib"
)

// Clouds drift behind the level, slower than the camera.
const (
	cloudCount    = 8
	cloudParallax = 0.25 // how far clouds move for each pixel the camera moves
	cloudSpan     = 520  // pixels across which clouds are scattered: the screen and what the parallax reveals
)

// camera returns the top-left corner of the part of the level the screen
// shows: the player in the middle, as far as the level goes. It is in whole
// pixels, as the map and the sprites are drawn, so they move together.
func camera(w *world) (float32, float32) {
	p := w.player
	x := max(0, min(p.x+playerWidth/2-screenWidth/2, w.width-screenWidth))
	y := max(0, min(p.y+playerHeight/2-screenHeight/2, w.height-screenHeight))
	return float32(math.Round(float64(x))), float32(math.Round(float64(y)))
}

// drawWorld draws the level and everything in it, from back to front. The map
// is drawn one layer at a time, so that the chests, the snakes and the hero
// go between its layers.
func drawWorld(screen *golib.Screen, w *world, clouds []cloud) {
	cameraX, cameraY := camera(w)
	x, y := -cameraX, -cameraY // where the level's top-left corner goes on the screen
	screen.Clear(skyColor)
	drawClouds(screen, clouds, cameraX)
	screen.DrawMapLayer(level, "far", x, y) // Tiled gives it a parallax factor of 0.5 and a blue tint
	screen.DrawMapLayer(level, "back", x, y)
	screen.DrawMapLayer(level, groundLayer, x, y)
	for _, c := range w.chests {
		frame := closedChestFrame
		if c.open {
			frame = openChestFrame
		}
		screen.DrawSprite(tiles, frame, c.bounds.X+x, c.bounds.Y+y)
	}
	for _, s := range w.snakes {
		if !s.defeated {
			left, top := s.x-snakeOffsetX+x, s.y+snakeHeight-frameSize+y
			screen.DrawSprite(characters, snakeCrawl.Frame(s.crawlTime), left, top, golib.DrawOptions{FlipX: s.facingLeft})
		}
	}
	drawHero(screen, w.player, x, y)
	screen.DrawMapLayer(level, "front", x, y) // grass and flowers, in front of the hero's feet
}

// drawHero draws the player in the pose it is in, with the level's top-left
// corner at x, y. The hero faces right in the sheet, so facing left flips the
// frame.
func drawHero(screen *golib.Screen, p player, x, y float32) {
	frame := heroStand
	switch {
	case p.slashLeft > 0:
		frame = heroSlash.Frame(slashTime - p.slashLeft)
	case !p.onGround && p.velocityY < 0:
		frame = heroRise
	case !p.onGround:
		frame = heroFall
	case p.walkTime > 0:
		frame = heroWalk.Frame(p.walkTime)
	}
	flip := golib.DrawOptions{FlipX: p.facingLeft}
	screen.DrawSprite(characters, frame, p.x-heroOffsetX+x, p.y+playerHeight-frameSize+y, flip)

	if p.slashLeft > 0 {
		// The swoosh's crescent is in the right half of its frame: put that
		// half in front of the hero.
		middle := p.x + playerWidth/2 + x
		left := middle - 8
		if p.facingLeft {
			left = middle - frameSize + 8
		}
		top := p.y + playerHeight/2 - frameSize/2 + y
		screen.DrawSprite(swoosh, slashEffect.Frame(slashTime-p.slashLeft), left, top, flip)
	}
}

// cloud is a background decoration made of three circles.
type cloud struct {
	x, y   float32 // center of the middle circle, in pixels
	radius float32 // radius of the middle circle, in pixels
}

// newClouds scatters clouds across the upper sky. They are random, so each
// play looks a little different; golib shot always gets the same ones.
func newClouds() []cloud {
	clouds := make([]cloud, cloudCount)
	for i := range clouds {
		clouds[i] = cloud{
			x:      golib.RandomFloat(0, cloudSpan),
			y:      golib.RandomFloat(12, 60),
			radius: golib.RandomFloat(5, 9),
		}
	}
	return clouds
}

func drawClouds(screen *golib.Screen, clouds []cloud, cameraX float32) {
	shift := float32(math.Round(float64(cameraX * cloudParallax)))
	for _, c := range clouds {
		x := c.x - shift
		screen.DrawCircle(x-c.radius, c.y+c.radius/3, c.radius*0.7, cloudColor)
		screen.DrawCircle(x+c.radius, c.y+c.radius/3, c.radius*0.7, cloudColor)
		screen.DrawCircle(x, c.y, c.radius, cloudColor)
	}
}

// drawMessage darkens the whole screen and shows a heading and a few hints in
// the middle.
func drawMessage(screen *golib.Screen, heading string, hints ...string) {
	screen.DrawRectangle(golib.Rectangle{Width: screen.Width(), Height: screen.Height()}, overlayColor)
	drawCentered(screen, heading, 56, 20)
	for i, hint := range hints {
		drawCentered(screen, hint, 90+float32(i)*14, 10)
	}
}

// drawCentered draws text centered across the screen, with its top at y.
func drawCentered(screen *golib.Screen, text string, y, size float32) {
	drawShadowed(screen, text, (screen.Width()-screen.TextWidth(text, size))/2, y, size)
}

// drawShadowed draws text with a dark copy one pixel down and to the right,
// so it reads over the level.
func drawShadowed(screen *golib.Screen, text string, x, y, size float32) {
	screen.DrawText(text, x+1, y+1, size, shadowColor)
	screen.DrawText(text, x, y, size, textColor)
}
