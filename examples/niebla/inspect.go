package main

import (
	"fmt"

	"golib"
)

// Tooltip tuning, in screen pixels. The panel is one dark plate with a row
// per line: the cell's header, one card title per thing with its headline
// at the right, the expanded cards' details and, for deposits, worker
// portraits and a button to send another robot. The card of a
// building or a site ends its title in a trash can, which on the second
// press orders the building taken down - a builder goes and works it
// down - or cancels the site at once. A building already ordered down
// wears no can; its headline says `demolishing`.
const (
	tooltipWidth     = 290
	tooltipPad       = 12
	titleSize        = 15
	textSize         = 13
	titleRow         = 24 // line height of a card's title
	textRow          = 17 // line height of the header and of a detail line
	buttonRow        = 22 // line height of a button, with air around it
	buttonWidth      = 112
	barWidth         = 3  // the color bar on a card's left
	detailLabelW     = 80 // where a detail's value starts, past its label
	detailIndent     = 12 // how far details sit inside their card
	tooltipGap       = 14 // from the cell's corner to the panel
	tooltipMargin    = 8  // kept between the panel and the screen's edges
	trashWidth       = 11 // the trash can at the end of a card's title
	trashHeight      = 13
	trashGap         = 8 // between the trash can and the title's headline
	portraitPageSize = 8
	portraitColumns  = 4
	portraitWidth    = 56
	portraitHeight   = 34
	portraitGap      = 4
)

// What a card's headline reads while its trash can is armed, and while
// the pointer is over one that can't go.
const (
	armedSummary   = "demolish?"
	blockedSummary = "its bubble shelters others"
)

// The labels a card's button carries. Update reads them to know which
// action to apply, so treat them as identifiers, not prose.
const (
	buttonSend          = "send robot"
	buttonRecall        = "recall robot"
	buttonBuildBuilder  = "build builder"
	buttonBuildWorker   = "build worker"
	buttonTrooper       = "build trooper"
	buttonMechanic      = "build mechanic"
	buttonOrder         = "give order"
	buttonBuildPump     = "build pump"
	buttonLayPipe       = "lay pipe"
	buttonRemovePipe    = "remove"
	buttonBackToDeposit = "back to deposit"
	buttonPortraitPrev  = "previous robots"
	buttonPortraitNext  = "next robots"
)

const pipeButtonWidth = 58 // a pipe row's remove button: its note needs the room

// tooltipRow is one line the panel draws: the header, a card's title, an
// expanded card's detail, or a button. Unlocked production options stay
// visible when disabled; other buttons appear only when they can work.
type tooltipRow struct {
	thing       Thing  // the card the row belongs to; empty on the header
	detail      Detail // on a detail row
	header      bool
	title       bool
	open        bool   // the card is expanded
	summary     string // the title's headline, right-aligned
	button      string // the label on a button row, "" otherwise
	note        string // markup written beside the button
	costs       []costPart
	ref         int64   // what the button acts on, when it isn't the card's thing: a pipe
	bx, by      float32 // the button's rectangle on the screen
	bw, bh      float32 //
	trash       bool    // a title row that ends in a trash can, at bx, by
	blocked     bool    // the trash can is dimmed: this one can't go
	disabled    bool    // a visible button that cannot be pressed now
	armed       bool    // the trash can was pressed once and asks again
	workers     bool
	portraits   []robotPortrait
	page        int
	pages       int
	workerCount int
	prevPage    golib.Rectangle
	nextPage    golib.Rectangle
}

type robotPortrait struct {
	robot Robot
	area  golib.Rectangle
}

// tooltip is the picked cell's panel, laid out: where it stands on the
// screen and the rows it draws. Update hit-tests it and Draw paints it, so
// both see the same geometry.
type tooltip struct {
	col, row int // the picked cell
	x, y     float32
	w, h     float32
	heading  string
	lone     bool // the cell holds one thing, whose card starts open
	rows     []tooltipRow
}

// cardOpen says whether a card stands open in the panel: a lone card and
// the primary things (deposits, buildings, the core) start open, the
// rest start folded, and a stored click overrides the default.
func cardOpen(info ThingInfo, lone bool, expanded map[string]bool, id string) bool {
	if over, ok := expanded[id]; ok {
		return over
	}
	return info.Primary || lone
}

// tooltipLayout lays the picked cell's panel out. The camera decides where
// the cell's corner lands, so the panel follows the cell while the view
// moves. Cards start open on their own: the cell's primary thing and, on
// a cell with a single thing, that thing. Deposit cards that still hold
// something get their robot button.
func tooltipLayout(
	s *State,
	camera *golib.Camera,
	col, row int,
	expanded map[string]bool,
) tooltip {
	return tooltipLayoutForSelection(s, camera, col, row, expanded, "")
}

func tooltipLayoutForSelection(
	s *State,
	camera *golib.Camera,
	col, row int,
	expanded map[string]bool,
	focusedThing string,
) tooltip {
	return tooltipLayoutForPage(
		s, camera, col, row, expanded, focusedThing, 0,
	)
}

func tooltipLayoutForPage(
	s *State,
	camera *golib.Camera,
	col, row int,
	expanded map[string]bool,
	focusedThing string,
	page int,
) tooltip {
	things := thingsAt(s, col, row)
	if focusedThing != "" {
		for _, thing := range things {
			if thing.ID == focusedThing {
				things = []Thing{thing}
				break
			}
		}
	}
	return tooltipLayoutForThings(
		s, camera, col, row, expanded, things,
		focusedThing == "", false, page,
	)
}

func tooltipLayoutForRobot(
	s *State,
	camera *golib.Camera,
	col, row int,
	expanded map[string]bool,
	robotID int64,
) tooltip {
	r, ok := s.Robots[robotID]
	if !ok {
		return tooltip{}
	}
	return tooltipLayoutForThings(
		s, camera, col, row, expanded,
		[]Thing{robotThing(s, r)}, false, true, 0,
	)
}

func tooltipLayoutForThings(
	s *State,
	camera *golib.Camera,
	col, row int,
	expanded map[string]bool,
	things []Thing,
	showWorkers, remoteRobot bool,
	page int,
) tooltip {
	t := tooltip{col: col, row: row, w: tooltipWidth}
	tcol, trow := cellTile(col, row)
	t.lone = len(things) == 1
	t.rows = append(t.rows, tooltipRow{header: true})
	for _, thing := range things {
		info := catalogInfo(thing.Type)
		open := cardOpen(info, t.lone, expanded, thing.ID)
		trash, blocked := trashFor(s, thing)
		t.rows = append(t.rows, tooltipRow{
			thing:   thing,
			title:   true,
			open:    open,
			summary: info.summarize(thing),
			trash:   trash,
			blocked: blocked,
		})
		if !open {
			continue
		}
		for _, detail := range info.Details(s, thing) {
			t.rows = append(t.rows, tooltipRow{thing: thing, detail: detail})
		}
		if showWorkers && (thing.Type == TypeOil || thing.Type == TypeLilac) {
			workers := postRobots(s, tcol, trow)
			if len(workers) > 0 {
				pages := (len(workers) + portraitPageSize - 1) / portraitPageSize
				if page >= pages {
					page = pages - 1
				}
				start := page * portraitPageSize
				end := min(start+portraitPageSize, len(workers))
				row := tooltipRow{
					thing: thing, workers: true, page: page,
					pages: pages, workerCount: len(workers),
				}
				for _, worker := range workers[start:end] {
					row.portraits = append(row.portraits,
						robotPortrait{robot: worker})
				}
				t.rows = append(t.rows, row)
			}
		}
		if b, ok := s.Buildings[thing.Ref]; ok && b.Damage > 0 &&
			buildingType(b.Kind) == thing.Type {
			left := 100 * (1 - b.Damage/buildingHealth(b.Kind))
			repairLilac, repairOil := buildingRepairCost(b.Kind, b.Damage)
			t.rows = append(t.rows, tooltipRow{thing: thing, detail: Detail{
				"integrity", fmt.Sprintf("[danger]%.0f%%[/], mechanics can repair it", left),
			}}, tooltipRow{thing: thing, detail: Detail{
				"repair cost left", costWords(repairLilac, repairOil),
			}})
		}
		if thing.Type == TypeFactory {
			if b, ok := s.Buildings[thing.Ref]; ok {
				t.rows = append(t.rows,
					productionButtonRow(
						s, thing, b, buttonBuildBuilder, RobotBuilder,
					),
					productionButtonRow(
						s, thing, b, buttonBuildWorker, RobotWorker,
					),
				)
			}
			continue
		}
		if thing.Type == TypeWarFactory {
			if b, ok := s.Buildings[thing.Ref]; ok {
				if repairProtocolUnlocked(s) {
					t.rows = append(t.rows, productionButtonRow(
						s, thing, b, buttonMechanic, RobotRepair,
					))
				}
				t.rows = append(t.rows, productionButtonRow(
					s, thing, b, buttonTrooper, RobotCombat,
				))
				if len(squadMembers(s, b.ID)) > 0 {
					t.rows = append(t.rows, tooltipRow{
						thing: thing, button: buttonOrder,
						note: "[dim]a rival or the ground[/]",
					})
				}
			}
			continue
		}
		if thing.Type == TypeSquad {
			if _, ok := s.Buildings[thing.Ref]; ok &&
				len(squadMembers(s, thing.Ref)) > 0 {
				t.rows = append(t.rows, tooltipRow{
					thing: thing, button: buttonOrder,
					note: "[dim]a rival or the ground[/]",
				})
			}
			continue
		}
		if thing.Type == TypeRobot {
			if r, ok := s.Robots[thing.Ref]; ok && r.hasPost() {
				t.rows = append(t.rows, tooltipRow{
					thing: thing, button: buttonRecall,
				})
			}
			if remoteRobot {
				t.rows = append(t.rows, tooltipRow{
					thing: thing, button: buttonBackToDeposit,
				})
			}
			continue
		}
		if end, ok := pipeEndOf(s, thing); ok {
			for _, p := range pipesOf(s, end) {
				t.rows = append(t.rows, tooltipRow{
					thing: thing, button: buttonRemovePipe,
					note: pipeNote(s, p, end), ref: p.ID,
				})
			}
			if freePorts(s, end) > 0 && dropArrived(s, techFrontierID) {
				t.rows = append(t.rows, tooltipRow{
					thing: thing, button: buttonLayPipe,
					note: fmt.Sprintf("[dim]%d of %d ports free[/]",
						freePorts(s, end), freePorts(s, end)+len(pipesOf(s, end))),
				})
			}
		}
		if thing.Type != TypeOil && thing.Type != TypeLilac {
			continue
		}
		if thing.Amount <= 0 {
			continue // a dry deposit has nobody to send
		}
		if pickRobot(s, tcol, trow) >= 0 {
			t.rows = append(t.rows, tooltipRow{
				thing: thing, button: buttonSend,
			})
		}
		if d, ok := depositAt(tcol, trow); ok && thing.Type == TypeOil {
			if pc, pr := pumpCell(d); canPlace(s, BuildingPump, pc, pr) &&
				kindUnlocked(s, BuildingPump) {
				lilac, oil := buildingCost(BuildingPump)
				t.rows = append(t.rows, tooltipRow{
					thing:    thing,
					button:   buttonBuildPump,
					costs:    resourceCosts(s, lilac, oil),
					disabled: !canAfford(s, BuildingPump),
				})
			}
		}
	}
	// Lay the rows out inside the plate: heights first, then the buttons'
	// rectangles relative to it, then everything at its final place.
	y := float32(tooltipPad)
	for i := range t.rows {
		r := &t.rows[i]
		if r.button != "" {
			r.bx, r.by = tooltipPad+detailIndent, y+2
			r.bw, r.bh = buttonWidth, buttonRow-4
			if r.button == buttonRemovePipe {
				r.bw = pipeButtonWidth
			}
		}
		if r.trash {
			r.bx, r.by = t.w-tooltipPad-trashWidth, y-1
			r.bw, r.bh = trashWidth, trashHeight
		}
		if r.workers {
			left := float32(tooltipPad + detailIndent)
			top := y + 20
			for j := range r.portraits {
				column := j % portraitColumns
				line := j / portraitColumns
				r.portraits[j].area = golib.Rectangle{
					X:     left + float32(column*(portraitWidth+portraitGap)),
					Y:     top + float32(line*(portraitHeight+portraitGap)),
					Width: portraitWidth, Height: portraitHeight,
				}
			}
			if r.pages > 1 {
				footerY := top + 2*(portraitHeight+portraitGap)
				r.prevPage = golib.Rectangle{
					X: left, Y: footerY, Width: 42, Height: 16,
				}
				r.nextPage = golib.Rectangle{
					X: t.w - tooltipPad - 42,
					Y: footerY, Width: 42, Height: 16,
				}
			}
		}
		y += rowHeight(r)
	}
	t.h = y + tooltipPad
	ux, uy := cellCenterUnits(col, row)
	sx, sy := project(float32(ux), float32(uy))
	middle := camera.ToScreen(golib.Vector2{X: sx, Y: sy})
	half := float32(buildingCell) * unitW / 2 * camera.Zoom
	panelX := middle.X + half + tooltipGap
	if panelX+t.w > float32(screenWidth)-tooltipMargin {
		panelX = middle.X - half - tooltipGap - t.w
	}
	panelX = clampf(panelX, tooltipMargin,
		float32(screenWidth)-tooltipMargin-t.w)
	panelY := clampf(
		middle.Y-titleRow/2, tooltipMargin,
		float32(screenHeight)-tooltipMargin-t.h,
	)
	t.moveTo(panelX, panelY)
	return t
}

func tooltipLayoutForSquad(
	scene *playScene,
	home int64,
) tooltip {
	building, ok := scene.state.Buildings[home]
	if !ok || len(squadMembers(scene.state, home)) == 0 {
		return tooltip{}
	}
	thing := squadThing(scene.state, home)
	t := tooltipLayoutForThings(
		scene.state, scene.camera, building.Col, building.Row,
		scene.expanded,
		[]Thing{thing}, false, false, 0,
	)
	t.heading = "squad order"
	anchor, ok := scene.squadMarkScreen(home)
	if !ok {
		return t
	}
	x := anchor.X + tooltipGap
	if x+t.w > float32(screenWidth)-tooltipMargin {
		x = anchor.X - tooltipGap - t.w
	}
	x = clampf(x, tooltipMargin,
		float32(screenWidth)-tooltipMargin-t.w)
	y := clampf(
		anchor.Y-titleRow/2, tooltipMargin,
		float32(screenHeight)-tooltipMargin-t.h,
	)
	t.moveTo(x, y)
	return t
}

func (t *tooltip) moveTo(x, y float32) {
	dx, dy := x-t.x, y-t.y
	t.x, t.y = x, y
	for i := range t.rows {
		t.rows[i].bx += dx
		t.rows[i].by += dy
		for j := range t.rows[i].portraits {
			t.rows[i].portraits[j].area.X += dx
			t.rows[i].portraits[j].area.Y += dy
		}
		t.rows[i].prevPage.X += dx
		t.rows[i].prevPage.Y += dy
		t.rows[i].nextPage.X += dx
		t.rows[i].nextPage.Y += dy
	}
}

func productionButtonRow(
	s *State,
	thing Thing,
	b Building,
	button string,
	kind RobotKind,
) tooltipRow {
	lilac, oil, _, _ := robotProduction(kind)
	return tooltipRow{
		thing:    thing,
		button:   button,
		disabled: !canQueueUnit(s, b, kind),
		costs:    resourceCosts(s, lilac, oil),
	}
}

// trashFor reports whether a thing's card carries a trash can, and
// whether it is dimmed: every building and site can go but the core,
// which has none, a building already ordered down, whose card says
// `demolishing`, and a protector that alone shelters another building.
func trashFor(s *State, thing Thing) (trash, blocked bool) {
	if thing.Type == TypeSite {
		return true, false
	}
	b, ok := s.Buildings[thing.Ref]
	if !ok || buildingType(b.Kind) != thing.Type {
		return false, false
	}
	if b.Demolish > 0 {
		return false, false
	}
	return true, !canDemolish(s, b)
}

// arm marks the card whose trash can was pressed once, so it draws red
// and asks again. Arming is view, not state.
func (t *tooltip) arm(id string) {
	for i := range t.rows {
		r := &t.rows[i]
		r.armed = r.trash && !r.blocked && id != "" && r.thing.ID == id
	}
}

// trashAt returns the thing whose trash can holds the screen point, and
// whether that one is dimmed.
func (t tooltip) trashAt(x, y float32) (thing Thing, blocked, ok bool) {
	for i := range t.rows {
		r := &t.rows[i]
		if r.trashHolds(x, y) {
			return r.thing, r.blocked, true
		}
	}
	return Thing{}, false, false
}

// trashHolds reports whether the screen point is on the row's trash
// can, with some slack around so small a target.
func (r *tooltipRow) trashHolds(x, y float32) bool {
	const slack = 3
	return r.trash &&
		x >= r.bx-slack && x <= r.bx+r.bw+slack &&
		y >= r.by-slack && y <= r.by+r.bh+slack
}

// rowHeight returns the line height of a panel row.
func rowHeight(r *tooltipRow) float32 {
	switch {
	case r.workers:
		return 20 + 2*(portraitHeight+portraitGap) + 20
	case r.title:
		return titleRow
	case r.button != "":
		return buttonRow
	}
	return textRow
}

// contains reports whether the screen point is anywhere on the panel.
func (t tooltip) contains(x, y float32) bool {
	return x >= t.x && x <= t.x+t.w && y >= t.y && y <= t.y+t.h
}

// cardAt returns the thing whose card title holds the screen point, to
// expand or fold it.
func (t tooltip) cardAt(x, y float32) (Thing, bool) {
	if x < t.x || x > t.x+t.w {
		return Thing{}, false
	}
	lineY := t.y + tooltipPad
	for i := range t.rows {
		r := &t.rows[i]
		height := rowHeight(r)
		if r.title && y >= lineY && y < lineY+height {
			return r.thing, true
		}
		lineY += height
	}
	return Thing{}, false
}

// buttonAt returns the thing whose button holds the screen point, with
// the button's label.
func (t tooltip) buttonAt(x, y float32) (Thing, string, bool) {
	if r := t.buttonRowAt(x, y); r != nil {
		return r.thing, r.button, true
	}
	return Thing{}, "", false
}

// buttonRowAt returns the row of the button that holds the screen point,
// or nil.
func (t tooltip) buttonRowAt(x, y float32) *tooltipRow {
	if x < t.x || x > t.x+t.w {
		return nil
	}
	for i := range t.rows {
		r := &t.rows[i]
		if r.workers {
			if r.page > 0 && r.prevPage.Contains(x, y) {
				r.button = buttonPortraitPrev
				return r
			}
			if r.page+1 < r.pages && r.nextPage.Contains(x, y) {
				r.button = buttonPortraitNext
				return r
			}
		}
		if r.button == "" {
			continue
		}
		if r.disabled {
			continue
		}
		if x >= r.bx && x <= r.bx+r.bw && y >= r.by && y <= r.by+r.bh {
			return r
		}
	}
	return nil
}

func (t tooltip) robotAt(x, y float32) (Robot, bool) {
	for i := range t.rows {
		for _, portrait := range t.rows[i].portraits {
			if portrait.area.Contains(x, y) {
				return portrait.robot, true
			}
		}
	}
	return Robot{}, false
}

// pipeEndOf returns the pipe end a card stands for: the core's tank, a
// silo's, a charger's, or a pump.
func pipeEndOf(s *State, thing Thing) (end int64, ok bool) {
	if thing.Type == TypeCore {
		return coreTank, true
	}
	b, found := s.Buildings[thing.Ref]
	if !found || buildingType(b.Kind) != thing.Type {
		return 0, false
	}
	_, ok = pipeEndSpot(s, b.ID)
	return b.ID, ok
}

// drawTooltip paints the panel: a dark plate, the tile's header, and one
// card per thing, named and colored by the catalog, with its details and
// button when expanded. The pointer's place lights the button it is over.
func drawTooltip(screen *golib.Screen, t tooltip, mx, my float32) {
	plate := golib.Rectangle{X: t.x, Y: t.y, Width: t.w, Height: t.h}
	screen.DrawRectangle(plate, panelColor)
	screen.DrawRectangleOutline(plate, 1, panelEdgeColor)
	x := t.x + tooltipPad
	y := t.y + tooltipPad
	for i := range t.rows {
		r := &t.rows[i]
		switch {
		case r.workers:
			drawRobotPortraits(screen, *r, mx, my)
		case r.header:
			heading := t.heading
			if heading == "" {
				heading = fmt.Sprintf("cell %d, %d", t.col, t.row)
			}
			drawMarkup(screen, "[dim]"+heading+"[/]",
				x, y, textSize, panelTextColor)
		case r.title:
			info := catalogInfo(r.thing.Type)
			screen.DrawRectangle(
				golib.Rectangle{X: x, Y: y + 1, Width: barWidth, Height: titleRow - 6},
				info.Color)
			tx := x + barWidth + 6
			mark := "+ "
			if r.open {
				mark = "- "
			}
			tx = drawMarkup(screen, mark, tx, y, titleSize, panelDimColor)
			screen.DrawText(info.Name, tx, y, titleSize, info.Color, uiText)
			summary, summaryColor := r.summary, panelDimColor
			right := t.x + t.w - tooltipPad
			if r.trash {
				over := r.trashHolds(mx, my)
				color := panelDimColor
				switch {
				case r.blocked:
					color = blockedColor
					if over {
						summary = blockedSummary
					}
				case r.armed:
					color = dangerColor
					summary, summaryColor = armedSummary, dangerColor
				case over:
					color = panelTextColor
				}
				drawTrashCan(screen, r.bx, r.by, color)
				right = r.bx - trashGap
			}
			screen.DrawText(summary, right, y, textSize, summaryColor,
				golib.TextOptions{Font: uiFont, Align: golib.AlignRight})
		case r.button != "":
			fill, ink, edge := buttonColor, panelTextColor, buttonEdgeColor
			rect := golib.Rectangle{X: r.bx, Y: r.by, Width: r.bw, Height: r.bh}
			if r.disabled {
				fill, ink, edge = panelColor, panelDimColor, blockedColor
			} else if rect.Contains(mx, my) {
				fill = buttonHoverColor
			}
			screen.DrawRectangle(rect, fill)
			screen.DrawRectangleOutline(rect, 1, edge)
			screen.DrawText(r.button, r.bx+8, r.by+4, textSize, ink, uiText)
			if len(r.costs) > 0 {
				drawCostParts(screen, r.costs,
					r.bx+r.bw+10, r.by+4, textSize)
			} else if r.note != "" {
				drawMarkup(screen, r.note, r.bx+r.bw+10, r.by+4,
					textSize, panelTextColor)
			}
		default:
			screen.DrawText(r.detail.Label, x+detailIndent, y, textSize, panelDimColor, uiText)
			drawMarkup(screen, r.detail.Value, x+detailIndent+detailLabelW, y,
				textSize, panelTextColor)
		}
		y += rowHeight(r)
	}
}

func drawRobotPortraits(
	screen *golib.Screen,
	row tooltipRow,
	mx, my float32,
) {
	first := row.page*portraitPageSize + 1
	last := min((row.page+1)*portraitPageSize, row.workerCount)
	if len(row.portraits) > 0 {
		screen.DrawText(
			fmt.Sprintf("robots %d-%d of %d", first, last, row.workerCount),
			row.portraits[0].area.X, row.portraits[0].area.Y-18,
			textSize, panelDimColor, uiText,
		)
	}
	for _, portrait := range row.portraits {
		fill := buttonColor
		if portrait.area.Contains(mx, my) {
			fill = buttonHoverColor
		}
		screen.DrawRectangle(portrait.area, fill)
		screen.DrawRectangleOutline(portrait.area, 1, buttonEdgeColor)
		center := golib.Vector2{
			X: portrait.area.X + portrait.area.Width/2,
			Y: portrait.area.Y + 8,
		}
		robotModel(portrait.robot.Kind).drawIcon(
			screen, center, portrait.robot.Facing, golib.White,
		)
		screen.DrawText(
			fmt.Sprintf("#%d", portrait.robot.ID),
			center.X, portrait.area.Y+19, 9, panelTextColor,
			golib.TextOptions{Font: uiFont, Align: golib.AlignCenter},
		)
	}
	if row.pages <= 1 {
		return
	}
	for i, button := range []struct {
		area golib.Rectangle
		text string
	}{
		{row.prevPage, "prev"},
		{row.nextPage, "next"},
	} {
		fill := buttonColor
		enabled := (i == 0 && row.page > 0) ||
			(i == 1 && row.page+1 < row.pages)
		if !enabled {
			fill = panelColor
		} else if button.area.Contains(mx, my) {
			fill = buttonHoverColor
		}
		screen.DrawRectangle(button.area, fill)
		screen.DrawRectangleOutline(button.area, 1, buttonEdgeColor)
		screen.DrawText(
			button.text, button.area.X+button.area.Width/2,
			button.area.Y+2, 10, panelTextColor,
			golib.TextOptions{Font: uiFont, Align: golib.AlignCenter},
		)
	}
}

// drawTrashCan paints a trash can, trashWidth by trashHeight pixels
// from its top left corner: a handle, a lid and a ribbed body.
func drawTrashCan(screen *golib.Screen, x, y float32, color golib.Color) {
	rect := func(rx, ry, w, h float32) {
		screen.DrawRectangle(
			golib.Rectangle{X: x + rx, Y: y + ry, Width: w, Height: h}, color)
	}
	rect(4, 0, 3, 1)  // the handle
	rect(0, 2, 11, 2) // the lid
	rect(1, 5, 1, 8)  // the body's sides and bottom
	rect(9, 5, 1, 8)
	rect(1, 12, 9, 1)
	rect(4, 6, 1, 5) // the ribs
	rect(6, 6, 1, 5)
}

// findButton returns the row of the first button labeled so, or nil.
func (t tooltip) findButton(label string) *tooltipRow {
	for i := range t.rows {
		if t.rows[i].button == label {
			return &t.rows[i]
		}
	}
	return nil
}

// pickPumpAt selects the pump or pump site whose drawn body holds the
// screen point.
func (s *playScene) pickPumpAt(x, y float32) bool {
	for _, id := range sortedBuildingIDs(s.state) {
		b := s.state.Buildings[id]
		if b.Kind != BuildingPump {
			continue
		}
		if !pumpBodyContains(s, x, y, b.Col, b.Row) {
			continue
		}
		s.picked = true
		s.pickedCol, s.pickedRow = b.Col, b.Row
		s.pickedThing = buildingThing(b).ID
		s.pickedRobot = 0
		s.clearPickedUnit()
		s.robotPage = 0
		s.armed = ""
		s.closeRadial()
		return true
	}
	for _, job := range s.state.Jobs {
		if job.Kind != BuildingPump {
			continue
		}
		if !pumpBodyContains(s, x, y, job.Col, job.Row) {
			continue
		}
		s.picked = true
		s.pickedCol, s.pickedRow = job.Col, job.Row
		s.pickedThing = siteThing(job).ID
		s.pickedRobot = 0
		s.clearPickedUnit()
		s.robotPage = 0
		s.armed = ""
		s.closeRadial()
		return true
	}
	return false
}

// pumpBodyContains tests the pump's projected body, including its icon
// scale when the camera is far out.
func pumpBodyContains(s *playScene, x, y float32, col, row int) bool {
	gx, gy := projectBuilding(Building{Col: col, Row: row})
	across, height := buildingSize(BuildingPump)
	icon := buildingIcon(across, height, s.zoom) * s.zoom
	foot := s.camera.ToScreen(golib.Vector2{X: gx, Y: gy})
	halfWidth := across * unitW * icon / 2
	capRadius := across * unitW * icon * 0.22
	top := foot.Y - (height+1)*unitH*icon - capRadius
	bottom := foot.Y + across*unitH*icon/2
	return x >= foot.X-halfWidth && x <= foot.X+halfWidth &&
		y >= top && y <= bottom
}

// cellDiamond returns a cell's outline in world space, lifted to a
// readable size on the screen while the view is far out.
func cellDiamond(col, row int, zoom float32) (shape []golib.Vector2, gx, gy float32) {
	x, y := cellCenterUnits(col, row)
	gx, gy = project(float32(x), float32(y))
	scale := float32(buildingCell) / unitsPerTile
	if min := 12 / zoom / tileW; scale < min {
		scale = min
	}
	return scaledDiamond(gx, gy, scale), gx, gy
}

// clampf keeps v inside low and high.
func clampf(v, low, high float32) float32 {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}
