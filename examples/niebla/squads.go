package main

import (
	"fmt"
	"math"

	"golib"
)

// The squads on the screen: the number keys and the marks that call a
// squad, the pointer's mode that gives it its order, the strip that
// lists the squads by key, the mark of what each squad is at, and the
// shots that fly between troopers and rivals. The order itself goes
// through OrderSquad.

const (
	orderPickPx         = 16 // screen pixels around a vehicle that still pick it
	orderTimeoutTicks   = 20 * 60
	orderPreviewOpacity = 0.3

	squadKeys      = 9  // the number keys that call a squad: 1 through 9
	squadPickPx    = 14 // screen pixels around a squad's mark that pick it
	squadBoxWidth  = 50 // a squad box of the strip, screen pixels
	squadBoxHeight = 44 //
	squadBoxGap    = 6  //
)

func (s *playScene) armOrdering(home int64) {
	s.ordering = home
	s.orderTicks = orderTimeoutTicks
}

func squadThing(s *State, home int64) Thing {
	return Thing{
		Type:    TypeSquad,
		ID:      fmt.Sprintf("squad-%d", home),
		Ref:     home,
		Caption: squadWords(s, home),
	}
}

func (s *playScene) selectSquad(home int64) {
	building, stands := s.state.Buildings[home]
	if !stands || building.Kind != BuildingWarFactory ||
		len(squadMembers(s.state, home)) == 0 {
		return
	}
	s.picked = true
	s.pickedSquad = home
	s.pickedCol, s.pickedRow = building.Col, building.Row
	s.pickedThing = squadThing(s.state, home).ID
	s.pickedRobot = 0
	s.robotPage = 0
	s.armed = ""
	s.clearPickedUnit()
	s.closeRadial()
	s.au.ui(1)
}

func (s *playScene) squadMarkScreen(home int64) (golib.Vector2, bool) {
	if len(squadMembers(s.state, home)) == 0 {
		return golib.Vector2{}, false
	}
	sq := squadOf(s.state, home)
	pole := dotRadius(14, s.zoom, 10)
	gx, gy := project(float32(sq.X), float32(sq.Y))
	py := gy - pole/2
	if e, stands := s.state.Enemies[sq.Focus]; stands &&
		sq.Order == OrderAttack {
		gx, gy = project(float32(e.X), float32(e.Y))
		py = gy - 2*unitH
	}
	return s.camera.ToScreen(golib.Vector2{X: gx, Y: py}), true
}

func (s *playScene) squadPennantInCell(col, row int) bool {
	for _, id := range sortedBuildingIDs(s.state) {
		if s.state.Buildings[id].Kind != BuildingWarFactory ||
			len(squadMembers(s.state, id)) == 0 {
			continue
		}
		sq := squadOf(s.state, id)
		if _, hasTarget := s.state.Enemies[sq.Focus];
			hasTarget && sq.Order == OrderAttack {
			continue
		}
		x, y := project(float32(sq.X), float32(sq.Y))
		markCol, markRow, inside := cellAtWorld(float64(x), float64(y))
		if inside && markCol == col && markRow == row {
			return true
		}
	}
	return false
}

func (s *playScene) tickOrdering() {
	if s.ordering == 0 {
		return
	}
	s.orderTicks--
	if s.orderTicks <= 0 {
		s.ordering = 0
	}
}

func (s *playScene) orderSpot(mx, my float32) (float64, float64) {
	world := s.camera.ToWorld(mx, my)
	x, y := unitsAtWorld(float64(world.X), float64(world.Y))
	most := float64(regionCols*unitsPerTile) - 1
	return clamp64(x, 1, most), clamp64(y, 1, most)
}

// squadSlots lists the war factories whose squads the number keys call,
// oldest first: 1 calls the first war factory raised, 2 the next, and
// so on. A factory whose troopers are still on the way holds its slot
// all the same, so a squad's key never changes under it.
func squadSlots(s *State) []int64 {
	var slots []int64
	for _, id := range sortedBuildingIDs(s) {
		if s.Buildings[id].Kind == BuildingWarFactory {
			slots = append(slots, id)
		}
	}
	return slots
}

// enemyUnder returns the rival vehicle whose sprite is under the pointer.
// Stationary city buildings keep the older small target around their foot.
func (s *playScene) enemyUnder(mx, my float32) (Enemy, bool) {
	if hit, ok := s.unitAtScreen(mx, my, true); ok {
		return s.state.Enemies[hit.selection.id], true
	}
	var best Enemy
	found, bestGap := false, float64(orderPickPx)
	for _, id := range sortedEnemyIDs(s.state) {
		e := s.state.Enemies[id]
		if selectableEnemyVehicle(e.Kind) {
			continue
		}
		gx, gy := project(float32(e.X), float32(e.Y))
		at := s.camera.ToScreen(golib.Vector2{X: gx, Y: gy})
		if gap := math.Hypot(float64(at.X-mx), float64(at.Y-my)); gap <= bestGap {
			best, found, bestGap = e, true, gap
		}
	}
	return best, found
}

// squadMarkAt returns the squad whose mark stands under the pointer:
// the pennant on the spot it guards - its war factory's door, when
// nobody has ordered it - or the ring around the vehicle it is to
// shoot first, at the same points the marks are drawn. The nearest
// within squadPickPx.
func (s *playScene) squadMarkAt(mx, my float32) (int64, bool) {
	var best int64
	found, bestGap := false, float64(squadPickPx)
	for _, id := range sortedBuildingIDs(s.state) {
		if s.state.Buildings[id].Kind != BuildingWarFactory {
			continue
		}
		if len(squadMembers(s.state, id)) == 0 {
			continue
		}
		at, stands := s.squadMarkScreen(id)
		if !stands {
			continue
		}
		if gap := math.Hypot(float64(at.X-mx), float64(at.Y-my)); gap <= bestGap {
			best, found, bestGap = id, true, gap
		}
	}
	return best, found
}

func (s *playScene) pickSquadMark(mx, my float32) bool {
	home, ok := s.squadMarkAt(mx, my)
	if !ok {
		return false
	}
	sq := squadOf(s.state, home)
	if sq.Order == OrderGuard {
		gx, gy := project(float32(sq.X), float32(sq.Y))
		col, row, inside := cellAtWorld(float64(gx), float64(gy))
		if inside {
			if b, stands := buildingAt(s.state, col, row); stands {
				s.pickCellOrBuild(col, row, true)
				s.pickedThing = buildingThing(b).ID
				s.closeRadial()
				return true
			}
		}
	}
	s.selectSquad(home)
	return true
}

func (s *playScene) syncPickedSquad() {
	if s.pickedSquad == 0 {
		return
	}
	if _, stands := s.state.Buildings[s.pickedSquad]; stands &&
		len(squadMembers(s.state, s.pickedSquad)) > 0 {
		return
	}
	s.picked = false
	s.pickedSquad = 0
	s.pickedThing = ""
}

// updateOrdering is the pointer while it gives a squad its order: a left
// click on a rival vehicle sends the squad after its party, that vehicle
// first; a left click anywhere else on the region posts the squad there;
// a right click puts the pointer away. One click, one order.
func (s *playScene) updateOrdering(input *golib.Input, clickTaken, rightClick bool) {
	if _, stands := s.state.Buildings[s.ordering]; !stands || rightClick {
		s.ordering = 0
		return
	}
	if !input.MousePressed(golib.MouseLeft) || clickTaken {
		return
	}
	mx, my := input.MousePosition()
	order := OrderSquad{Squad: s.ordering}
	if e, ok := s.enemyUnder(mx, my); ok {
		order.Enemy = e.ID
	} else if !s.hoverCell {
		return
	}
	order.X, order.Y = s.orderSpot(mx, my)
	Apply(s.state, order)
	s.ordering = 0
}

func (s *playScene) drawOrderingPreview(screen *golib.Screen) {
	if !s.hoverCell {
		return
	}
	if _, over := s.enemyUnder(s.mouse.X, s.mouse.Y); over {
		return
	}
	x, y := s.orderSpot(s.mouse.X, s.mouse.Y)
	gx, gy := project(float32(x), float32(y))
	drawSquadPennant(screen, gx, gy, s.zoom,
		golib.WithOpacity(guardColor, orderPreviewOpacity))
}

// drawOrderingLabel writes by the pointer what a click would order.
func (s *playScene) drawOrderingLabel(screen *golib.Screen) {
	words := "guard here"
	if e, ok := s.enemyUnder(s.mouse.X, s.mouse.Y); ok {
		words = "attack, " + string(e.Kind) + " first"
	}
	width := screen.TextWidth(words, textSize, uiText)
	x, y := s.mouse.X+16, s.mouse.Y-textRow/2
	screen.DrawRectangle(golib.Rectangle{
		X: x - 6, Y: y - 4, Width: width + 12, Height: textRow + 6,
	}, panelColor)
	screen.DrawText(words, x, y, textSize, guardColor, uiText)
}

// drawSquadMarks paints what each squad is at, over the fog: a small
// pennant on the spot it guards, or a ring around the vehicle it is to
// shoot first.
func drawSquadMarks(s *State, screen *golib.Screen, zoom float32) {
	for _, id := range sortedBuildingIDs(s) {
		if s.Buildings[id].Kind != BuildingWarFactory {
			continue
		}
		if len(squadMembers(s, id)) == 0 {
			continue
		}
		sq := squadOf(s, id)
		if e, ok := s.Enemies[sq.Focus]; ok && sq.Order == OrderAttack {
			gx, gy := project(float32(e.X), float32(e.Y))
			r := dotRadius(14, zoom, 9)
			screen.DrawCircleOutline(gx, gy-2*unitH, r, 1.5/zoom, guardColor)
			continue
		}
		gx, gy := project(float32(sq.X), float32(sq.Y))
		drawSquadPennant(screen, gx, gy, zoom, guardColor)
	}
}

func drawSquadPennant(
	screen *golib.Screen, gx, gy, zoom float32, color golib.Color,
) {
	pole := dotRadius(14, zoom, 10)
	screen.DrawLine(gx, gy, gx, gy-pole, 1.5/zoom, color)
	screen.DrawTriangle(gx, gy-pole, gx, gy-pole*0.55,
		gx+pole*0.6, gy-pole*0.78, color)
}

// drawSquadStrip paints the squads' boxes at the top right, one per
// squad: a little tank icon, how many troopers the squad counts and,
// below, the key that calls it. The box of the squad being ordered is
// ringed in the squads' green, and so is the box under the pointer
// lit.
func drawSquadStrip(s *playScene, screen *golib.Screen) {
	for i, home := range squadSlots(s.state) {
		if i >= squadKeys {
			break
		}
		rect := squadBoxRect(i)
		armed := s.ordering == home
		fill := buttonColor
		if armed || rect.Contains(s.mouse.X, s.mouse.Y) {
			fill = buttonHoverColor
		}
		screen.DrawRectangle(rect, fill)
		edge := buttonEdgeColor
		if armed {
			edge = guardColor
		}
		screen.DrawRectangleOutline(rect, 1, edge)
		count := len(squadMembers(s.state, home))
		ink := panelTextColor
		tint := golib.White
		if count == 0 && !armed {
			tint, ink = golib.WithOpacity(panelDimColor, 0.5), panelDimColor
		}
		drawTrooperIcon(screen, rect.X+4, rect.Y+8, tint)
		words := fmt.Sprintf("%d", count)
		screen.DrawText(words, rect.X+squadBoxWidth-8-screen.TextWidth(words, textSize, uiText),
			rect.Y+8, textSize, ink, uiText)
		key := fmt.Sprintf("%d", i+1)
		keyColor := panelDimColor
		if armed {
			keyColor = guardColor
		}
		screen.DrawText(key, rect.X+squadBoxWidth/2-screen.TextWidth(key, 11, uiText)/2,
			rect.Y+28, 11, keyColor, uiText)
	}
}

// squadBoxRect is where a squad's box stands, right to left from the
// screen's top right corner: the squad the 1 calls is the rightmost.
func squadBoxRect(i int) golib.Rectangle {
	return golib.Rectangle{
		X:      float32(screenWidth-16) - float32(i+1)*squadBoxWidth - float32(i)*squadBoxGap,
		Y:      12,
		Width:  squadBoxWidth,
		Height: squadBoxHeight,
	}
}

// squadBoxAt returns the box under a screen point, if any.
func squadBoxAt(mx, my float32) (int, bool) {
	for i := 0; i < squadKeys; i++ {
		if squadBoxRect(i).Contains(mx, my) {
			return i, true
		}
	}
	return 0, false
}

func drawTrooperIcon(screen *golib.Screen, x, y float32, tint golib.Color) {
	center := golib.Vector2{X: x + 11, Y: y + 8}
	trooperModel.drawIcon(screen, center, facingUpRight, tint)
}

// squadWords says a squad in a few words: how many it is and what it is
// at.
func squadWords(s *State, home int64) string {
	count := len(squadMembers(s, home))
	if count == 0 {
		return "no troopers yet"
	}
	sq := squadOf(s, home)
	doing := "guarding"
	if sq.Order == OrderAttack {
		doing = "attacking"
		if e, ok := s.Enemies[sq.Focus]; ok {
			doing = fmt.Sprintf("attacking, %s first", e.Kind)
		}
	}
	return fmt.Sprintf("squad of %d, %s", count, doing)
}
