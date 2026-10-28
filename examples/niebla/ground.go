package main

import (
	"math"

	"golib"
)

// The ground's look: the relief as lit and shaded slopes, the cover as
// zones of color with a grain, rocks, bushes and tufts close up, and the
// deposits as bodies of cells. View only: it reads the region and the
// state and keeps nothing but caches.

const (
	// propZoom is the zoom from which rocks and bushes are drawn: zoomed
	// out they are a speck of noise, and their growing as you zoom in is
	// the detail the zoom is for.
	propZoom = 4
	// tuftZoom is the zoom from which the ground shows its tufts and
	// pebbles.
	tuftZoom = 8

	slopeLight = 0.25  // how much a slope facing the light gains, per level
	slopeShade = 0.12  // how much one turned to the viewer's left loses
	levelTint  = 0.05  // how much lighter the ground reads per level up
	grainDepth = 0.035 // the cells' mottle, as a part of their color

	// groundReach is how far from the core a tile's ground is drawn, in
	// tiles: to where the fog is whole, and a tile's corner past it, so the
	// ground's stepped edge never shows through the thinner mist.
	groundReach = fogLineRadius - 0.7 + fogFadeTiles + 0.75
)

// The cover's colors, from bare ground to thick scrub, and where along
// the cover each one stands.
var coverStops = []struct {
	at    float32
	color golib.Color
}{
	{0, groundColor},
	{0.4, golib.Color{R: 86, G: 98, B: 100, A: 255}},
	{0.65, golib.Color{R: 84, G: 110, B: 92, A: 255}},
	{1, golib.Color{R: 68, G: 98, B: 78, A: 255}},
}

// groundStrides are the sides, in cells, of the blocks the ground is
// drawn by: the farther out, the larger, which keeps the ground at a few
// thousand triangles a frame whatever the zoom.
var groundStrides = [...]int{4, 2, 1}

// groundPaint is the ground's colors for one region, a layer per stride.
type groundPaint struct {
	region *Region
	layers [len(groundStrides)][]golib.Color
}

var paintCache *groundPaint

func paintOf(g *Region) *groundPaint {
	if paintCache != nil && paintCache.region == g {
		return paintCache
	}
	p := &groundPaint{region: g}
	for layer, stride := range groundStrides {
		cols, rows := regionCellCols/stride, regionCellRows/stride
		colors := make([]golib.Color, cols*rows)
		for row := 0; row < rows; row++ {
			for col := 0; col < cols; col++ {
				colors[row*cols+col] = blockColor(g, col*stride, row*stride, stride)
			}
		}
		p.layers[layer] = colors
	}
	paintCache = p
	return p
}

// blockColor is the mean of a block's cells.
func blockColor(g *Region, col, row, stride int) golib.Color {
	var r, gr, b float32
	for dr := 0; dr < stride; dr++ {
		for dc := 0; dc < stride; dc++ {
			c := cellColor(g, col+dc, row+dr)
			r, gr, b = r+float32(c.R), gr+float32(c.G), b+float32(c.B)
		}
	}
	n := float32(stride * stride)
	return golib.Color{R: uint8(r / n), G: uint8(gr / n), B: uint8(b / n), A: 255}
}

// cellColor is a cell's ground: its cover's color, lighter the higher it
// stands, mottled by a stable grain, and lit by its slope. The light
// comes from the screen's upper left, which is down the world's x axis.
func cellColor(g *Region, col, row int) golib.Color {
	color := coverColor(g.coverAt(col, row))
	l00, l10 := float32(g.level(col, row)), float32(g.level(col+1, row))
	l01, l11 := float32(g.level(col, row+1)), float32(g.level(col+1, row+1))
	alongX := (l10 + l11 - l00 - l01) / 2
	alongY := (l01 + l11 - l00 - l10) / 2
	mean := (l00+l10+l01+l11)/4 - reliefPlain
	light := 1 + levelTint*mean + slopeLight*alongX - slopeShade*alongY
	light += grainDepth * (float32(lattice(uint64(g.Seed)+11, col, row)) - 0.5) * 2
	scale := func(v uint8) uint8 {
		return uint8(golib.Clamp(float32(v)*light, 0, 255))
	}
	return golib.Color{R: scale(color.R), G: scale(color.G), B: scale(color.B), A: 255}
}

func coverColor(cover float32) golib.Color {
	for i := 1; i < len(coverStops); i++ {
		low, high := coverStops[i-1], coverStops[i]
		if cover <= high.at {
			return blend(low.color, high.color, (cover-low.at)/(high.at-low.at))
		}
	}
	return coverStops[len(coverStops)-1].color
}

// blend mixes two colors, part of the way from a to b.
func blend(a, b golib.Color, part float32) golib.Color {
	mix := func(x, y uint8) uint8 {
		return uint8(golib.Lerp(float32(x), float32(y), golib.Clamp(part, 0, 1)))
	}
	return golib.Color{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: a.A}
}

// cornerOnScreen returns where a cell corner lands on the screen, lifted
// by the relief.
func cornerOnScreen(g *Region, vcol, vrow int) golib.Vector2 {
	x, y := projectFlat(float32(vcol*buildingCell), float32(vrow*buildingCell))
	return golib.Vector2{X: x, Y: y - g.cornerHeight(vcol, vrow)*unitH}
}

// blockQuad returns a block's corners on the screen: back, right, front
// and left.
func blockQuad(g *Region, col, row, stride int) [4]golib.Vector2 {
	return [4]golib.Vector2{
		cornerOnScreen(g, col, row),
		cornerOnScreen(g, col+stride, row),
		cornerOnScreen(g, col+stride, row+stride),
		cornerOnScreen(g, col, row+stride),
	}
}

func drawQuad(screen *golib.Screen, q [4]golib.Vector2, color golib.Color) {
	screen.DrawTriangle(q[0].X, q[0].Y, q[1].X, q[1].Y, q[2].X, q[2].Y, color)
	screen.DrawTriangle(q[0].X, q[0].Y, q[2].X, q[2].Y, q[3].X, q[3].Y, color)
}

// cellsInView returns the range of cells a view of the screen may show,
// a little wider than it for the relief's lift.
func cellsInView(view golib.Rectangle) (col0, row0, col1, row1 int) {
	const margin = 2 * buildingCell
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, corner := range [4][2]float32{
		{view.X, view.Y}, {view.X + view.Width, view.Y},
		{view.X, view.Y + view.Height}, {view.X + view.Width, view.Y + view.Height},
	} {
		a := float64(corner[0]-regionOriginX) / float64(unitW/2)
		b := float64(corner[1]-regionOriginY) / float64(unitH/2)
		x, y := (a+b)/2, (b-a)/2
		minX, maxX = math.Min(minX, x), math.Max(maxX, x)
		minY, maxY = math.Min(minY, y), math.Max(maxY, y)
	}
	clamp := func(v float64, most int) int {
		return int(math.Max(0, math.Min(float64(most), v/buildingCell)))
	}
	return clamp(minX-margin, regionCellCols), clamp(minY-margin, regionCellRows),
		clamp(maxX+margin, regionCellCols), clamp(maxY+margin, regionCellRows)
}

// quadInView reports whether a quad may show in a view.
func quadInView(q [4]golib.Vector2, view golib.Rectangle) bool {
	return q[1].X >= view.X && q[3].X <= view.X+view.Width &&
		q[2].Y+levelHeight*reliefLevels*unitH >= view.Y &&
		q[0].Y-levelHeight*reliefLevels*unitH <= view.Y+view.Height
}

// drawGround paints the ground inside the fog's reach, by blocks sized
// to the zoom: the farther out, the larger the block and the fewer of
// them. Close up it adds the props and the tufts.
func drawGround(
	s *State,
	screen *golib.Screen,
	zoom float32,
	view golib.Rectangle,
) {
	g := land
	layer := 0
	switch {
	case zoom >= 3:
		layer = 2
	case zoom >= 1.4:
		layer = 1
	}
	stride := groundStrides[layer]
	colors := paintOf(g).layers[layer]
	cols := regionCellCols / stride
	col0, row0, col1, row1 := cellsInView(view)
	for row := row0 / stride; row <= min(row1/stride, cols-1); row++ {
		for col := col0 / stride; col <= min(col1/stride, cols-1); col++ {
			tcol, trow := cellTile(col*stride, row*stride)
			if !groundShows(s, tcol, trow) {
				continue
			}
			q := blockQuad(g, col*stride, row*stride, stride)
			if quadInView(q, view) {
				drawQuad(screen, q, colors[row*cols+col])
			}
		}
	}
	if zoom < propZoom {
		return
	}
	taken := map[[2]int]bool{}
	for _, b := range s.Buildings {
		taken[[2]int{b.Col, b.Row}] = true
	}
	for _, job := range s.Jobs {
		taken[[2]int{job.Col, job.Row}] = true
	}
	for row := row0; row <= min(row1, regionCellRows-1); row++ {
		for col := col0; col <= min(col1, regionCellCols-1); col++ {
			tcol, trow := cellTile(col, row)
			if tileAt(tcol, trow) != kindGround || taken[[2]int{col, row}] ||
				!groundShows(s, tcol, trow) {
				continue
			}
			q := blockQuad(g, col, row, 1)
			if !quadInView(q, view) {
				continue
			}
			if zoom >= tuftZoom {
				drawTufts(screen, g, col, row)
			}
			drawProp(screen, g, col, row)
		}
	}
}

// groundSpot returns a stable spot on a cell, off its middle by up to
// spread of its side, on the screen.
func groundSpot(g *Region, col, row, salt int, spread float32) (x, y float32) {
	cx, cy := cellCenterUnits(col, row)
	return project(
		float32(cx)+jitter(col, row, salt)*2*spread*buildingCell,
		float32(cy)+jitter(col, row, salt+1)*2*spread*buildingCell)
}

// drawTufts draws the ground's small print: blades where something
// grows, the more the thicker the cover, and pebbles where nothing does.
func drawTufts(screen *golib.Screen, g *Region, col, row int) {
	cover := g.coverAt(col, row)
	base := cellColor(g, col, row)
	if cover < 0.3 {
		for i := 0; i < 2; i++ {
			if lattice(uint64(g.Seed)+21, col*4+i, row) > 0.45 {
				continue
			}
			x, y := groundSpot(g, col, row, 20+i*2, 0.42)
			screen.DrawCircle(x, y, 0.5*unitW, blend(base, scarColor, 0.55))
		}
		return
	}
	blade := blend(base, golib.Color{R: 132, G: 160, B: 124, A: 255}, 0.55)
	dark := blend(base, scarColor, 0.35)
	for i := 0; i < int(cover*5); i++ {
		x, y := groundSpot(g, col, row, 30+i*2, 0.45)
		w, h := 0.7*unitW, 2.4*unitH
		screen.DrawTriangle(x-w, y, x-w*0.3, y-h, x+w*0.2, y, dark)
		screen.DrawTriangle(x-w*0.2, y, x+w*0.5, y-h*1.3, x+w, y, blade)
	}
}

// drawProp draws the rock or the bush a cell grows, if any: bushes in
// the thick cover, rocks on bare ground and, mostly, on the slopes.
// They are decoration; a building raised on the cell takes their place.
func drawProp(screen *golib.Screen, g *Region, col, row int) {
	cover := g.coverAt(col, row)
	chance := lattice(uint64(g.Seed)+31, col, row)
	size := float32(0.7 + 0.6*lattice(uint64(g.Seed)+32, col, row))
	x, y := groundSpot(g, col, row, 1, 0.3)
	switch {
	case cover > 0.62 && chance < float64(cover-0.62)*0.8:
		drawBush(screen, x, y, size)
	case !g.flatCell(col, row) && chance < 0.2, cover < 0.3 && chance < 0.05:
		drawRock(screen, x, y, size)
	}
}

// drawRock draws a rock about 6 u (6 m) across: two facets, dark against
// the light; world-sized, so it grows from a pebble to a boulder as the
// view closes in.
func drawRock(screen *golib.Screen, cx, cy, size float32) {
	u := 3 * unitW * size // the rock's half width
	screen.DrawPolygon([]golib.Vector2{
		{X: cx - u, Y: cy + 0.6*u},
		{X: cx - 0.4*u, Y: cy - 1.2*u},
		{X: cx + 0.8*u, Y: cy - 0.6*u},
		{X: cx + u, Y: cy + 0.6*u},
	}, rockColor)
	screen.DrawPolygon([]golib.Vector2{
		{X: cx - 0.4*u, Y: cy - 1.2*u},
		{X: cx + 0.8*u, Y: cy - 0.6*u},
		{X: cx + 0.2*u, Y: cy + 0.2*u},
		{X: cx - 0.4*u, Y: cy - 0.4*u},
	}, rockLightColor)
}

// drawBush draws a bush of three circles about 6 u across. World-sized,
// like the rock.
func drawBush(screen *golib.Screen, cx, cy, size float32) {
	r := 1.5 * unitW * size
	screen.DrawCircle(cx-r, cy, r, bushColor)
	screen.DrawCircle(cx+r, cy, r, bushColor)
	screen.DrawCircle(cx, cy-r, r, bushLightColor)
}

// oreShown returns how much of each cell's richness still shows: a
// deposit wears from its rim in, so what is left of it is the level that
// cuts its cells' richness down to the part that remains. The level is
// cached by deposit until the amount moves.
type oreLevel struct {
	region *Region
	amount float64
	level  float32
}

var oreLevels = map[int]oreLevel{}

func oreCut(g *Region, d Deposit, amount float64) float32 {
	if cached, ok := oreLevels[d.Index]; ok &&
		cached.region == g && cached.amount == amount {
		return cached.level
	}
	body := g.bodies[d.Index]
	whole := float32(0)
	for _, c := range body {
		whole += c.Rich
	}
	want := whole * float32(golib.Clamp(float32(amount/d.Full), 0, 1))
	low, high := float32(0), float32(1)
	for i := 0; i < 16; i++ {
		cut := (low + high) / 2
		left := float32(0)
		for _, c := range body {
			if c.Rich > cut {
				left += c.Rich - cut
			}
		}
		if left > want {
			low = cut
		} else {
			high = cut
		}
	}
	oreLevels[d.Index] = oreLevel{region: g, amount: amount, level: high}
	return high
}

// drawDeposits paints each deposit cell by cell: the richer the cell,
// the more there is on it - a wider sheet of oil, more and taller lilac
// crystals - thinning out to specks at the rim. A worked deposit wears
// from the rim in, and a dry one leaves the scar of its heart. Each body
// is painted in layers, so a cell's sheet never covers its neighbor's
// shine.
func drawDeposits(
	s *State,
	screen *golib.Screen,
	zoom float32,
	view golib.Rectangle,
) {
	g := land
	sides := 9
	if zoom < 2 {
		sides = 4
	}
	for _, d := range g.deposits {
		amount := s.Drain[depositKey(d)]
		cut := oreCut(g, d, amount)
		if amount <= 0 {
			cut = 2
		}
		layers := oilLayers
		if d.Kind == kindLilac {
			layers = lilacLayers
		}
		for _, layer := range layers {
			for _, c := range g.bodies[d.Index] {
				shown := c.Rich - cut
				if cut > 1 {
					shown = -c.Rich
				}
				size := layer.size(shown)
				if size <= 0 {
					continue
				}
				q := blockQuad(g, c.Col, c.Row, 1)
				if !quadInView(q, view) {
					continue
				}
				middle := q[0].Add(q[1]).Add(q[2]).Add(q[3]).Scale(0.25)
				middle.X += jitter(c.Col, c.Row, 40) * buildingCell * unitW * 0.4
				middle.Y += jitter(c.Col, c.Row, 41) * buildingCell * unitH * 0.4
				drawBlob(screen, middle, size, sides,
					jitter(c.Col, c.Row, 42), layer.color)
			}
		}
		if d.Kind != kindLilac || amount <= 0 {
			continue
		}
		for _, c := range g.bodies[d.Index] {
			if shown := c.Rich - cut; shown > 0 {
				drawCrystals(screen, g, c, shown, zoom, view)
			}
		}
	}
}

// oreLayer is one coat of a deposit's paint: its color, and how large a
// blot a cell gets for the ore it shows, in cells across; 0 is none.
// What shows is negative for a dry deposit: minus the cell's richness.
type oreLayer struct {
	color golib.Color
	size  func(shown float32) float32
}

// Rich cells' blots overlap their neighbors', so a body's middle is one
// sheet, and its rim breaks into puddles and specks.
var (
	oilLayers = []oreLayer{
		{golib.WithOpacity(scarColor, 0.55), func(shown float32) float32 {
			if shown < -0.45 {
				return 1.6
			}
			return stepSize(shown, 0, 0.9, 1.4)
		}},
		{oilDarkColor, func(shown float32) float32 {
			return stepSize(shown, 0.06, 0.45, 1.9)
		}},
		{oilColor, func(shown float32) float32 {
			return stepSize(shown, 0.35, 0.3, 2.2)
		}},
	}
	lilacLayers = []oreLayer{
		{golib.WithOpacity(scarColor, 0.5), func(shown float32) float32 {
			if shown < -0.45 {
				return 1.6
			}
			return 0
		}},
		{golib.WithOpacity(lilacDarkColor, 0.55), func(shown float32) float32 {
			return stepSize(shown, 0, 0.8, 1.6)
		}},
		{lilacDarkColor, func(shown float32) float32 {
			return stepSize(shown, 0.2, 0.4, 1.8)
		}},
	}
)

// stepSize is a blot's size: none under from, then base and growing.
func stepSize(shown, from, base, growth float32) float32 {
	if shown <= from {
		return 0
	}
	return base + (shown-from)*growth
}

// drawBlob paints a blot on the ground: a flattened, many-sided shape
// size cells across, turned by a part of a side so blots don't line up.
func drawBlob(
	screen *golib.Screen,
	middle golib.Vector2,
	size float32,
	sides int,
	turn float32,
	color golib.Color,
) {
	halfW := size * buildingCell * unitW / 2
	halfH := size * buildingCell * unitH / 2
	if sides == 4 {
		turn = 0
	}
	corner := func(i int) (x, y float32) {
		angle := (float64(i) + float64(turn)) / float64(sides) * 2 * math.Pi
		return middle.X + halfW*float32(math.Cos(angle)),
			middle.Y + halfH*float32(math.Sin(angle))
	}
	x0, y0 := corner(0)
	for i := 1; i < sides-1; i++ {
		x1, y1 := corner(i)
		x2, y2 := corner(i + 1)
		screen.DrawTriangle(x0, y0, x1, y1, x2, y2, color)
	}
}

// drawCrystals stands up to three crystals on a vein's cell, taller the
// richer. Far out they grow past their true size, and only the rich
// cells wear them.
func drawCrystals(
	screen *golib.Screen,
	g *Region,
	c OreCell,
	shown, zoom float32,
	view golib.Rectangle,
) {
	if shown < 0.12 || zoom < 2 && shown < 0.35 {
		return
	}
	if !quadInView(blockQuad(g, c.Col, c.Row, 1), view) {
		return
	}
	far := float32(math.Max(1, float64(2.4/zoom)))
	crystals := 1 + int(shown*2.6)
	for i := 0; i < crystals; i++ {
		cx, cy := cellCenterUnits(c.Col, c.Row)
		x, y := project(
			float32(cx)+jitter(c.Col, c.Row, 5+i*2)*16,
			float32(cy)+jitter(c.Col, c.Row, 6+i*2)*16)
		w := (2 + 3*shown) * unitW * far
		h := (6 + 18*shown) * unitH * far * (1 - 0.25*float32(i))
		screen.DrawTriangle(x-w, y, x-w*0.2, y-h, x+w*0.3, y, lilacColor)
		screen.DrawTriangle(x+w*0.3, y, x-w*0.2, y-h, x+w, y, lilacLightColor)
	}
}

// groundShows reports whether a tile's ground is drawn: out to
// groundReach, and wherever a protector clears the air farther out.
func groundShows(s *State, tcol, trow int) bool {
	if tileDistance(tcol, trow) <= groundReach {
		return true
	}
	x, y := tileCenterUnits(tcol, trow)
	for _, b := range s.Buildings {
		if b.Kind != BuildingProtector {
			continue
		}
		bx, by := cellCenterUnits(b.Col, b.Row)
		if math.Hypot(x-bx, y-by) <=
			(protectorRadiusTiles(b)+0.75)*unitsPerTile {
			return true
		}
	}
	return false
}
