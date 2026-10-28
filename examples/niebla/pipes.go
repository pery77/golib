package main

import (
	"fmt"
	"math"

	"golib"
)

// Pipes on the screen, and the pointer's mode that lays them. All of it
// is view: the pipes' picture reads the state and its tick, and a pipe
// being drawn by the player lives in the scene until the click that ends
// it sends LayPipe.
const (
	pipeWidthUnits = 5.0 // u across, close up
	pipeMinPx      = 3.0 // and never thinner than this on the screen

	// A pipe runs above the ground on posts, a section apart, and casts
	// its shadow straight under it.
	pipeLiftUnits = 9.0  // u off the ground, close up
	pipeLiftPx    = 5.0  // and never lower than this on the screen
	pipePostPx    = 14.0 // posts never stand closer than this on the screen

	pipeShadowLean = 0.8 // how far left of the pipe its shadow falls, by its lift

	// The oil shows as bands running down the pipe, one every gap. Their
	// length says what the source offers; their speed follows actual flow.
	flowGapUnits      = 30.0 // u between two bands, close up
	flowGapPx         = 16.0 // and never closer than this on the screen
	flowBandMaxPart   = 0.9  // the gap's orange share at pump capacity
	flowBeatTicks     = 40
	flowLitersPerBeat = pumpLitersPerSecond * flowBeatTicks / 60.0

	layReachPx = 12.0 // how near a store's body a click ends a pipe
	layNodePx  = 10.0 // how near the pipe's last node a click opens its menu

	layMenuRadius = 46 // from the node to an option's center
	layMenuItemR  = 13 // an option circle's radius
)

// The options of the menu a click on the pipe's last node opens. Update
// reads them to know what to do, so treat them as identifiers.
const (
	layConnect = "connect"
	layUndo    = "undo"
	layCancel  = "cancel"
)

var layMenuOrder = []string{layConnect, layUndo, layCancel}

// pipeLaying is the pointer's mode while the player draws a pipe: the
// pump or the tank it leaves from and the bends clicked so far.
type pipeLaying struct {
	on    bool
	from  int64
	bends []PipePoint
	menu  bool // the last node's menu stands open
}

// pipeSegment is one straight piece of a pipe's curve, projected.
type pipeSegment struct {
	ax, ay, bx, by float32
}

func projectPoint(p PipePoint) (x, y float32) {
	return project(float32(p.X), float32(p.Y))
}

func pipeFlowPhase(moved float64) float64 {
	return math.Mod(moved, flowLitersPerBeat) / flowLitersPerBeat
}

func pipeFlowBandPart(offered float64) float64 {
	capacity := pumpLitersPerSecond / 60
	return flowBandMaxPart * math.Max(0, math.Min(1, offered/capacity))
}

// drawPipes paints the pipes, in two passes like the piles: the
// stretches under a bubble or on clear ground go under the buildings and
// the robots, and the ones in the mist over the fog, so the colony never
// loses sight of its things. A pipe runs above the ground: its shadow
// lies on the ground under it, where the mist doesn't hide it, posts
// hold it up a section apart, and the pipe itself is drawn lifted. The
	// sections still to be laid show as a faint line, over the fog too. Oil
	// runs down a pipe that carries it in bands sized by source offer and
	// placed by the oil that actually passed through it.
func drawPipes(s *State, screen *golib.Screen, zoom float32, sheltered bool) {
	width := 2 * dotRadius(pipeWidthUnits/2, zoom, pipeMinPx/2)
	lift := float32(math.Max(pipeLiftUnits*float64(unitH), float64(pipeLiftPx/zoom)))
	inPass := func(p PipePoint) bool {
		return (fogAt(s, p.X, p.Y) <= 0) == sheltered
	}
	for _, id := range sortedPipeIDs(s) {
		p := s.Pipes[id]
		path, ok := pipeSpine(s, p)
		if !ok {
			continue
		}
		length := pathLength(path)
		laidAt := func(along float64) bool {
			section := int64(math.Floor(along / pipeSectionMeters))
			inside := section >= 0 && section < p.Sections
			return inside && sectionLeft(p, section) == 0
		}
		var solid, ghost []pipeSegment
		at := 0.0
		for i := 1; i < len(path); i++ {
			a, b := path[i-1], path[i]
			start := at
			at += pointGap(a, b)
			var seg pipeSegment
			seg.ax, seg.ay = projectPoint(a)
			seg.bx, seg.by = projectPoint(b)
			if !laidAt((start + at) / 2) {
				ghost = append(ghost, seg)
			} else if inPass(lerpPoint(a, b, 0.5)) {
				solid = append(solid, seg)
			}
		}
		if sheltered {
			// The sun stands to the right, as the buildings' faces say.
			lean := lift * pipeShadowLean
			for _, seg := range solid {
				screen.DrawLine(seg.ax-lean, seg.ay, seg.bx-lean, seg.by, width,
					pipeShadowColor)
			}
		}
		stride := math.Ceil(float64(pipePostPx/zoom) / float64(pipeSectionMeters*unitW))
		for along := 0.0; along <= length; along += pipeSectionMeters * math.Max(1, stride) {
			post := pathPointAt(path, along)
			standing := laidAt(along) || laidAt(along-pipeSectionMeters/2)
			if !standing || !inPass(post) {
				continue
			}
			x, y := projectPoint(post)
			screen.DrawLine(x, y, x, y-lift, width*0.5, pipeDarkColor)
		}
		for _, seg := range solid {
			screen.DrawLine(seg.ax, seg.ay-lift, seg.bx, seg.by-lift, width, pipeDarkColor)
		}
		for _, seg := range solid {
			screen.DrawLine(seg.ax, seg.ay-lift, seg.bx, seg.by-lift, width*0.5, pipeColor)
		}
		if !sheltered {
			for _, seg := range ghost {
				screen.DrawLine(seg.ax, seg.ay-lift, seg.bx, seg.by-lift, width*0.5,
					golib.WithOpacity(siteColor, 0.6))
			}
		}
		if p.Flow <= 0 || p.Offered <= 0 {
			continue
		}
		gap := math.Max(flowGapUnits, float64(flowGapPx/zoom/unitW))
		beat := pipeFlowPhase(p.Moved)
		part := pipeFlowBandPart(p.Offered)
		for along := gap * beat; along < length; along += gap {
			if !inPass(pathPointAt(path, along)) {
				continue
			}
			half := gap * part / 2
			tailX, tailY := projectPoint(pathPointAt(path, along-half))
			x, y := projectPoint(pathPointAt(path, along))
			headX, headY := projectPoint(pathPointAt(path, along+half))
			screen.DrawLine(tailX, tailY-lift, x, y-lift, width*0.7, oilColor)
			screen.DrawLine(x, y-lift, headX, headY-lift, width*0.7, oilColor)
		}
	}
}

// startLaying arms the pointer to draw a pipe out of a pump or a tank.
func (s *playScene) startLaying(from int64) {
	s.laying = pipeLaying{on: true, from: from}
	s.picked = false
	s.pickedSquad = 0
	s.pickedThing = ""
	s.pickedRobot = 0
	s.clearPickedUnit()
	s.robotPage = 0
	s.closeRadial()
}

// pointerUnits returns the world units under the pointer.
func (s *playScene) pointerUnits() (x, y float64) {
	world := s.camera.ToWorld(s.mouse.X, s.mouse.Y)
	return unitsAtWorld(float64(world.X), float64(world.Y))
}

// layTargets lists the tanks the pipe in hand may end at.
func (s *playScene) layTargets() []int64 {
	var tanks []int64
	for _, tank := range allOilTanks(s.state) {
		if canJoin(s.state, s.laying.from, tank) {
			tanks = append(tanks, tank)
		}
	}
	return tanks
}

// layTarget returns the tank the pointer would end the pipe at: the
// nearest one within reach, measured on the screen against the tank's
// whole body, foot to top, since that is what the player aims at. The
// core is tall and its reach wide, so it only answers when no building
// does. With the last node's menu open the target is the tank nearest
// that node instead.
func (s *playScene) layTarget() (tank int64, found bool) {
	if s.laying.menu {
		return s.nearestTarget()
	}
	best, core := math.Inf(1), false
	for _, id := range s.layTargets() {
		spot, _ := tankSpot(s.state, id)
		foot := s.toScreen(spot)
		across, height := float32(coreSlabWide), float32(coreHeight)
		if id != coreTank {
			across, height = buildingSize(s.state.Buildings[id].Kind)
		}
		k := buildingIcon(across, height, s.zoom) * s.zoom
		top := foot.Y - height*k*unitH
		reach := math.Max(layReachPx, float64(across*k*unitW/2))
		nearY := golib.Clamp(s.mouse.Y, top, foot.Y)
		d := math.Hypot(float64(foot.X-s.mouse.X), float64(nearY-s.mouse.Y))
		if d > reach {
			continue
		}
		if id == coreTank {
			core = true
		} else if d < best {
			best, tank, found = d, id, true
		}
	}
	if !found && core {
		return coreTank, true
	}
	return tank, found
}

// nearestTarget returns the tank the pipe may end at that is closest to
// its last node, which is where the node's menu connects it.
func (s *playScene) nearestTarget() (tank int64, found bool) {
	node, best := s.lastNode(), math.Inf(1)
	for _, id := range s.layTargets() {
		spot, _ := tankSpot(s.state, id)
		if gap := pointGap(node, spot); gap < best {
			best, tank, found = gap, id, true
		}
	}
	return tank, found
}

func (s *playScene) toScreen(p PipePoint) golib.Vector2 {
	x, y := projectPoint(p)
	return s.camera.ToScreen(golib.Vector2{X: x, Y: y})
}

// lastNode returns where the pipe in hand stands now: its last bend, or
// its source while it has none.
func (s *playScene) lastNode() PipePoint {
	if n := len(s.laying.bends); n > 0 {
		return s.laying.bends[n-1]
	}
	from, _ := pipeEndSpot(s.state, s.laying.from)
	return from
}

// layMenuItem is one option of the last node's menu, laid out on the
// screen.
type layMenuItem struct {
	label string
	x, y  float32
	ready bool
}

// layMenuLayout lays the menu's options out around the last node:
// connect, which ends the pipe at the nearest tank that takes it, undo,
// which takes the node back, and cancel, which drops the whole pipe.
func (s *playScene) layMenuLayout() []layMenuItem {
	center := s.toScreen(s.lastNode())
	_, sections, _ := s.layingPreview()
	_, reachable := s.nearestTarget()
	items := make([]layMenuItem, 0, len(layMenuOrder))
	for i, label := range layMenuOrder {
		angle := -math.Pi/2 + float64(i)*2*math.Pi/float64(len(layMenuOrder))
		ready := true
		switch label {
		case layConnect:
			ready = reachable && s.state.Stock.Lilac >= pipeCost(sections)
		case layUndo:
			ready = len(s.laying.bends) > 0
		}
		items = append(items, layMenuItem{
			label: label,
			x:     center.X + layMenuRadius*float32(math.Cos(angle)),
			y:     center.Y + layMenuRadius*float32(math.Sin(angle)),
			ready: ready,
		})
	}
	return items
}

func layMenuHover(items []layMenuItem, mx, my float32) (layMenuItem, bool) {
	for _, item := range items {
		if d := math.Hypot(float64(mx-item.x), float64(my-item.y)); d <= layMenuItemR+4 {
			return item, true
		}
	}
	return layMenuItem{}, false
}

// pickLayMenu acts on a click while the menu stands open: an option that
// is ready does its thing, and a click anywhere else puts the menu away.
func (s *playScene) pickLayMenu() {
	item, hit := layMenuHover(s.layMenuLayout(), s.mouse.X, s.mouse.Y)
	if hit && !item.ready {
		return
	}
	s.laying.menu = false
	if !hit {
		return
	}
	switch item.label {
	case layConnect:
		if tank, found := s.nearestTarget(); found {
			s.sendPipe(tank)
		}
	case layUndo:
		s.laying.bends = s.laying.bends[:len(s.laying.bends)-1]
	case layCancel:
		s.laying = pipeLaying{}
	}
}

// sendPipe marks the pipe in hand to a tank. Stores that can't pay leave
// it in the player's hand.
func (s *playScene) sendPipe(tank int64) {
	before := len(s.state.Pipes)
	Apply(s.state, LayPipe{
		From: s.laying.from, To: tank, Bends: s.laying.bends,
	})
	if len(s.state.Pipes) > before {
		s.laying = pipeLaying{}
	}
}

// updateLaying is the pointer while it draws a pipe: a left click on a
// tank ends the pipe there and sends it, one on the pipe's last
// node opens that node's menu, one on the ground adds a bend, and a
// right click closes the menu, takes the last bend back, or puts the
// mode away when there is none.
func (s *playScene) updateLaying(input *golib.Input, clickTaken, rightClick bool) {
	if _, ok := pipeEndSpot(s.state, s.laying.from); !ok {
		s.laying = pipeLaying{}
		return
	}
	if rightClick {
		if s.laying.menu {
			s.laying.menu = false
		} else if n := len(s.laying.bends); n > 0 {
			s.laying.bends = s.laying.bends[:n-1]
		} else {
			s.laying = pipeLaying{}
		}
		return
	}
	if !input.MousePressed(golib.MouseLeft) || clickTaken {
		return
	}
	if s.laying.menu {
		s.pickLayMenu()
		return
	}
	if store, found := s.layTarget(); found {
		s.sendPipe(store)
		return
	}
	node := s.toScreen(s.lastNode())
	if math.Hypot(float64(node.X-s.mouse.X), float64(node.Y-s.mouse.Y)) <= layNodePx {
		s.laying.menu = true
		return
	}
	x, y := s.pointerUnits()
	if inRegion(x, y) && len(s.laying.bends) < pipeMaxBends {
		s.laying.bends = append(s.laying.bends, PipePoint{x, y})
	}
}

// layingPreview returns the curve the pipe in hand would lie along, the
// sections it would take and whether it ends on a tank.
func (s *playScene) layingPreview() (path []PipePoint, sections int64, ends bool) {
	from, _ := pipeEndSpot(s.state, s.laying.from)
	x, y := s.pointerUnits()
	to := PipePoint{x, y}
	tank, found := s.layTarget()
	if found {
		to, _ = tankSpot(s.state, tank)
	}
	path = pipePath(from, s.laying.bends, to)
	return path, pipeSections(pathLength(path)), found
}

// drawLaying paints the pipe in hand, in the world: the curve through
// the bends to the pointer, in the lilac it costs or in red when the
// stores can't pay, the bends as dots and a ring on the store it would
// end at.
func (s *playScene) drawLaying(screen *golib.Screen) {
	path, sections, ends := s.layingPreview()
	color := lilacLightColor
	if s.state.Stock.Lilac < pipeCost(sections) {
		color = dangerColor
	}
	for i := 1; i < len(path); i++ {
		ax, ay := projectPoint(path[i-1])
		bx, by := projectPoint(path[i])
		screen.DrawLine(ax, ay, bx, by, 2/s.zoom, color)
	}
	for _, bend := range s.laying.bends {
		x, y := projectPoint(bend)
		screen.DrawCircle(x, y, 3/s.zoom, color)
	}
	// Every tank the pipe may end at wears a ring, and the one it would
	// end at now a brighter one.
	for _, id := range s.layTargets() {
		spot, _ := tankSpot(s.state, id)
		x, y := projectPoint(spot)
		screen.DrawCircleOutline(x, y, 9/s.zoom, 1.5/s.zoom,
			golib.WithOpacity(lilacLightColor, 0.45))
	}
	if ends {
		x, y := projectPoint(path[len(path)-1])
		screen.DrawCircleOutline(x, y, 9/s.zoom, 2.5/s.zoom, color)
	}
}

// drawLayMenu paints the last node's menu, in screen pixels, the way the
// build menu reads: a circle per option, lit under the pointer, dimmed
// when it can't be picked.
func (s *playScene) drawLayMenu(screen *golib.Screen) {
	items := s.layMenuLayout()
	hovered, _ := layMenuHover(items, s.mouse.X, s.mouse.Y)
	for _, item := range items {
		edge := lilacLightColor
		if item.label == layCancel {
			edge = dangerColor
		}
		if !item.ready {
			edge = blockedColor
		}
		fill, ink := panelColor, edge
		if item.ready && item.label == hovered.label {
			fill, ink = edge, panelTextColor
		}
		screen.DrawCircle(item.x, item.y, layMenuItemR, fill)
		screen.DrawCircleOutline(item.x, item.y, layMenuItemR, 1.5, edge)
		screen.DrawText(item.label, item.x, item.y+layMenuItemR+4, 13, ink,
			golib.TextOptions{Font: uiFont, Align: golib.AlignCenter})
	}
}

// drawLayingLabel writes what the pipe in hand would cost, by the
// pointer, in screen pixels.
func (s *playScene) drawLayingLabel(screen *golib.Screen) {
	_, sections, _ := s.layingPreview()
	cost := fmt.Sprintf("[lilac]%s[/]", si(pipeCost(sections), "kg"))
	if s.state.Stock.Lilac < pipeCost(sections) {
		cost = fmt.Sprintf("[danger]%s[/]", si(pipeCost(sections), "kg"))
	}
	words := fmt.Sprintf("%s   %s", si(float64(sections)*pipeSectionMeters, "m"), cost)
	if tank, found := s.layTarget(); found {
		words += "   to " + pipeEndName(s.state, tank)
	}
	x, y := s.mouse.X+16, s.mouse.Y+14
	if s.laying.menu {
		node := s.toScreen(s.lastNode())
		x, y = node.X+layMenuRadius+24, node.Y-textRow/2
	}
	width := float32(0)
	for _, span := range parseMarkup(words, panelTextColor) {
		width += screen.TextWidth(span.text, textSize, uiText)
	}
	plate := golib.Rectangle{
		X: x - 6, Y: y - 4, Width: width + 12, Height: textRow + 6,
	}
	screen.DrawRectangle(plate, panelColor)
	drawMarkup(screen, words, x, y, textSize, panelTextColor)
}
