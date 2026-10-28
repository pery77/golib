package main

import (
	"math"

	"golib"
)

// Where things go on the screen, in pixels.
const (
	headingY = 3                              // top of the text line above the room
	roomTop  = 16                             // top of the area the room is centered in
	hintY    = roomTop + maxRows*tileSize + 6 // top of the text line below the room
)

// roomOrigin returns where the top-left corner of level l goes on the
// screen: the room centered between the heading and the hints, on whole
// pixels, so tiles stay sharp.
func roomOrigin(l *layout) (x, y float32) {
	return float32((screenWidth - l.columns*tileSize) / 2), float32(roomTop + (maxRows-l.rows)*tileSize/2)
}

// cellPosition returns the top-left corner of cell c, in a room whose
// top-left corner is at x, y.
func cellPosition(c cell, x, y float32) (float32, float32) {
	return x + float32(c.column*tileSize), y + float32(c.row*tileSize)
}

// between returns the point a fraction t of the way from a to b.
func between(ax, ay, bx, by, t float32) (float32, float32) {
	return ax + (bx-ax)*t, ay + (by-ay)*t
}

// easeOut slows a motion down as it ends: t goes from 0 to 1.
func easeOut(t float32) float32 {
	return 1 - (1-t)*(1-t)
}

// warehouse is the decoration behind the title and the level list: a floor
// with walls along the top and the bottom, and a few crates, some on goals.
var warehouse = struct {
	crates, goals []cell
}{
	crates: []cell{{1, 3}, {2, 3}, {17, 2}, {18, 5}, {1, 8}, {18, 9}},
	goals:  []cell{{2, 3}, {18, 5}, {3, 6}, {17, 7}},
}

// drawWarehouse fills the screen with the warehouse, darkened so text reads
// over it.
func drawWarehouse(screen *golib.Screen) {
	screen.Clear(backgroundColor)
	rows := (screenHeight + tileSize - 1) / tileSize // the last row is cut off
	for row := range rows {
		for column := range screenWidth / tileSize {
			frame := floorFrame
			switch {
			case (row == 0 || row == rows-1) && (column*5+row)%7 == 3:
				frame = crackedWallFrame
			case row == 0 || row == rows-1:
				frame = wallFrame
			case (column*7+row*3)%11 == 0:
				frame = crackedFloorFrame
			}
			screen.DrawSprite(tiles, frame, float32(column*tileSize), float32(row*tileSize))
		}
	}
	for _, g := range warehouse.goals {
		x, y := cellPosition(g, 0, 0)
		screen.DrawSprite(tiles, goalFrame, x, y)
	}
	for _, c := range warehouse.crates {
		frame := crateFrame
		for _, g := range warehouse.goals {
			if g == c {
				frame = crateOnGoalFrame
			}
		}
		x, y := cellPosition(c, 0, 0)
		screen.DrawSprite(tiles, frame, x, y)
	}
	screen.DrawRectangle(golib.Rectangle{Width: screenWidth, Height: screenHeight}, overlayColor)
}

// panelWidth is the width of the box drawPanel draws, in pixels.
const panelWidth = 208

// drawPanel darkens the whole screen and draws a box from top to bottom
// across its middle, for a message or a menu over a scene.
func drawPanel(screen *golib.Screen, top, bottom float32) {
	screen.DrawRectangle(golib.Rectangle{Width: screenWidth, Height: screenHeight}, overlayColor)
	box := golib.Rectangle{X: (screenWidth - panelWidth) / 2, Y: top, Width: panelWidth, Height: bottom - top}
	screen.DrawRectangle(golib.Rectangle{X: box.X - 1, Y: box.Y - 1, Width: box.Width + 2, Height: box.Height + 2}, panelBorderColor)
	screen.DrawRectangle(box, panelColor)
}

// drawCentered draws text centered across the screen, with its top at y.
func drawCentered(screen *golib.Screen, text string, y, size float32, color golib.Color) {
	drawShadowed(screen, text, (screenWidth-screen.TextWidth(text, size))/2, y, size, color)
}

// drawRight draws text with its right side at x.
func drawRight(screen *golib.Screen, text string, x, y, size float32, color golib.Color) {
	drawShadowed(screen, text, x-screen.TextWidth(text, size), y, size, color)
}

// drawShadowed draws text with a dark copy one pixel down and to the right,
// so it reads over anything.
func drawShadowed(screen *golib.Screen, text string, x, y, size float32, color golib.Color) {
	screen.DrawText(text, x+1, y+1, size, shadowColor)
	screen.DrawText(text, x, y, size, color)
}

// drawStar draws a five-pointed star centered at x, y, radius pixels from the
// center to each point.
func drawStar(screen *golib.Screen, x, y, radius float32, color golib.Color) {
	var px, py [10]float32
	for i := range 10 {
		r := radius
		if i%2 == 1 {
			r = radius * 0.45
		}
		angle := float64(i)*math.Pi/5 - math.Pi/2
		px[i] = x + r*float32(math.Cos(angle))
		py[i] = y + r*float32(math.Sin(angle))
	}
	for i := range 10 {
		j := (i + 1) % 10
		screen.DrawTriangle(x, y, px[i], py[i], px[j], py[j], color)
	}
}
