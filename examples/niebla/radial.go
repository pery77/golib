package main

import (
	"math"
	"strings"

	"golib"
)

// The build menu: a radial of blueprints around the clicked cell. It is
// view, not state: the scene remembers the cell it opened on, and the
// options lay out around the cell's projected center every frame, so the
// menu follows the view while it pans or zooms. Picking a blueprint
// raises it right on the cell - the click that opened the menu already
// chose where.
//
// The menu is two rings deep. The first offers the build groups, the
// second the blueprints of the group picked; a right click goes back a
// ring and closes the menu from the first.
//
// The rings hold what could rise on the cell: what the schematics, the
// ground or the fog refuse is not on them. The stores are the one
// referee that takes nothing off the rings: a blueprint they can't pay
// stands washed out to gray, refuses the click, and the tip beside it
// names it, prices it and boxes in red each resource that falls short.

// buildGroup names a family of blueprints the menu groups together.
type buildGroup string

const (
	groupIndustry  buildGroup = "industry"
	groupMilitary  buildGroup = "military"
	groupLogistics buildGroup = "logistics"
)

// buildGroups is the first ring, in the order it sits around the cell.
var buildGroups = []buildGroup{
	groupIndustry,
	groupMilitary,
	groupLogistics,
}

// groupMembers is the second ring of each group, in the order its
// blueprints sit around the cell. The pump is not here: it rises from an
// oil pool's card, never from the menu.
var groupMembers = map[buildGroup][]BuildingKind{
	groupIndustry: {BuildingFactory, BuildingWarFactory},
	groupMilitary: {BuildingGuard, BuildingArtillery},
	groupLogistics: {
		BuildingCharger, BuildingSilo, BuildingWarehouse, BuildingProtector,
	},
}

// groupColor is the ink a group's ring and glyph read in.
func groupColor(group buildGroup) golib.Color {
	switch group {
	case groupIndustry:
		return factoryColor
	case groupMilitary:
		return guardColor
	case groupLogistics:
		return warehouseColor
	}
	return panelTextColor
}

// Radial tuning, in screen pixels.
const (
	radialRadius = 62   // from the cell's center to an option's center
	radialItemR  = 15   // an option circle's radius
	washedVeil   = 0.55 // the panel's dark lying over a washed option's icon
)

// radialCenter is where the menu's cell lands on the screen.
func radialCenter(s *playScene) golib.Vector2 {
	cx, cy := projectBuilding(Building{Col: s.radialCol, Row: s.radialRow})
	center := s.camera.ToScreen(golib.Vector2{X: cx, Y: cy})
	margin := float32(radialRadius + radialItemR + 24)
	center.X = clampf(center.X, margin, float32(screenWidth)-margin)
	center.Y = clampf(center.Y, margin, float32(screenHeight)-margin)
	return center
}

// radialSpot is where the i-th of n options sits on the ring around a
// center, the first at the top and the rest clockwise.
func radialSpot(n, i int, cx, cy float32) (x, y float32) {
	angle := -math.Pi/2 + float64(i)*2*math.Pi/float64(n)
	return cx + radialRadius*float32(math.Cos(angle)),
		cy + radialRadius*float32(math.Sin(angle))
}

// radialHit reports whether the pointer is on an option's circle.
func radialHit(x, y, mx, my float32) bool {
	d := math.Hypot(float64(mx-x), float64(my-y))
	return d <= radialItemR+4
}

// radialGroupItem is one group on the first ring.
type radialGroupItem struct {
	group  buildGroup
	afford bool // some blueprint it holds could be paid right now
	x, y   float32
}

// radialLeafItem is one blueprint on the second ring.
type radialLeafItem struct {
	kind   BuildingKind
	afford bool // the stores could pay it right now
	x, y   float32
}

// radialOffered reports whether a blueprint belongs on the menu's rings
// at all: its schematics have arrived and the cell could take it. The
// stores don't answer here - what they can't pay stands washed out (see
// radialLeafItem.afford), so the player sees the building exists and
// what it costs instead of reading its absence as a missing drop.
func radialOffered(s *playScene, kind BuildingKind) bool {
	return kindUnlocked(s.state, kind) &&
		canPlace(s.state, kind, s.radialCol, s.radialRow)
}

// radialGroupLayout lays the first ring out around the menu's cell:
// every group that holds a blueprint the colony could raise here. The
// group reads washed out while nothing in it could be paid.
func radialGroupLayout(s *playScene) []radialGroupItem {
	center := radialCenter(s)
	groups := make([]buildGroup, 0, len(buildGroups))
	for _, group := range buildGroups {
		for _, kind := range groupMembers[group] {
			if radialOffered(s, kind) {
				groups = append(groups, group)
				break
			}
		}
	}
	items := make([]radialGroupItem, 0, len(groups))
	for i, group := range groups {
		x, y := radialSpot(len(groups), i, center.X, center.Y)
		items = append(items, radialGroupItem{
			group:  group,
			afford: groupAfford(s, group),
			x:      x, y: y,
		})
	}
	return items
}

// groupAfford reports whether any blueprint of the group could be paid
// on the menu's cell right now.
func groupAfford(s *playScene, group buildGroup) bool {
	for _, kind := range groupMembers[group] {
		if radialOffered(s, kind) && canAfford(s.state, kind) {
			return true
		}
	}
	return false
}

// radialLeafLayout lays the picked group's blueprints out around the
// menu's cell, each carrying whether the stores could pay it.
func radialLeafLayout(s *playScene) []radialLeafItem {
	center := radialCenter(s)
	kinds := make([]BuildingKind, 0, len(groupMembers[s.radialGroup]))
	for _, kind := range groupMembers[s.radialGroup] {
		if radialOffered(s, kind) {
			kinds = append(kinds, kind)
		}
	}
	items := make([]radialLeafItem, 0, len(kinds))
	for i, kind := range kinds {
		x, y := radialSpot(len(kinds), i, center.X, center.Y)
		items = append(items, radialLeafItem{
			kind:   kind,
			afford: canAfford(s.state, kind),
			x:      x, y: y,
		})
	}
	return items
}

func radialGroupHover(
	items []radialGroupItem,
	mx, my float32,
) (radialGroupItem, bool) {
	for _, item := range items {
		if radialHit(item.x, item.y, mx, my) {
			return item, true
		}
	}
	return radialGroupItem{}, false
}

func radialLeafHover(
	items []radialLeafItem,
	mx, my float32,
) (radialLeafItem, bool) {
	for _, item := range items {
		if radialHit(item.x, item.y, mx, my) {
			return item, true
		}
	}
	return radialLeafItem{}, false
}

// pickRadial acts on a click while the menu stands open. A group opens
// its ring - the ring holds everything the cell could take, washed out
// or not, so any group on it opens. A blueprint is marked on the cell,
// unless the stores can't pay it: then the click refuses, a dull click,
// and the menu stands. A click anywhere else puts the menu away.
func (s *playScene) pickRadial(mx, my float32) {
	if s.radialLevel == 0 {
		item, hit := radialGroupHover(radialGroupLayout(s), mx, my)
		if !hit {
			s.closeRadial()
			return
		}
		s.radialGroup = item.group
		s.radialLevel = 1
		s.au.ui(1)
		return
	}
	item, hit := radialLeafHover(radialLeafLayout(s), mx, my)
	if !hit {
		s.closeRadial()
		return
	}
	if !item.afford {
		s.au.ui(0.6)
		return
	}
	before := len(s.state.Jobs)
	Apply(s.state, MarkBuilding{
		Kind: item.kind, Col: s.radialCol, Row: s.radialRow,
	})
	if len(s.state.Jobs) > before {
		s.au.placed()
	}
	s.closeRadial()
}

// openRadial opens the build menu on a cell, on its first ring.
func (s *playScene) openRadial(col, row int) {
	s.radial = true
	s.radialLevel = 0
	s.radialCol, s.radialRow = col, row
	s.au.ui(1.05)
}

// closeRadial puts the build menu away.
func (s *playScene) closeRadial() {
	s.radial = false
	s.radialLevel = 0
}

// backRadial takes one ring back, and closes the menu from its first.
func (s *playScene) backRadial() {
	if s.radialLevel > 0 {
		s.radialLevel = 0
		return
	}
	s.closeRadial()
}

// drawRadial paints the open menu: a marker on the cell's center and one
// circle per option of the ring that stands open, ringed in its color
// and labeled under it, the option under the pointer lit. Every option
// on the rings is one the cell could take; what the stores can't pay
// stands washed out. A blueprint's circle carries its body in miniature;
// a group's carries its mark, and the group picked stands on the cell so
// the player knows which ring they are in. The blueprint under the
// pointer carries its tip.
func drawRadial(s *playScene, screen *golib.Screen, mx, my float32) {
	center := radialCenter(s)
	if s.radialLevel == 0 {
		screen.DrawCircle(center.X, center.Y, 3, pickedTileColor)
		drawRadialGroups(s, screen, center, mx, my)
		return
	}
	drawGroupGlyph(screen, s.radialGroup, center.X, center.Y, 18,
		groupColor(s.radialGroup))
	drawRadialLeaves(s, screen, center, mx, my)
}

// washed turns an ink into the gray of an option the stores can't pay:
// still the option's own color at heart, but reading as out of reach.
func washed(c golib.Color) golib.Color {
	return mid(mid(c, panelDimColor), panelDimColor)
}

func drawRadialGroups(
	s *playScene,
	screen *golib.Screen,
	center golib.Vector2,
	mx, my float32,
) {
	items := radialGroupLayout(s)
	hovered, over := radialGroupHover(items, mx, my)
	for _, item := range items {
		ink := groupColor(item.group)
		if !item.afford {
			ink = washed(ink)
		}
		lit := over && item.group == hovered.group
		fill, edge, label := panelColor, ink, ink
		if lit {
			fill = mid(panelColor, ink)
			if item.afford {
				edge, label = panelTextColor, panelTextColor
			}
		}
		screen.DrawCircle(item.x, item.y, radialItemR, fill)
		screen.DrawCircleOutline(item.x, item.y, radialItemR, 1.5, edge)
		drawGroupGlyph(screen, item.group, item.x, item.y,
			radialItemR*1.6, label)
		screen.DrawText(string(item.group), item.x, item.y+radialItemR+5,
			13, label,
			golib.TextOptions{Font: uiFont, Align: golib.AlignCenter})
	}
}

func drawRadialLeaves(
	s *playScene,
	screen *golib.Screen,
	center golib.Vector2,
	mx, my float32,
) {
	items := radialLeafLayout(s)
	hovered, over := radialLeafHover(items, mx, my)
	for _, item := range items {
		info := catalogInfo(buildingType(item.kind))
		ink := info.Color
		if !item.afford {
			ink = washed(ink)
		}
		lit := over && item.kind == hovered.kind
		fill, edge, label := panelColor, ink, ink
		if lit {
			fill = mid(panelColor, ink)
			if item.afford {
				edge, label = panelTextColor, panelTextColor
			}
		}
		screen.DrawCircle(item.x, item.y, radialItemR, fill)
		drawBlueprintIcon(screen, item.kind, item.x, item.y, radialItemR*1.5)
		// A veil of the panel's own dark draws the miniature body's
		// colors down with the rest: the option reads gray, not lit.
		if !item.afford {
			screen.DrawCircle(item.x, item.y, radialItemR,
				golib.WithOpacity(panelColor, washedVeil))
		}
		screen.DrawCircleOutline(item.x, item.y, radialItemR, 1.5, edge)
		screen.DrawText(string(item.kind), item.x, item.y+radialItemR+5,
			13, label,
			golib.TextOptions{Font: uiFont, Align: golib.AlignCenter})
	}
	if over {
		drawRadialTip(screen, hovered, radialTipOf(s.state, hovered.kind))
	}
}

// radialTip is what hovering a blueprint on the ring says: its name and
// its cost, each part marked when the stores fall short of it.
type radialTip struct {
	name  string
	color golib.Color
	parts []costPart
}

// shorts names what the stores lack, or "" while they pay everything.
func (t radialTip) shorts() string {
	var names []string
	for _, part := range t.parts {
		if part.missing {
			names = append(names, part.name)
		}
	}
	if names == nil {
		return ""
	}
	return "short of " + strings.Join(names, " + ")
}

// radialTipOf gathers a blueprint's tip. Pure: the draw measures and
// places the words on the screen.
func radialTipOf(s *State, kind BuildingKind) radialTip {
	info := catalogInfo(buildingType(kind))
	tip := radialTip{name: info.Name, color: info.Color}
	lilac, oil := buildingCost(kind)
	tip.parts = resourceCosts(s, lilac, oil)
	return tip
}

// drawRadialTip paints the tip beside the option under the pointer: a
// small plate naming the blueprint and pricing it, a red box around each
// resource's value that the stores fall short of, and a line saying so.
// It hangs to the option's right, and flips to its left near the screen's
// right edge.
func drawRadialTip(screen *golib.Screen, item radialLeafItem, tip radialTip) {
	const (
		pad    = 10 // air inside the plate
		row    = 17 // line height
		size   = 12 // text size
		offset = 10 // from the option's rim to the plate
	)
	const label = "costs "
	nameW := screen.TextWidth(tip.name, size, uiText)
	labelW := screen.TextWidth(label, size, uiText)
	partsW := costPartsWidth(screen, tip.parts, size)
	w := nameW
	if w < labelW+partsW {
		w = labelW + partsW
	}
	w += 2 * pad
	h := float32(2*row + 2*pad)
	if tip.shorts() != "" {
		h += row
	}
	x := item.x + radialItemR + offset
	if x+w > float32(screenWidth)-tooltipMargin {
		x = item.x - radialItemR - offset - w
	}
	x = clampf(x, tooltipMargin,
		float32(screenWidth)-tooltipMargin-w)
	y := clampf(item.y-h/2, tooltipMargin,
		float32(screenHeight)-tooltipMargin-h)
	plate := golib.Rectangle{X: x, Y: y, Width: w, Height: h}
	screen.DrawRectangle(plate, panelColor)
	screen.DrawRectangleOutline(plate, 1, panelEdgeColor)
	screen.DrawText(tip.name, x+pad, y+pad, size, tip.color, uiText)
	drawMarkup(screen, "[dim]"+label+"[/]", x+pad, y+pad+row, size,
		panelTextColor)
	cx := x + pad + labelW + 4
	cy := y + pad + row
	drawCostParts(screen, tip.parts, cx, cy, size)
	if tip.shorts() != "" {
		drawMarkup(screen, "[danger]"+tip.shorts()+"[/]",
			x+pad, y+pad+2*row, size, panelTextColor)
	}
}
