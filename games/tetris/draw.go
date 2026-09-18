package main

import (
	"fmt"

	"golib"
)

// particle is a spark thrown off when a line clears. play.go moves them and
// this file draws them.
type particle struct {
	x, y   float32
	vx, vy float32
	life   float32 // seconds left
	max    float32 // seconds it started with, for the fade
	color  golib.Color
}

// drawPlay draws the whole game: the board and its blocks, the hold and next
// panels, the score, and the effects. It never changes the game's state.
func drawPlay(screen *golib.Screen, s *playScene) {
	screen.Clear(backgroundColor)

	// The board shudders with the shake, so the blocks and its frame move
	// together.
	sx, sy := s.shakeX, s.shakeY

	// The board itself: a dark panel with a border and a faint grid.
	board := golib.Rectangle{X: boardX + sx, Y: boardY + sy, Width: boardPixelW, Height: boardPixelH}
	screen.DrawRectangle(board, boardColor)
	drawGrid(screen, board)

	drawStack(screen, s, sx, sy)
	drawActivePiece(screen, s, sx, sy)
	drawParticles(screen, s)

	// The flash a clear sets off: a white wash over the board, fading out.
	if s.flash > 0 {
		screen.DrawRectangle(board, golib.WithOpacity(golib.White, s.flash))
	}
	screen.DrawRectangleOutline(board, 3, boardEdgeColor)

	drawHoldPanel(screen, s)
	drawNextPanel(screen, s)
	drawStatsPanel(screen, s)
	drawToast(screen, s)

	if !golib.WindowFocused() {
		drawCentered(screen, "Paused: click the window to carry on", screen.Height()-26, 20, dimTextColor)
	}
}

// drawGrid draws the faint lines between the board's cells.
func drawGrid(screen *golib.Screen, board golib.Rectangle) {
	for x := 1; x < boardColumns; x++ {
		px := board.X + float32(x*boardCell)
		screen.DrawLine(px, board.Y, px, board.Y+board.Height, 1, gridColor)
	}
	for y := 1; y < boardRows; y++ {
		py := board.Y + float32(y*boardCell)
		screen.DrawLine(board.X, py, board.X+board.Width, py, 1, gridColor)
	}
}

// drawStack draws every block fixed on the board. A row that is clearing
// flashes white instead of its color.
func drawStack(screen *golib.Screen, s *playScene, sx, sy float32) {
	clearing := s.world.phase == phaseClearing
	for y := 0; y < boardRows; y++ {
		rowClearing := clearing && s.world.isFull(y)
		for x := 0; x < boardColumns; x++ {
			kind := s.world.board[y][x]
			if kind == 0 {
				continue
			}
			px := boardX + float32(x*boardCell) + sx
			py := boardY + float32(y*boardCell) + sy
			color := pieceColors[kind-1]
			if rowClearing {
				// Pulse between white and the block's own color.
				pulse := 0.5 + 0.5*float32(s.world.clearTimer/clearDuration)
				color = mixColors(golib.White, color, pulse)
			}
			drawBlock(screen, px, py, color)
		}
	}
}

// drawActivePiece draws the piece the player is moving and its ghost: a faint
// outline where it would land.
func drawActivePiece(screen *golib.Screen, s *playScene, sx, sy float32) {
	piece := s.world.piece
	kind := int(piece.kind)
	if kind < 0 || kind >= int(pieceKinds) {
		return
	}
	color := pieceColors[kind]

	// The ghost, unless the piece is already resting where it would land.
	ghost := s.world.ghostY()
	if ghost != piece.y {
		for _, c := range piece.cells() {
			gy := c.y + ghost - piece.y
			if gy < 0 {
				continue
			}
			px := boardX + float32(c.x*boardCell) + sx
			py := boardY + float32(gy*boardCell) + sy
			screen.DrawRectangleOutline(golib.Rectangle{X: px + 2, Y: py + 2, Width: boardCell - 4, Height: boardCell - 4},
				2, golib.WithOpacity(color, 0.45))
		}
	}

	for _, c := range piece.cells() {
		if c.y < 0 {
			continue
		}
		px := boardX + float32(c.x*boardCell) + sx
		py := boardY + float32(c.y*boardCell) + sy
		drawBlock(screen, px, py, color)
	}
}

// drawParticles draws the sparks from a clear, adding their light to what is
// behind them so they glow.
func drawParticles(screen *golib.Screen, s *playScene) {
	if len(s.particles) == 0 {
		return
	}
	screen.SetBlendMode(golib.BlendAdd)
	for _, p := range s.particles {
		fade := p.life / p.max
		screen.DrawCircle(p.x, p.y, 2+4*fade, golib.WithOpacity(p.color, fade))
	}
	screen.SetBlendMode(golib.BlendNormal)
}

// drawBlock draws one board cell as a block: a filled square with a lighter top
// and left edge and a darker bottom and right, so it reads as a little tile.
func drawBlock(screen *golib.Screen, x, y float32, color golib.Color) {
	const gap = 2
	size := float32(boardCell)
	inner := golib.Rectangle{X: x + gap, Y: y + gap, Width: size - 2*gap, Height: size - 2*gap}
	screen.DrawRectangle(inner, color)
	screen.DrawRectangle(golib.Rectangle{X: inner.X, Y: inner.Y, Width: inner.Width, Height: 3}, lighten(color, 0.35))
	screen.DrawRectangle(golib.Rectangle{X: inner.X, Y: inner.Y, Width: 3, Height: inner.Height}, lighten(color, 0.2))
	screen.DrawRectangle(golib.Rectangle{X: inner.X, Y: inner.Y + inner.Height - 3, Width: inner.Width, Height: 3}, darken(color, 0.35))
	screen.DrawRectangle(golib.Rectangle{X: inner.X + inner.Width - 3, Y: inner.Y, Width: 3, Height: inner.Height}, darken(color, 0.35))
	screen.DrawRectangleOutline(golib.Rectangle{X: x + 1, Y: y + 1, Width: size - 2, Height: size - 2}, 1, darken(color, 0.5))
}

// drawHoldPanel shows the piece set aside with the hold key.
func drawHoldPanel(screen *golib.Screen, s *playScene) {
	panel := golib.Rectangle{X: 60, Y: boardY, Width: 200, Height: 170}
	drawPanel(screen, panel, "HOLD")
	if !s.world.hasHold {
		drawCenteredIn(screen, "C or Shift", panel, dimTextColor)
		return
	}
	drawPieceIn(screen, panel, s.world.hold)
}

// drawNextPanel shows the piece that comes next.
func drawNextPanel(screen *golib.Screen, s *playScene) {
	panel := golib.Rectangle{X: screen.Width() - 60 - 200, Y: boardY, Width: 200, Height: 170}
	drawPanel(screen, panel, "NEXT")
	drawPieceIn(screen, panel, s.world.next)
}

// drawStatsPanel shows the score, the best score, the level and the lines.
func drawStatsPanel(screen *golib.Screen, s *playScene) {
	panel := golib.Rectangle{X: screen.Width() - 60 - 200, Y: boardY + 190, Width: 200, Height: 300}
	drawPanel(screen, panel, "SCORE")

	rows := []struct {
		label string
		value string
	}{
		{"Score", fmt.Sprint(s.world.score)},
		{"Best", fmt.Sprint(s.records.Best)},
		{"Level", fmt.Sprint(s.world.level)},
		{"Lines", fmt.Sprint(s.world.lines)},
	}
	y := panel.Y + 60
	for _, row := range rows {
		screen.DrawText(row.label, panel.X+16, y, 22, dimTextColor)
		screen.DrawText(row.value, panel.X+panel.Width-16, y, 26, textColor, golib.TextOptions{Align: golib.AlignRight})
		y += 54
	}
}

// drawToast shows "TETRIS!", "LEVEL 2" and the like, fading out over the board.
func drawToast(screen *golib.Screen, s *playScene) {
	if s.toastTimer <= 0 || s.toast == "" {
		return
	}
	fade := golib.Clamp(s.toastTimer, 0, 1)
	const size float32 = 64
	y := float32(boardY) + float32(boardPixelH)/2 - size/2
	drawCentered(screen, s.toast, y+4, size, golib.WithOpacity(golib.Black, fade*0.4))
	drawCentered(screen, s.toast, y, size, golib.WithOpacity(accentColor, fade))
}

// drawPanel draws a box with a title, used for hold, next and score.
func drawPanel(screen *golib.Screen, panel golib.Rectangle, title string) {
	screen.DrawRectangle(panel, panelColor)
	screen.DrawRectangleOutline(panel, 3, panelEdgeColor)
	screen.DrawText(title, panel.X+16, panel.Y+14, 26, dimTextColor)
}

// drawPieceIn draws a piece centered in the lower part of a panel, at a small
// size, so it fits whatever piece it is.
func drawPieceIn(screen *golib.Screen, panel golib.Rectangle, kind pieceKind) {
	shape := shapes[kind][0]
	// Find the piece's bounding box among its spawn cells.
	minX, minY, maxX, maxY := 4, 4, -1, -1
	for _, c := range shape {
		minX, minY = min(minX, c.x), min(minY, c.y)
		maxX, maxY = max(maxX, c.x), max(maxY, c.y)
	}
	cols := maxX - minX + 1
	rows := maxY - minY + 1
	const cell = 26
	width := float32(cols * cell)
	height := float32(rows * cell)
	// Center it in the panel, a little below the title.
	originX := panel.X + (panel.Width-width)/2
	originY := panel.Y + 50 + (panel.Height-50-height)/2
	color := pieceColors[kind]
	for _, c := range shape {
		x := originX + float32((c.x-minX)*cell)
		y := originY + float32((c.y-minY)*cell)
		box := golib.Rectangle{X: x + 1, Y: y + 1, Width: cell - 2, Height: cell - 2}
		screen.DrawRectangle(box, color)
		screen.DrawRectangleOutline(box, 2, lighten(color, 0.3))
	}
}

// drawCenteredIn draws text centered in a panel, in the built-in font.
func drawCenteredIn(screen *golib.Screen, text string, panel golib.Rectangle, color golib.Color) {
	const size = 20
	x := panel.X + (panel.Width-screen.TextWidth(text, size))/2
	y := panel.Y + panel.Height/2
	screen.DrawText(text, x, y, size, color)
}

// drawFallingBackdrop draws slow falling blocks behind the title, so the menu
// moves a little. time drives the animation.
func drawFallingBackdrop(screen *golib.Screen, time float32) {
	screen.Clear(backgroundColor)
	const cell = 48
	columns := int(screen.Width()/cell) + 1
	rows := int(screen.Height()/cell) + 2
	span := float32(rows * cell)
	for col := 0; col < columns; col++ {
		fall := time*22 + float32(col*67)
		for row := 0; row < rows; row++ {
			y := float32(row*cell) - mod(fall, span)
			kind := pieceKind((col + row) % int(pieceKinds))
			screen.DrawRectangle(golib.Rectangle{X: float32(col*cell) + 4, Y: y + 4, Width: cell - 8, Height: cell - 8},
				golib.WithOpacity(pieceColors[kind], 0.16))
		}
	}
}

// drawCentered draws text centered across the screen, with its top at y.
func drawCentered(screen *golib.Screen, text string, y, size float32, color golib.Color) {
	screen.DrawText(text, (screen.Width()-screen.TextWidth(text, size))/2, y, size, color)
}

// drawMessage darkens the whole screen and shows a heading and two hint lines
// in the middle.
func drawMessage(screen *golib.Screen, heading, line1, line2 string) {
	screen.DrawRectangle(golib.Rectangle{Width: screen.Width(), Height: screen.Height()}, golib.WithOpacity(golib.Black, 0.6))
	drawCentered(screen, heading, 250, 72, accentColor)
	drawCentered(screen, line1, 360, 30, textColor)
	drawCentered(screen, line2, 402, 24, dimTextColor)
}

// lighten returns color with each part moved a share towards white.
func lighten(color golib.Color, amount float32) golib.Color {
	return golib.Color{
		R: uint8(float32(color.R) + (255-float32(color.R))*amount),
		G: uint8(float32(color.G) + (255-float32(color.G))*amount),
		B: uint8(float32(color.B) + (255-float32(color.B))*amount),
		A: color.A,
	}
}

// darken returns color with each part moved a share towards black.
func darken(color golib.Color, amount float32) golib.Color {
	keep := 1 - amount
	return golib.Color{
		R: uint8(float32(color.R) * keep),
		G: uint8(float32(color.G) * keep),
		B: uint8(float32(color.B) * keep),
		A: color.A,
	}
}

// mixColors returns the color a share t of the way from a to b, both solid.
func mixColors(a, b golib.Color, t float32) golib.Color {
	t = golib.Clamp(t, 0, 1)
	mix := func(x, y uint8) uint8 { return uint8(float32(x) + (float32(y)-float32(x))*t) }
	return golib.Color{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: 255}
}

// mod returns a modulo b, both positive.
func mod(a, b float32) float32 {
	for a >= b {
		a -= b
	}
	return a
}
