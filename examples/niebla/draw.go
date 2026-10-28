package main

import (
	"fmt"
	"math"
	"sort"

	"golib"
)

// dotRadius returns the drawing radius, in projected pixels, of a world
// radius given in units, never smaller than minPx pixels on the screen:
// things show as points while the view is far out and grow to their true
// size as it closes in.
func dotRadius(units, zoom, minPx float32) float32 {
	r := units * unitW
	if min := minPx / zoom; r < min {
		r = min
	}
	return r
}

// drawRegion paints the still region and the robots on it: the ground
// inside the fog line, the core's monolith and its bubble, and the fog
// standing beyond the line. It reads the state and changes nothing.
func drawRegion(
	s *State,
	screen *golib.Screen,
	camera *golib.Camera,
	zoom float32,
	view golib.Rectangle,
) {
	screen.Clear(fogColor)
	drawGround(s, screen, zoom, view)
	drawCorePad(screen)
	drawDeposits(s, screen, zoom, view)
	drawMarks(s, screen, zoom)
	drawPipes(s, screen, zoom, true)
	drawPiles(s, screen, zoom, true)
	drawBuildings(s, screen, zoom)
	drawRobots(s, screen, camera, zoom)
	drawFogCover(s, screen, zoom, view)
	drawSwellWaves(s, screen)
	drawFogLine(screen, zoom, s)
	drawPipes(s, screen, zoom, false)
	drawPiles(s, screen, zoom, false)
	drawJobs(s, screen, zoom)
	drawEnemies(s, screen, camera, zoom)
	drawBubbles(s, screen, zoom)
}

// drawPiles paints the loose items, each pile a small heap on its
// cell's middle: crates for the lilac, a drum for the oil, whatever the
// amounts. The sheltered ones go under the buildings and the robots,
// which walk through them; the ones left in the mist are drawn over the
// fog, like the sites, so the colony never loses sight of its things.
func drawPiles(s *State, screen *golib.Screen, zoom float32, sheltered bool) {
	const (
		heapAcross = 14.0 // u, the whole heap
		heapHeight = 6.0
	)
	for _, id := range sortedPileIDs(s) {
		p := s.Piles[id]
		x, y := cellCenterUnits(p.Col, p.Row)
		if inSafeZone(s, x, y) != sheltered {
			continue
		}
		gx, gy := project(float32(x), float32(y))
		scale := pileMiteScale(p)
		k := buildingIcon(heapAcross*scale, heapHeight*scale, zoom)
		crate, drum := 6*k, 4.5*k
		if p.Lilac >= pileDust {
			isoBox(screen, gx-crate*unitW*0.45, gy, crate, 5*k,
				pileColor, mid(pileColor, pileDarkColor), pileDarkColor)
			isoBox(screen, gx+crate*unitW*0.1, gy+crate*unitH*0.55, crate, 4*k,
				pileColor, mid(pileColor, pileDarkColor), pileDarkColor)
			isoBox(screen, gx-crate*unitW*0.45, gy-5*k*unitH, crate*0.5, 1.5*k,
				lilacLightColor, lilacColor, lilacDarkColor)
		}
		if p.Oil >= pileDust {
			isoBox(screen, gx+drum*unitW*0.75, gy-drum*unitH*0.3, drum, 6*k,
				oilColor, mid(oilColor, oilDarkColor), oilDarkColor)
		}
	}
}

// mid blends two colors halfway, for a box face between light and shade.
func mid(a, b golib.Color) golib.Color {
	mix := func(x, y uint8) uint8 { return uint8((int(x) + int(y)) / 2) }
	return golib.Color{
		R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: a.A,
	}
}

// drawBuildings paints the colony's structures and the core's monolith
// among them, back to front, each an isometric body standing on its
// cell. Like the robots, they never shrink under an icon size on the
// screen: far out they are little marked blocks, and the view closes in
// on their true 40 u.
func drawBuildings(s *State, screen *golib.Screen, zoom float32) {
	type spot struct {
		id   int64
		y    float32
		core bool
	}
	spots := make([]spot, 0, len(s.Buildings)+1)
	for _, id := range sortedBuildingIDs(s) {
		b := s.Buildings[id]
		_, y := projectBuilding(b)
		spots = append(spots, spot{id: id, y: y})
	}
	_, coreY := projectCore()
	spots = append(spots, spot{y: coreY, core: true})
	sort.SliceStable(spots, func(i, j int) bool { return spots[i].y < spots[j].y })
	for _, sp := range spots {
		if sp.core {
			drawCore(s, screen, zoom)
			continue
		}
		b := s.Buildings[sp.id]
		gx, gy := projectBuilding(b)
		across, height := buildingSize(b.Kind)
		k := buildingIcon(across, height, zoom)
		fx, fy := gunFacing(gx, gy)
		drawBuilding(screen, b.Kind, gx, gy, across*k, height*k, fx, fy)
		if part, color, holds := buildingFill(s, b); holds {
			// Down the middle of the body's left face.
			x := gx - across*k*unitW/4
			foot := gy + across*k*unitH/4
			drawFillBar(screen, x, foot, height*k*unitH, zoom, part, color)
		}
		health := buildingHealth(b.Kind)
		drawHealthBar(screen, gx, gy, across*k, zoom, health-b.Damage, health, dangerColor)
	}
}

// buildingFill returns how full a building that stores something is, 0
// to 1, and the color of what it stores. Oil is in the building's own
// tank; lilac is one stock under every roof, so every warehouse reads
// the same.
func buildingFill(s *State, b Building) (part float32, color golib.Color, holds bool) {
	switch b.Kind {
	case BuildingSilo, BuildingCharger, BuildingProtector:
		return float32(b.Oil / tankCapOf(b.Kind)), oilColor, true
	case BuildingWarehouse:
		return float32(s.Stock.Lilac / lilacCap(s)), lilacColor, true
	}
	return 0, golib.Color{}, false
}

// drawFillBar paints how full a store is on its own body: a bar
// standing on foot, as tall as the face it is on, filling from the
// bottom. It keeps a width the eye can read while the view is far out.
func drawFillBar(
	screen *golib.Screen,
	x, foot, height, zoom, part float32,
	color golib.Color,
) {
	width := 2 * dotRadius(1.2, zoom, 1)
	edge := width / 4
	height -= 2 * edge
	foot -= edge
	screen.DrawRectangle(golib.Rectangle{
		X: x - width/2 - edge, Y: foot - height - edge,
		Width: width + 2*edge, Height: height + 2*edge,
	}, fillBarEdgeColor)
	screen.DrawRectangle(golib.Rectangle{
		X: x - width/2, Y: foot - height, Width: width, Height: height,
	}, fillBarColor)
	filled := height * golib.Clamp(part, 0, 1)
	screen.DrawRectangle(golib.Rectangle{
		X: x - width/2, Y: foot - filled, Width: width, Height: filled,
	}, color)
}

// buildingIcon returns the factor that lifts a body's true size until it
// clears the icon minimum on the screen — the same law the robots obey —
// easing back to one as the view closes in.
func buildingIcon(across, height, zoom float32) float32 {
	const (
		iconW = 9.0 // the smallest a building reads on the screen
		iconH = 6.0 //
	)
	k := float32(1)
	if w := across * unitW; w < iconW/zoom {
		k = iconW / zoom / w
	}
	if h := height * unitH; h*k < iconH/zoom {
		k = iconH / zoom / h
	}
	return k
}

// projectBuilding returns where a building's ground center lands on the
// screen.
func projectBuilding(b Building) (gx, gy float32) {
	x, y := cellCenterUnits(b.Col, b.Row)
	return project(float32(x), float32(y))
}

// drawJobs paints the build sites: the part already built rises from the
// ground in solid colors, wrapped in the wireframe of the whole body, so
// the building grows bottom-up inside its scaffold. The work's progress
// bar sits under the cell. It runs after the fog, so a site in the mist
// stays visible — the colony's work shows through.
func drawJobs(s *State, screen *golib.Screen, zoom float32) {
	for _, job := range s.Jobs {
		gx, gy := projectBuilding(Building{Col: job.Col, Row: job.Row})
		info := catalogInfo(buildingType(job.Kind))
		// The cell's ground diamond, in the kind's color.
		screen.DrawPolygonOutline(scaledDiamond(gx, gy, buildingCell/unitsPerTile),
			1.5/zoom, info.Color)
		across, height := buildingSize(job.Kind)
		k := buildingIcon(across, height, zoom)
		across, height = across*k, height*k
		// The built part, from the ground up.
		done := golib.Clamp(1-float32(job.Left)/float32(buildingWorkTicks), 0, 1)
		if done > 0 {
			fx, fy := gunFacing(gx, gy)
			drawBuilding(screen, job.Kind, gx, gy,
				across, height*done, fx, fy)
		}
		// The scaffold: the whole body, in wireframe.
		drawBuildingWireframe(screen, gx, gy, across, height, zoom,
			info.Color)
		// The progress bar: the site's width, filling with the work done.
		w := across * unitW
		hh := across * unitH / 2
		y := gy + hh + 4/zoom
		screen.DrawRectangle(
			golib.Rectangle{X: gx - w/2, Y: y, Width: w, Height: 3 / zoom}, scarColor)
		if done > 0 {
			screen.DrawRectangle(
				golib.Rectangle{X: gx - w/2, Y: y, Width: w * done, Height: 3 / zoom},
				info.Color)
		}
	}
}

func drawBuildingWireframe(
	screen *golib.Screen,
	gx, gy, across, height, zoom float32,
	color golib.Color,
) {
	hw := across * unitW / 2
	hh := across * unitH / 2
	hy := height * unitH
	wire := func(points []golib.Vector2) {
		screen.DrawPolygonOutline(points, 1.5/zoom, color)
	}
	wire([]golib.Vector2{
		{X: gx, Y: gy - hy - hh}, {X: gx + hw, Y: gy - hy},
		{X: gx, Y: gy - hy + hh}, {X: gx - hw, Y: gy - hy},
	})
	wire([]golib.Vector2{
		{X: gx, Y: gy - hy + hh}, {X: gx + hw, Y: gy - hy},
		{X: gx + hw, Y: gy}, {X: gx, Y: gy + hh},
	})
	wire([]golib.Vector2{
		{X: gx - hw, Y: gy - hy}, {X: gx, Y: gy - hy + hh},
		{X: gx, Y: gy + hh}, {X: gx - hw, Y: gy},
	})
}

func drawTechPlacementGhost(s *playScene, screen *golib.Screen) {
	col, row, inside := s.techPlacementCell()
	if !inside {
		return
	}
	valid := techPlacementValid(s.state, s.techPlacing, col, row)
	alpha := float32(0.32)
	if valid {
		alpha = 0.82
	}
	ink := golib.WithOpacity(techGhostColor, alpha)
	cell, gx, gy := cellDiamond(col, row, s.zoom)
	screen.DrawPolygon(cell, golib.WithOpacity(techGhostColor, alpha*0.12))
	screen.DrawPolygonOutline(cell, 2/s.zoom, ink)
	screen.DrawCircle(gx, gy, 2.5/s.zoom, ink)
	across, height := buildingSize(s.techPlacing)
	k := buildingIcon(across, height, s.zoom)
	drawBuildingWireframe(screen, gx, gy, across*k, height*k, s.zoom, ink)
}

// buildingSize returns a kind's body: its footprint across and its
// height, both in world units. Every footprint fits the 25 u cell.
func buildingSize(kind BuildingKind) (across, height float32) {
	switch kind {
	case BuildingFactory:
		return 24, 14
	case BuildingCharger:
		return 16, 12
	case BuildingSilo:
		return 22, 22
	case BuildingWarehouse:
		return 25, 13
	case BuildingProtector:
		return 8, 24
	case BuildingPump:
		return 18, 20
	case BuildingGuard:
		return 14, 18
	case BuildingWarFactory:
		return 24, 12
	case BuildingArtillery:
		return 20, 10
	}
	return 20, 10
}

// gunFacing returns the unit vector a gun points along out in the region:
// away from the core, which is where the rivals come from.
func gunFacing(gx, gy float32) (fx, fy float32) {
	cx, cy := projectCore()
	dx, dy := gx-cx, gy-cy
	gap := float32(math.Hypot(float64(dx), float64(dy)))
	if gap <= 0 {
		return 0, -1
	}
	return dx / gap, dy / gap
}

// drawBuilding paints a kind's body standing on the ground point gx, gy,
// across units on a side and height units tall. fx, fy is the unit vector
// its gun points along, for the kinds that carry one. This is the only
// place a building's look lives: the world calls it at the world's scale
// and the build menu in miniature, so a change to one is a change to
// both.
func drawBuilding(
	screen *golib.Screen,
	kind BuildingKind,
	gx, gy, across, height, fx, fy float32,
) {
	switch kind {
	case BuildingFactory:
		isoBox(screen, gx, gy, across, height,
			factoryColor, mid(factoryColor, factoryDark), factoryDark)
		// The chimney, a narrow stack at the works' corner.
		isoBox(screen, gx+across*unitW*0.22, gy-height*unitH*0.22,
			across*0.22, height*1.6, factoryColor, factoryDark, factoryDark)
	case BuildingCharger:
		isoBox(screen, gx, gy, across, height*0.6,
			chargerColor, mid(chargerColor, chargerDark), chargerDark)
		glow := across * unitW * 0.3
		screen.DrawCircle(gx, gy-height*unitH*0.8, glow, chargerColor)
		screen.DrawCircle(gx, gy-height*unitH*0.8, glow*0.5, oilColor)
	case BuildingSilo:
		// Three stacked tiers, each narrower: a silo.
		tier := across
		for i := 0; i < 3; i++ {
			h := height / 3
			isoBox(screen, gx, gy-float32(i)*h*unitH, tier, h,
				siloColor, mid(siloColor, siloDark), siloDark)
			tier *= 0.72
		}
	case BuildingWarehouse:
		isoBox(screen, gx, gy, across, height*0.75,
			warehouseColor, mid(warehouseColor, warehouseDark), warehouseDark)
		isoBox(screen, gx, gy-height*0.75*unitH, across*0.45, height*0.35,
			warehouseColor, warehouseDark, warehouseDark)
	case BuildingProtector:
		isoBox(screen, gx, gy, across, height,
			protectorColor, mid(protectorColor, protectorDark), protectorDark)
		glow := across * unitW * 0.7 // world-sized, like the core's
		screen.DrawCircle(gx, gy-(height+2)*unitH, glow, protectorColor)
		screen.DrawCircle(gx, gy-(height+2)*unitH, glow*0.5, bubbleEdgeColor)
	case BuildingPump:
		// A squat wellhead, a riser out of its middle and a cap of oil.
		isoBox(screen, gx, gy, across, height*0.4,
			pumpColor, mid(pumpColor, pumpDark), pumpDark)
		isoBox(screen, gx, gy-height*0.4*unitH, across*0.4, height*0.6,
			pumpColor, mid(pumpColor, pumpDark), pumpDark)
		screen.DrawCircle(gx, gy-(height+1)*unitH, across*unitW*0.22, oilColor)
	case BuildingArtillery:
		// A round pit of sandbags and a long barrel, up and away from the core.
		isoBox(screen, gx, gy, across, height*0.35,
			warFactoryColor, mid(warFactoryColor, warFactoryDark), warFactoryDark)
		isoBox(screen, gx, gy-height*0.35*unitH, across*0.4, height*0.4,
			guardColor, mid(guardColor, guardDark), guardDark)
		reach := across * unitW * 0.9
		screen.DrawLine(gx, gy-height*0.8*unitH, gx+fx*reach,
			gy+fy*reach-height*1.9*unitH, across*unitW*0.09, guardDark)
	case BuildingWarFactory:
		// A low hangar, a watch tower at its corner and the guard's lamp.
		isoBox(screen, gx, gy, across, height*0.7,
			warFactoryColor, mid(warFactoryColor, warFactoryDark), warFactoryDark)
		isoBox(screen, gx-across*unitW*0.22, gy-height*unitH*0.1,
			across*0.24, height*1.5, warFactoryColor, warFactoryDark, warFactoryDark)
		screen.DrawCircle(gx-across*unitW*0.22, gy-(height*1.6+1)*unitH,
			across*unitW*0.08, enemyLampColor)
	case BuildingGuard:
		// A bunker, a narrow tower on it and a lamp that watches.
		isoBox(screen, gx, gy, across, height*0.45,
			guardColor, mid(guardColor, guardDark), guardDark)
		isoBox(screen, gx, gy-height*0.45*unitH, across*0.4, height*0.55,
			guardColor, mid(guardColor, guardDark), guardDark)
		screen.DrawCircle(gx, gy-(height+1)*unitH, across*unitW*0.16, enemyLampColor)
	}
}

// isoBox draws an isometric box of a square footprint, across units on a
// side and height units tall, standing on the ground point gx, gy: a lit
// top, a side in shade and one halfway between.
func isoBox(
	screen *golib.Screen,
	gx, gy, across, height float32,
	top, right, left golib.Color,
) {
	isoSlab(screen, gx, gy, across, across, height, top, right, left)
}

// isoSlab is isoBox for a footprint that isn't square: alongX units on
// the world's x axis, which runs down and to the right of the screen, and
// alongY on its y axis, down and to the left. The right face is alongY
// wide and the left one alongX.
func isoSlab(
	screen *golib.Screen,
	gx, gy, alongX, alongY, height float32,
	top, right, left golib.Color,
) {
	sum := (alongX + alongY) / 2
	diff := (alongX - alongY) / 2
	hy := height * unitH
	back := golib.Vector2{X: gx - diff*unitW/2, Y: gy - sum*unitH/2}
	east := golib.Vector2{X: gx + sum*unitW/2, Y: gy + diff*unitH/2}
	front := golib.Vector2{X: gx + diff*unitW/2, Y: gy + sum*unitH/2}
	west := golib.Vector2{X: gx - sum*unitW/2, Y: gy - diff*unitH/2}
	lift := func(p golib.Vector2) golib.Vector2 {
		return golib.Vector2{X: p.X, Y: p.Y - hy}
	}
	screen.DrawPolygon([]golib.Vector2{
		lift(back), lift(east), lift(front), lift(west),
	}, top)
	screen.DrawPolygon([]golib.Vector2{
		lift(front), lift(east), east, front,
	}, right)
	screen.DrawPolygon([]golib.Vector2{
		lift(west), lift(front), front, west,
	}, left)
}

// projectCore returns where the middle of the core's tile lands on the
// screen: the monolith's foot, and the center of its bubble.
func projectCore() (cx, cy float32) {
	cx, cy = projectTile(coreCol, coreRow)
	return cx, cy + tileH/2
}

// drawCore paints the core: a dark monolith on its tile's middle, its
// broad face to the right, under a top that shines the core's warm white
// - the one part bright enough for the monitor's glow. It obeys the
// buildings' icon law, so far out it is a small lit pillar.
func drawCore(s *State, screen *golib.Screen, zoom float32) {
	cx, cy := projectCore()
	k := buildingIcon((coreSlabWide+coreSlabDeep)/2, coreHeight, zoom)
	deep, wide, height := coreSlabDeep*k, coreSlabWide*k, coreHeight*k
	isoSlab(screen, cx, cy, deep, wide, height,
		coreGlowColor, coreFaceColor, coreShadeColor)
	// A seam of light down the broad face, a fifth of the way in.
	along := wide/2 - wide*0.2
	sx := cx + (deep/2-along)*unitW/2
	sy := cy + (deep/2+along)*unitH/2
	screen.DrawLine(sx, sy-height*0.12*unitH, sx, sy-height*0.88*unitH,
		dotRadius(0.5, zoom, 1), coreGlowColor)
	// The core's own stores, on the far half of the same face.
	bars := []struct {
		along float32
		part  float64
		color golib.Color
	}{
		{-wide * 0.08, s.Stock.Oil / coreOilCap, oilColor},
		{-wide * 0.3, s.Stock.Lilac / lilacCap(s), lilacColor},
	}
	for _, bar := range bars {
		x := cx + (deep/2-bar.along)*unitW/2
		foot := cy + (deep/2+bar.along)*unitH/2 - height*0.12*unitH
		drawFillBar(screen, x, foot, height*0.76*unitH, zoom,
			float32(bar.part), bar.color)
	}
}

// protectorBubbleCenter returns where a protector's bubble sits on the
// screen: the ground center of the cell it was raised on - cells, not
// tiles, the same law the body obeys.
func protectorBubbleCenter(b Building) (cx, cy float32) {
	x, y := cellCenterUnits(b.Col, b.Row)
	return project(float32(x), float32(y))
}

// drawBubbles outlines the outside edge of the core's and protectors'
// joined clear ground. Their overlapping inner edges are left unpainted.
func drawBubbles(s *State, screen *golib.Screen, zoom float32) {
	discs := colonyClearDiscs(s)
	for i := range discs {
		color, thickness := bubbleEdgeColor, 3/zoom
		if i > 0 {
			color, thickness = protectorEdgeColor, 2/zoom
		}
		drawDiscOutline(screen, discs, i, thickness, color)
	}
}

// drawCorePad paints the core's whole tile as its pad, on the ground, so
// whatever crosses it walks over it.
func drawCorePad(screen *golib.Screen) {
	cx, cy := projectCore()
	screen.DrawPolygon(scaledDiamond(cx, cy, 1), coreColor)
}

// fogCover returns how much fog sits on a tile, from 0 to 1: none inside
// the line, then fading in across fogFadeTiles until the tile is fog, one with
// the fog around the region. Fading per tile keeps the front hugging the
// tiles, with no gaps between the tiles and a smooth band.
func fogCover(distance, line float32) float32 {
	start := line - 0.7
	return golib.Clamp((distance-start)/fogFadeTiles, 0, 1)
}

// tileDiamond returns the four corners of a tile's top face, from its
// projected top corner.
func tileDiamond(x, y float32) []golib.Vector2 {
	return []golib.Vector2{
		{X: x, Y: y},
		{X: x + tileW/2, Y: y + tileH/2},
		{X: x, Y: y + tileH},
		{X: x - tileW/2, Y: y + tileH/2},
	}
}

// scaledDiamond returns a tile-shaped diamond, shrunk around a center.
func scaledDiamond(cx, cy, scale float32) []golib.Vector2 {
	w := tileW / 2 * scale
	h := tileH / 2 * scale
	return []golib.Vector2{
		{X: cx, Y: cy - h},
		{X: cx + w, Y: cy},
		{X: cx, Y: cy + h},
		{X: cx - w, Y: cy},
	}
}

// drawIdleCount writes how many robots rest by the core, in screen
// pixels over their ranks: when they are more than the ranks show, and
// while the view is so far out that the ranks are one dot.
func drawIdleCount(s *State, screen *golib.Screen, camera *golib.Camera) {
	idle := idleRobots(s)
	merged := parkSpacing*unitW*camera.Zoom < 6
	if idle <= parkSlots && (idle < 2 || !merged) {
		return
	}
	x, y := parkCenter()
	gx, gy := project(float32(x), float32(y))
	at := camera.ToScreen(golib.Vector2{X: gx, Y: gy})
	reach := float32(parkSlots/parkRankSize) * parkSpacing * unitH * camera.Zoom
	words := fmt.Sprintf("%d idle", idle)
	width := screen.TextWidth(words, textSize, uiText)
	top := at.Y + reach + 8
	screen.DrawRectangle(golib.Rectangle{
		X: at.X - width/2 - 5, Y: top - 3, Width: width + 10, Height: textRow + 3,
	}, panelColor)
	screen.DrawText(words, at.X, top, textSize, panelTextColor,
		golib.TextOptions{Font: uiFont, Align: golib.AlignCenter})
}

// drawFogLine paints the fog's front: pressed in and in a stronger hand
// while a swell is up, and - when the forecast names it - a ghost of the
// line standing where the fog will press in.
func drawFogLine(screen *golib.Screen, zoom float32, s *State) {
	cx, cy := projectCore()
	if p := float32(s.Fog.Pressure); p > 0 {
		ellipseOutline(screen, cx, cy, fogLineNow(s), (3+p)/zoom,
			golib.WithOpacity(fogBandColor, golib.Lerp(0.55, 0.9, p)))
		return
	}
	if s.Fog.NextIn <= 1 {
		ellipseOutline(screen, cx, cy, fogLineRadius-swellReach(s), 2/zoom,
			golib.WithOpacity(fogBandColor, 0.22))
	}
	ellipseOutline(screen, cx, cy, fogLineRadius, 3/zoom,
		golib.WithOpacity(fogBandColor, 0.55))
}

// ellipsePoints returns the corners of a world circle of a radius in tiles
// around a screen point, flattened by the projection.
func ellipsePoints(cx, cy, radius float32) []golib.Vector2 {
	const points = 192
	halfW, halfH := ellipseSemiAxes(radius)
	shape := make([]golib.Vector2, points)
	for i := range shape {
		angle := float64(i) / points * 2 * math.Pi
		shape[i] = golib.Vector2{
			X: cx + halfW*float32(math.Cos(angle)),
			Y: cy + halfH*float32(math.Sin(angle)),
		}
	}
	return shape
}

func ellipseOutline(
	screen *golib.Screen,
	cx, cy, radius, thickness float32,
	color golib.Color,
) {
	screen.DrawPolygonOutline(ellipsePoints(cx, cy, radius), thickness, color)
}
