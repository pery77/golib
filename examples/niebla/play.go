package main

import (
	"fmt"
	"math"

	"golib"
)

// Camera tuning, after R.U.S.E.: the wheel moves between whole stops and
// each stop doubles the zoom, so the range is huge — stop 0 shows the
// whole region with its deposits as icons, stop 5 shows about 80 m of
// ground. The zoom glides from stop to stop; only at rest is it a whole
// power of two, so shapes stay square. Panning keeps its speed on the
// screen, not in the world, so it glides the same at every zoom.
const (
	zoomOut = 0 // the stop that shows the whole region
	zoomIn  = 5 // the stop that shows the ground

	panSpeed = 480 // screen pixels per second

	zoomGlide = 0.1 // seconds for the zoom to close most of the way to its next stop
)

// zoomOfStop returns the zoom a stop stands for: two to the stop's power.
func zoomOfStop(stop float32) float32 {
	return float32(math.Exp2(float64(stop)))
}

// autosaveTicks is how often the base saves itself: 900 ticks, 15
// seconds of game time, so a window closed without ceremony loses less
// than that. Leaving to the menu saves at once.
const autosaveTicks = 900

// playScene shows the region through a camera. The camera, selection,
// menus and robot roster are view, not state: they live here, outside the
// simulation, and never get serialized. The simulation's state does:
// Update turns input into actions and sends one Tick per update, and Draw
// renders the state and changes nothing.
type playScene struct {
	state *State
	slot  string
	mites *miteField // the fog's wear on what stands in it; looks only
	fx    *fxField   // shots' light, flashes, sparks and smoke; looks only
	costs *spendingField
	au    *audioField // the region's sound; looks and hears only
	dev   devTools

	camera       *golib.Camera
	zoom         float32       // the zoom on screen, gliding toward zoomOfStop(zoomStop)
	zoomStop     float32       // the whole stop the wheel last asked for
	anchorWorld  golib.Vector2 // while gliding, the point kept under the cursor
	anchorScreen golib.Vector2
	dragging     bool          // the right button is down and moving the view
	dragFrom     golib.Vector2 // where the cursor stood at the last drag update
	mouse        golib.Vector2 // where the pointer stands, to light buttons

	savedTicks int64 // ticks since the base last saved itself
	saveFailed bool  // the last autosave couldn't be written

	picked         bool // a cell is selected and shows its panel
	pickedCol      int  // the selected cell
	pickedRow      int
	pickedThing    string          // a visible body's card picked on that cell
	pickedRobot    int64           // a worker opened from a deposit portrait
	pickedSquad    int64           // a squad opened from its pennant or target ring
	pickedUnit     unitSelection   // a world unit selected directly
	robotPage      int             // which page of the selected deposit's portraits
	expanded       map[string]bool // which cards stand open, by thing ID
	armed          string          // the card whose trash can was pressed once, by thing ID
	radial         bool            // the build menu stands open on a cell
	radialCol      int             // the cell the menu opened on
	radialRow      int
	radialLevel    int        // the ring open: 0 the groups, 1 their blueprints
	radialGroup    buildGroup // the group the second ring shows
	laying         pipeLaying // the pipe the pointer is drawing, if any
	ordering       int64      // the war factory whose squad the pointer is ordering; 0 is none
	orderTicks     int
	robotsOpen     bool
	robotsPage     int
	robotsPicked   int64
	assigningRobot int64
	techCallout    string // the schematics drop whose callout stands open, by ID
	techPlacing    BuildingKind
	techUsed       map[BuildingKind]bool
	hoverCellCol   int  // the cell under the pointer, the cursor
	hoverCellRow   int  //
	hoverCell      bool // the pointer is over a cell
	rightWasDown   bool
	rightFrom      golib.Vector2 // where the right button went down
}

// newPlayScene takes up the base it is given, dealing a new one when
// none came — the menu hands in the player's save, or nil for a new
// region. The monitor filters are already on: the menu, where every
// visit to this scene starts, put them up.
func newPlayScene(state *State) *playScene {
	if state == nil {
		state = newGame()
	}
	state.enterRegion()
	s := &playScene{
		state:    state,
		slot:     regionSlot,
		mites:    newMiteField(),
		fx:       newFxField(),
		costs:    newSpendingField(),
		au:       newAudioField(),
		expanded: map[string]bool{},
	}
	s.camera = golib.NewCamera(float32(screenWidth), float32(screenHeight))
	s.camera.Bounds = regionOnScreen()
	s.camera.Target = s.camera.Bounds.Center()
	s.zoomStop, s.zoom = zoomOut, zoomOfStop(zoomOut)
	s.camera.Zoom = s.zoom
	s.camera.Snap()
	activeResize = s.resize
	return s
}

func (s *playScene) resize(width, height int) {
	camera := golib.NewCamera(float32(width), float32(height))
	camera.Bounds = s.camera.Bounds
	camera.Target = s.camera.Target
	camera.Zoom = s.zoom
	camera.Snap()
	s.camera = camera
}

func (s *playScene) Update(input *golib.Input, dt float32) {
	s.state.Costs = nil
	mx, my := input.MousePosition()
	s.mouse = golib.Vector2{X: mx, Y: my}
	// Esc or Back saves the base and returns to the menu, the way out of
	// the region. No other key quits, and the menu is where quitting from.
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadBack) {
		s.saveNow()
		s.au.stopLoops()
		golib.SwitchScene(newMenuScene())
		return
	}
	s.tickOrdering()
	// F11 or Alt+Enter switches fullscreen; GoLib resizes the screen too.
	altEnter := (input.KeyDown(golib.KeyLeftAlt) || input.KeyDown(golib.KeyRightAlt)) &&
		input.KeyPressed(golib.KeyEnter)
	if input.KeyPressed(golib.KeyF11) || altEnter {
		golib.SetFullscreen(!golib.IsFullscreen())
	}
	// F2 turns the monitor filters on and off.
	if input.KeyPressed(golib.KeyF2) {
		setFilters(!monitor.on)
	}
	s.updateCamera(input, dt)
	previousReport, hadReport := latestReport(s.state)
	taken := s.dev.update(s, input)
	if !taken {
		taken = s.updateTech(input)
	}
	if !taken {
		if s.techPlacing == "" {
			taken = s.updateRobotPanel(input)
		}
	}
	if !taken && s.techPlacing == "" {
		taken = s.updateSquadBoxes(input)
	}
	if s.techPlacing != "" {
		s.updateInspection(input, taken)
	} else {
		s.updateRadial()
		s.updateInspection(input, taken)
		s.updateSquadKeys(input)
	}
	// The loop is the clock: one tick of simulation per update, more
	// while the dev tools fast forward.
	ticks := s.dev.ticksPerUpdate()
	var deaths []UnitDeath
	var buildingDeaths []BuildingDeath
	costs := append([]CostReceipt(nil), s.state.Costs...)
	for i := 0; i < ticks; i++ {
		Apply(s.state, Tick{})
		deaths = append(deaths, s.state.Deaths...)
		buildingDeaths = append(
			buildingDeaths, s.state.BuildingDeaths...,
		)
		costs = append(costs, s.state.Costs...)
		s.au.update(s, 1)
		if report, ok := latestReport(s.state); ok &&
			(!hadReport || report != previousReport) {
			s.au.warning()
			previousReport, hadReport = report, true
		}
	}
	if s.pickedRobot != 0 {
		if _, alive := s.state.Robots[s.pickedRobot]; !alive {
			s.pickedRobot = 0
		}
	}
	s.syncPickedUnit()
	s.syncPickedSquad()
	s.clampRobotPage()
	s.clampRobotPanel()
	s.mites.update(s.state, dt)
	s.fx.update(s.state, dt, deaths, buildingDeaths)
	s.costs.update(costs, float32(ticks)/60)
	s.savedTicks += int64(ticks)
	if s.savedTicks >= autosaveTicks {
		s.saveNow()
	}
}

// saveNow writes the base to the local database, when saving is on, and
// remembers a failure so the HUD can say so.
func (s *playScene) saveNow() {
	s.savedTicks = 0
	if db == nil {
		return
	}
	s.saveFailed = saveBase(s.slot, s.state) != nil
}

// updateSquadKeys calls a squad with the number keys: 1 arms the oldest
// war factory's squad for an order, 2 the next, and so on, the same way
// its card's give order does; the same key again puts the order away.
// Arming takes effect from the next update, so the click of the update
// that pressed the key never fires an order by itself. The keys stand
// down while the pointer is laying a pipe, whose clicks they would
// fight.
func (s *playScene) updateSquadKeys(input *golib.Input) {
	if s.laying.on || s.assigningRobot != 0 {
		return
	}
	slots := squadSlots(s.state)
	for i := 0; i < squadKeys && i < len(slots); i++ {
		if input.KeyPressed(golib.KeyOne + golib.Key(i)) {
			if s.ordering == slots[i] {
				s.ordering = 0
			} else {
				s.armOrdering(slots[i])
				s.au.ui(1)
				s.closeRadial()
			}
			return
		}
	}
}

// updateTech opens the oldest unopened drop from its badge and arms a
// building from an unused square. Used and informational squares do
// nothing; other callout clicks dismiss it or pass through to the region.
func (s *playScene) updateTech(input *golib.Input) bool {
	if s.assigningRobot != 0 || s.techPlacing != "" {
		return false
	}
	if !input.MousePressed(golib.MouseLeft) {
		return false
	}
	mx, my := input.MousePosition()
	if s.techCallout != "" {
		if square, hit := techSquareAt(s, mx, my); hit {
			if square.item.informational || square.used {
				if square.used {
					s.au.ui(0.6)
				}
				return true
			}
			if s.selectTechBuilding(square.item.kind) {
				s.au.ui(1)
				return true
			}
			return true
		}
		return s.dismissTechCallout(mx, my)
	}
	if id := techPending(s.state); id != "" && s.techBadgeHolds(mx, my) {
		Apply(s.state, AckTech{ID: id})
		s.au.ui(1.1)
		s.techCallout = id
		s.techUsed = map[BuildingKind]bool{}
		s.picked = false
		s.pickedThing = ""
		s.pickedRobot = 0
		s.pickedSquad = 0
		s.clearPickedUnit()
		s.armed = ""
		s.ordering = 0
		s.laying = pipeLaying{}
		s.robotsOpen = false
		s.closeRadial()
		return true
	}
	return false
}

// updateSquadBoxes takes a click on one of the squads' boxes the way
// its key would: the box arms that squad for an order, or takes the
// order away if it was armed. It reports whether the click belonged to
// a box, so the region under it never hears of it; a box that no squad
// holds doesn't exist and passes the click through.
func (s *playScene) updateSquadBoxes(input *golib.Input) bool {
	if !input.MousePressed(golib.MouseLeft) || s.laying.on ||
		s.assigningRobot != 0 {
		return false
	}
	mx, my := input.MousePosition()
	i, over := squadBoxAt(mx, my)
	if !over {
		return false
	}
	slots := squadSlots(s.state)
	if i < len(slots) {
		if s.ordering == slots[i] {
			s.ordering = 0
		} else {
			s.armOrdering(slots[i])
			s.au.ui(1)
			s.closeRadial()
		}
		return true
	}
	return false
}

// updateCamera pans with WASD, the arrows or the left stick, drags with the
// right button, glides the zoom in whole steps with the wheel, and keeps the
// view inside the region.
func (s *playScene) updateCamera(input *golib.Input, dt float32) {
	s.zoomCamera(input, dt)
	s.dragCamera(input)
	s.panCamera(input, dt)
	s.camera.Update(dt)
}

func (s *playScene) panCamera(input *golib.Input, dt float32) {
	dx, dy := float32(0), float32(0)
	if input.KeyDown(golib.KeyW) || input.KeyDown(golib.KeyUp) {
		dy--
	}
	if input.KeyDown(golib.KeyS) || input.KeyDown(golib.KeyDown) {
		dy++
	}
	if input.KeyDown(golib.KeyA) || input.KeyDown(golib.KeyLeft) {
		dx--
	}
	if input.KeyDown(golib.KeyD) || input.KeyDown(golib.KeyRight) {
		dx++
	}
	if stickX, stickY := input.GamepadLeftStick(0); stickX != 0 || stickY != 0 {
		dx, dy = stickX, stickY
	}
	walked := math.Hypot(float64(dx), float64(dy))
	if walked == 0 {
		return
	}
	step := panSpeed / s.zoom * dt
	s.camera.Target.X += dx / float32(walked) * step
	s.camera.Target.Y += dy / float32(walked) * step
}

// zoomCamera glides the zoom to the stop the wheel asks for, keeping the
// point under the cursor under it while it moves. The in-between zooms
// only exist while gliding: at rest the zoom is a whole power of two.
func (s *playScene) zoomCamera(input *golib.Input, dt float32) {
	if notches := input.MouseWheel(); notches != 0 {
		stop := golib.Clamp(s.zoomStop+float32(int(notches)), zoomOut, zoomIn)
		if stop != s.zoomStop {
			s.zoomStop = stop
			mx, my := input.MousePosition()
			s.anchorWorld = s.camera.ToWorld(mx, my)
			s.anchorScreen = golib.Vector2{X: mx, Y: my}
		}
	}
	if target := zoomOfStop(s.zoomStop); s.zoom != target {
		keep := float32(math.Exp(float64(-dt / zoomGlide)))
		s.zoom += (target - s.zoom) * (1 - keep)
		if math.Abs(float64(target-s.zoom)) < 0.001 {
			s.zoom = target
		}
		s.camera.Zoom = s.zoom
		// Where the center must sit for the anchor to stay under the
		// cursor: the anchor minus the cursor's offset from the middle.
		s.camera.Target = golib.Vector2{
			X: s.anchorWorld.X -
				(s.anchorScreen.X-float32(screenWidth)/2)/s.zoom,
			Y: s.anchorWorld.Y -
				(s.anchorScreen.Y-float32(screenHeight)/2)/s.zoom,
		}
	}
}

// dragCamera moves the view with the right button, the way a hand drags a
// map: the grab moves the ground with the cursor's opposite.
func (s *playScene) dragCamera(input *golib.Input) {
	if !input.MouseDown(golib.MouseRight) {
		s.dragging = false
		return
	}
	mx, my := input.MousePosition()
	if !s.dragging {
		s.dragging = true
		s.dragFrom = golib.Vector2{X: mx, Y: my}
		return
	}
	// Shifting the anchor with the view keeps the zoom's glide from
	// fighting the drag when both move at once.
	dx := (mx - s.dragFrom.X) / s.zoom
	dy := (my - s.dragFrom.Y) / s.zoom
	s.camera.Target.X -= dx
	s.camera.Target.Y -= dy
	s.anchorWorld.X -= dx
	s.anchorWorld.Y -= dy
	s.dragFrom = golib.Vector2{X: mx, Y: my}
}

// updateRadial puts a menu away whose ring went empty while it stood
// open: a robot walked onto the cell, or a job or pile took it, and an
// empty ring is no menu. The stores don't empty a ring - what they
// can't pay stands washed out - so only the ground's answer closes it.
func (s *playScene) updateRadial() {
	if !s.radial {
		return
	}
	if s.radialLevel == 0 {
		if len(radialGroupLayout(s)) == 0 {
			s.closeRadial()
		}
		return
	}
	if len(radialLeafLayout(s)) == 0 {
		s.closeRadial()
	}
}

// updateInspection picks the cell under the pointer with the left button,
// cancels with a right click that never became a drag, expands or folds
// a card when a click lands on its title, and acts when a click lands on
// a card's button. A click on buildable empty ground opens the build menu,
// unless a guard pennant marks that cell; that click selects the cell
// instead. Picking a radial option marks its blueprint there. A blueprint
// selected from the schematics callout owns
// the pointer until it is placed or canceled. The camera has already
// moved, so the hover follows the view the frame it changes. A left click
// the dev tools took is none of its business.
func (s *playScene) updateInspection(input *golib.Input, clickTaken bool) {
	mx, my := input.MousePosition()
	world := s.camera.ToWorld(mx, my)
	// The cursor is the cell, the grid's last subdivision, whatever the
	// scene is doing with it.
	if cc, cr, inCell := cellAtWorld(float64(world.X), float64(world.Y)); inCell {
		s.hoverCell, s.hoverCellCol, s.hoverCellRow = true, cc, cr
	} else {
		s.hoverCell = false
	}

	rightClick := s.rightClicked(input)
	if s.techPlacing != "" {
		if rightClick {
			s.techPlacing = ""
		}
		if input.MousePressed(golib.MouseLeft) && !clickTaken {
			s.placeTechBuilding()
		}
		return
	}
	if rightClick && s.techCallout != "" {
		s.closeTechCallout()
		return
	}
	if s.assigningRobot != 0 {
		if rightClick {
			s.assigningRobot = 0
			return
		}
		if !input.MousePressed(golib.MouseLeft) || clickTaken {
			return
		}
		if s.assignRobotAtScreen(mx, my) {
			s.assigningRobot = 0
			s.au.ui(1)
		}
		return
	}
	if s.laying.on {
		s.updateLaying(input, clickTaken, rightClick)
		return
	}
	if s.ordering != 0 {
		s.updateOrdering(input, clickTaken, rightClick)
		return
	}
	if rightClick {
		s.picked = false
		s.pickedThing = ""
		s.pickedRobot = 0
		s.pickedSquad = 0
		s.clearPickedUnit()
		s.robotPage = 0
		s.armed = ""
		s.backRadial()
	}

	if input.MousePressed(golib.MouseLeft) && !clickTaken {
		// A trash can asks twice: any click but the second one on it
		// disarms it.
		armed := s.armed
		s.armed = ""
		// The panel, while it stands, wins over whatever sits under it:
		// its buttons act even where it covers buildable ground.
		if s.picked {
			panel := s.inspectionPanel()
			if panel.contains(mx, my) {
				if thing, blocked, ok := panel.trashAt(mx, my); ok {
					switch {
					case blocked:
					case armed == thing.ID:
						s.demolish(thing)
					default:
						s.armed = thing.ID
					}
					return
				}
				if row := panel.buttonRowAt(mx, my); row != nil {
					s.pressButton(*row)
					if row.button == buttonRecall && s.pickedRobot != 0 {
						s.pickedRobot = 0
					}
					return
				}
				if robot, ok := panel.robotAt(mx, my); ok {
					s.pickedRobot = robot.ID
					s.pickedThing = ""
					s.clearPickedUnit()
					s.expanded[robotThing(s.state, robot).ID] = true
					s.au.ui(1)
					return
				}
				if thing, ok := panel.cardAt(mx, my); ok {
					// A click folds or opens from where the card stands:
					// the default for its kind, or the last click's choice.
					open := cardOpen(catalogInfo(thing.Type), panel.lone, s.expanded, thing.ID)
					s.expanded[thing.ID] = !open
				}
				return
			}
		}
		s.pickedRobot = 0
		s.pickedSquad = 0
		s.robotPage = 0
		if s.pickPumpAt(mx, my) {
			return
		}
		if s.pickSquadMark(mx, my) {
			return
		}
		if hit, ok := s.hoveredUnit(); ok {
			s.selectUnit(hit)
			return
		}
		if s.radial {
			s.pickRadial(mx, my)
		} else {
			s.pickCellOrBuild(
				s.hoverCellCol, s.hoverCellRow, s.hoverCell,
			)
		}
	}
}

func (s *playScene) inspectionPanel() tooltip {
	if thing, col, row, ok := s.selectedUnitThing(); ok {
		return tooltipLayoutForThings(
			s.state, s.camera, col, row, s.expanded,
			[]Thing{thing}, false, false, 0,
		)
	}
	if s.pickedRobot != 0 {
		if _, alive := s.state.Robots[s.pickedRobot]; alive {
			return tooltipLayoutForRobot(
				s.state, s.camera, s.pickedCol, s.pickedRow,
				s.expanded, s.pickedRobot,
			)
		}
	}
	if s.pickedSquad != 0 {
		return tooltipLayoutForSquad(s, s.pickedSquad)
	}
	return tooltipLayoutForPage(
		s.state, s.camera, s.pickedCol, s.pickedRow,
		s.expanded, s.pickedThing, s.robotPage,
	)
}

func (s *playScene) clampRobotPage() {
	if !s.picked || s.pickedRobot != 0 || s.pickedSquad != 0 ||
		s.pickedUnit.id != 0 {
		return
	}
	tcol, trow := cellTile(s.pickedCol, s.pickedRow)
	workers := postRobots(s.state, tcol, trow)
	pages := (len(workers) + portraitPageSize - 1) / portraitPageSize
	if pages == 0 {
		s.robotPage = 0
		return
	}
	if s.robotPage >= pages {
		s.robotPage = pages - 1
	}
}

// rightClicked reports a right click: a press and a release within a few
// pixels. Anything more was a drag, and drags cancel nothing.
func (s *playScene) rightClicked(input *golib.Input) bool {
	mx, my := input.MousePosition()
	down := input.MouseDown(golib.MouseRight)
	if down && !s.rightWasDown {
		s.rightFrom = golib.Vector2{X: mx, Y: my}
	}
	clicked := s.rightWasDown && !down &&
		math.Abs(float64(mx-s.rightFrom.X)) < 4 &&
		math.Abs(float64(my-s.rightFrom.Y)) < 4
	s.rightWasDown = down
	return clicked
}

// buildableCell reports whether a cell may ask for the build menu:
// buildable ground, nothing raised, rising or lying there, no robot
// standing on it.
func (s *playScene) buildableCell(col, row int) bool {
	if !buildableGround(col, row) {
		return false
	}
	if _, occupied := buildingAt(s.state, col, row); occupied {
		return false
	}
	for _, job := range s.state.Jobs {
		if job.Col == col && job.Row == row {
			return false
		}
	}
	if _, littered := pileAt(s.state, col, row); littered {
		return false
	}
	return !unitOnCell(s.state, col, row)
}

func (s *playScene) pickCellOrBuild(col, row int, inside bool) {
	if inside && techAnyArrived(s.state) && s.buildableCell(col, row) &&
		!s.squadPennantInCell(col, row) {
		s.openRadial(col, row)
		s.picked = false
		s.pickedSquad = 0
		s.pickedThing = ""
		s.clearPickedUnit()
		return
	}
	s.picked = inside
	s.pickedCol, s.pickedRow = col, row
	s.pickedSquad = 0
	s.pickedThing = ""
	s.clearPickedUnit()
}

func unitOnCell(s *State, col, row int) bool {
	for _, id := range sortedRobotIDs(s) {
		r := s.Robots[id]
		if int(r.X/buildingCell) == col && int(r.Y/buildingCell) == row {
			return true
		}
	}
	return len(enemiesOnCell(s, col, row)) > 0
}

// pressButton applies the action a card's button asks for on the picked
// cell. Deposits are tiles, so their actions take the cell's tile.
func (s *playScene) pressButton(row tooltipRow) {
	s.au.ui(1)
	thing := row.thing
	tcol, trow := cellTile(s.pickedCol, s.pickedRow)
	switch row.button {
	case buttonSend:
		Apply(s.state, SendRobot{Col: tcol, Row: trow})
	case buttonRecall:
		Apply(s.state, RecallRobot{ID: thing.Ref})
	case buttonBackToDeposit:
		s.pickedRobot = 0
	case buttonPortraitPrev:
		if s.robotPage > 0 {
			s.robotPage--
		}
	case buttonPortraitNext:
		workers := postRobots(s.state, tcol, trow)
		pages := (len(workers) + portraitPageSize - 1) / portraitPageSize
		if s.robotPage+1 < pages {
			s.robotPage++
		}
	case buttonBuildBuilder:
		Apply(s.state, QueueRobot{
			Building: thing.Ref, Kind: RobotBuilder,
		})
	case buttonBuildWorker:
		Apply(s.state, QueueRobot{
			Building: thing.Ref, Kind: RobotWorker,
		})
	case buttonTrooper:
		Apply(s.state, QueueRobot{
			Building: thing.Ref, Kind: RobotCombat,
		})
	case buttonMechanic:
		Apply(s.state, QueueMechanic{Building: thing.Ref})
	case buttonOrder:
		s.armOrdering(thing.Ref)
	case buttonBuildPump:
		if d, ok := depositAt(tcol, trow); ok {
			col, row := pumpCell(d)
			before := len(s.state.Jobs)
			Apply(s.state, MarkBuilding{Kind: BuildingPump, Col: col, Row: row})
			if len(s.state.Jobs) > before {
				s.au.placed()
			}
		}
	case buttonLayPipe:
		if end, ok := pipeEndOf(s.state, thing); ok {
			s.startLaying(end)
		}
	case buttonRemovePipe:
		Apply(s.state, RemovePipe{Pipe: row.ref})
	}
}

// demolish applies what a card's armed trash can asks for: a site
// leaves the queue, a building comes down.
func (s *playScene) demolish(thing Thing) {
	s.au.ui(0.8)
	if thing.Type == TypeSite {
		Apply(s.state, CancelJob{Col: thing.CellCol, Row: thing.CellRow})
		return
	}
	Apply(s.state, Demolish{Building: thing.Ref})
}

// regionOnScreen returns where the region's diamond lands on the screen, the
// rectangle the camera's view stays inside.
func regionOnScreen() golib.Rectangle {
	width := float32(regionCols+regionRows) * tileW / 2
	height := float32(regionCols+regionRows) * tileH / 2
	return golib.Rectangle{
		X:      regionOriginX - width/2,
		Y:      regionOriginY,
		Width:  width,
		Height: height,
	}
}

// Draw draws the region through the camera, and the text over it in screen
// pixels. It reads the state and never changes it.
func (s *playScene) Draw(screen *golib.Screen) {
	screen.SetCamera(s.camera)
	corner := s.camera.ToWorld(0, 0)
	drawRegion(s.state, screen, s.camera, s.zoom, golib.Rectangle{
		X: corner.X, Y: corner.Y,
		Width:  float32(screenWidth) / s.zoom,
		Height: float32(screenHeight) / s.zoom,
	})
	s.mites.draw(screen, s.zoom)
	s.fx.draw(s.state, screen, s.zoom)
	// The cursor is a unit's body when the pointer lands on one, otherwise
	// the cell under it. The cell lifts to a readable size far out.
	if s.techPlacing != "" {
		drawTechPlacementGhost(s, screen)
	} else if hit, over := s.hoveredUnit(); over {
		drawUnitOutline(screen, s.camera, hit.visible, hoveredTileColor)
	} else if s.hoverCell {
		cursor, gx, gy := cellDiamond(s.hoverCellCol, s.hoverCellRow, s.zoom)
		screen.DrawPolygonOutline(cursor, 2/s.zoom, hoveredTileColor)
		screen.DrawCircle(gx, gy, 2.5/s.zoom, hoveredTileColor)
	}
	if s.picked {
		if s.pickedUnit.id != 0 {
			if outline, ok := s.unitBounds(s.pickedUnit); ok {
				drawUnitOutline(screen, s.camera, outline, pickedTileColor)
			}
		} else if s.pickedSquad == 0 {
			outline, _, _ := cellDiamond(s.pickedCol, s.pickedRow, s.zoom)
			screen.DrawPolygonOutline(outline, 2/s.zoom, pickedTileColor)
		}
		// A picked guard post shows its reach.
		if s.pickedUnit.id == 0 {
			if b, ok := buildingAt(s.state, s.pickedCol, s.pickedRow); ok && b.Kind == BuildingGuard {
				gx, gy := projectBuilding(b)
				ellipseOutline(screen, gx, gy,
					smallArmsRangeUnits/unitsPerTile,
					1.5/s.zoom, golib.WithOpacity(guardColor, 0.8))
			}
			// And a picked artillery piece its two: the reach, and the ring it
			// can't fire inside.
			if b, ok := buildingAt(s.state, s.pickedCol, s.pickedRow); ok && b.Kind == BuildingArtillery {
				gx, gy := projectBuilding(b)
				for _, reach := range []float32{artilleryRangeUnits, artilleryMinUnits} {
					ellipseOutline(screen, gx, gy, reach/unitsPerTile,
						1.5/s.zoom, golib.WithOpacity(guardColor, 0.8))
				}
			}
		}
	}
	if s.laying.on {
		s.drawLaying(screen)
	}
	if s.ordering != 0 {
		s.drawOrderingPreview(screen)
	}
	screen.SetCamera(nil)
	s.costs.draw(s.camera, screen)
	drawSwellStatic(s.state, screen, s.camera)
	drawIdleCount(s.state, screen, s.camera)
	drawTechBadge(s, screen)
	screen.DrawText("niebla", 16, 12, 24, textColor, uiText)
	hudRight := robotPanelButtonRect().X - 8
	if count := min(squadKeys, len(squadSlots(s.state))); count > 0 {
		hudRight = min(hudRight, squadBoxRect(count-1).X-8)
	}
	hudSize := statusTextSize()
	hudBottom := drawMarkupWrapped(screen, s.hudLine(), 16, 44,
		max(40, hudRight-16), hudSize, textColor)
	s.dev.stripY = max(devStripY, hudBottom+8)
	reportTop := hudBottom + 8
	if devOpen {
		for i := 0; i <= devSendBattalionButton; i++ {
			button := s.dev.buttonBounds(i)
			reportTop = max(reportTop, button.Y+button.Height+8)
		}
		hudRight = screen.Width() - 8
	}
	drawReport(s.state, screen, reportTop, hudRight)
	drawTechCallout(s, screen)
	drawSquadStrip(s, screen)
	s.dev.draw(s, screen)
	help := "click ground to build; inspect cells and units; " +
		"squad marks open cards; wheel zooms, WASD / right-drag pans, " +
		"1-9 squads, Esc menu, " +
		"F11 fullscreen, F2 filter"
	if s.techCallout != "" && s.techPlacing == "" {
		help = "schematics received: click outside or right-click to close"
		if techBuildingsRemain(s.techCallout, s.techUsed) {
			help = "click a building square once to place it; " +
				"outside or right-click closes the callout"
		}
	}
	if s.ordering != 0 {
		help = "ordering a squad (20 s): click a rival vehicle to attack its party, that vehicle first, or click the ground to post the squad there; right-click or the squad's number again puts the order away"
	}
	if s.laying.on {
		help = "laying a pipe: click the ground to bend it, click a ringed tank (silo, charger, core) to connect it, click the last node for its menu, right-click takes the last bend back"
	}
	if s.assigningRobot != 0 {
		help = fmt.Sprintf(
			"assigning robot #%d: click an oil pool or lilac vein; "+
				"right-click cancels",
			s.assigningRobot,
		)
	}
	if s.techPlacing != "" {
		name := catalogInfo(buildingType(s.techPlacing)).Name
		help = fmt.Sprintf(
			"placing %s: click valid ground to mark it; "+
				"right-click cancels",
			name,
		)
	}
	helpLines := techWrap(screen, help, screen.Width()-32, 13)
	for i, line := range helpLines {
		y := screen.Height() - 16*float32(len(helpLines)-i) - 10
		screen.DrawText(line, 16, y, 13, textColor, uiText)
	}
	if s.laying.on {
		s.drawLayingLabel(screen)
		if s.laying.menu {
			s.drawLayMenu(screen)
		}
	}
	if s.ordering != 0 {
		s.drawOrderingLabel(screen)
	}
	if s.picked && !s.radial && s.ordering == 0 && !s.robotsOpen {
		panel := s.inspectionPanel()
		panel.arm(s.armed)
		drawTooltip(screen, panel, s.mouse.X, s.mouse.Y)
	}
	if s.radial {
		drawRadial(s, screen, s.mouse.X, s.mouse.Y)
	}
	drawEdgeGuides(s, screen)
	drawRobotPanel(s, screen)
}

func statusTextSize() float32 {
	switch {
	case screenWidth < 750:
		return 20
	case screenWidth < 1000:
		return 18
	default:
		return 15
	}
}

// hudLine is the strip of stores and hands under the game's name, each
// store against the room the colony has for it, with the fog's cycle
// and the swell the forecast names. The words stay in the plain text
// color: the fog color would drown in the fog the screen is cleared
// with.
func (s *playScene) hudLine() string {
	fog := fmt.Sprintf("cycle %d", s.state.Fog.Cycle)
	switch {
	case s.state.Fog.SwellLeft > 0:
		fog += "   swell"
	case s.state.Fog.Pressure > 0:
		fog += "   swell easing"
	case s.state.Fog.NextIn <= 1:
		fog += "   swell next cycle"
	}
	if threat := threatWords(s.state); threat != "" {
		fog += "   [danger]" + threat + "[/]"
	}
	if techPending(s.state) != "" {
		fog += "   [light]schematics at the core[/]"
	}
	if s.saveFailed {
		fog += "   save failed"
	}
	return fmt.Sprintf("[oil]%s / %s[/]   [lilac]%s / %s[/]   [dim]%d robots[/]   %s",
		si(oilTotal(s.state), "L"), si(oilCap(s.state), "L"),
		si(s.state.Stock.Lilac, "kg"), si(lilacCap(s.state), "kg"),
		len(s.state.Robots), fog)
}
