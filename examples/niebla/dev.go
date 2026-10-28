package main

import (
	"fmt"
	"math"

	"golib"
)

// The dev tools: a strip of buttons under the HUD for whoever works on
// the game. Holding Control and clicking the game's name twice opens and
// closes it. The strip is view; what its buttons do goes through actions
// (DevHoldSwell, DevSpawnRobot, DevResetWorld, DevNextVisit, DevHurryRivals), like everything else that
// touches the state.

const (
	devDoubleClickTicks = 24 // updates between the two clicks, at the most

	devButtonWidth  = 120
	devButtonHeight = 18
	devButtonGap    = 8
	devStripX       = 16
	devStripY       = 68

	devSwellButton           = 0
	devRobotButton           = 1
	devResetButton           = 2 // another region, with a different seed
	devReplayButton          = 3 // the same region, dealt again
	devVisitButton           = 4 // the rivals' next visit, now
	devHurryButton           = 5 // a camped party stops waiting; it sits under the visit's
	devFastButton            = 6 // the game at devFastTicks a frame; under replay seed
	devTechButton            = 7
	devCityButton            = 8
	devBuildCityButton       = 9
	devFinishBattalionButton = 10
	devSendBattalionButton   = 11

	devFastTicks = 8 // ticks of simulation per update while fast forward is on
)

// devOpen outlives the play scene, so a trip to the menu keeps the strip.
var devOpen bool

// devTitle is where the game's name stands on the play scene's HUD.
var devTitle = golib.Rectangle{X: 16, Y: 12, Width: 84, Height: 28}

// devTools is the strip's view state.
type devTools struct {
	updates   int64
	clickedAt int64 // the update of the last Control click on the name
	placing   bool  // the next click on the region puts a robot there
	fast      bool  // the simulation runs devFastTicks to the update
	stripY    float32
}

// ticksPerUpdate returns how many ticks of simulation an update sends:
// one, or devFastTicks while the strip is open with fast forward on. The
// ticks are the same ones either way, so a game played fast is the same
// game.
func (d *devTools) ticksPerUpdate() int {
	if devOpen && d.fast {
		return devFastTicks
	}
	return 1
}

func devButtonBounds(index int) golib.Rectangle {
	const columns = 8
	return golib.Rectangle{
		X:      devStripX + float32(index%columns)*(devButtonWidth+devButtonGap),
		Y:      devStripY + float32(index/columns)*(devButtonHeight+4),
		Width:  devButtonWidth,
		Height: devButtonHeight,
	}
}

func (d *devTools) buttonBounds(index int) golib.Rectangle {
	button := devButtonBounds(index)
	button.Y += max(devStripY, d.stripY) - devStripY
	return button
}

// update handles the strip's clicks and reports whether it took this
// update's left click, so nothing under it acts on the same click.
func (d *devTools) update(s *playScene, input *golib.Input) bool {
	d.updates++
	if !devOpen {
		d.placing = false
	}
	if d.placing && input.MousePressed(golib.MouseRight) {
		d.placing = false
	}
	if !input.MousePressed(golib.MouseLeft) {
		return false
	}
	mx, my := input.MousePosition()
	control := input.KeyDown(golib.KeyLeftControl) ||
		input.KeyDown(golib.KeyRightControl)
	if control && devTitle.Contains(mx, my) {
		if d.clickedAt > 0 && d.updates-d.clickedAt <= devDoubleClickTicks {
			devOpen = !devOpen
			d.clickedAt = 0
		} else {
			d.clickedAt = d.updates
		}
		return true
	}
	if !devOpen {
		return false
	}
	if d.buttonBounds(devSwellButton).Contains(mx, my) {
		Apply(s.state, DevHoldSwell{On: !s.state.Fog.Held})
		return true
	}
	if d.buttonBounds(devRobotButton).Contains(mx, my) {
		d.placing = !d.placing
		return true
	}
	if d.buttonBounds(devResetButton).Contains(mx, my) {
		d.resetWorld(s, distinctWorldSeed(
			s.state.Seed,
			int64(golib.RandomInt(1, math.MaxInt32)),
		))
		return true
	}
	if d.buttonBounds(devReplayButton).Contains(mx, my) {
		d.resetWorld(s, s.state.Seed)
		return true
	}
	if d.buttonBounds(devVisitButton).Contains(mx, my) {
		Apply(s.state, DevNextVisit{})
		return true
	}
	if d.buttonBounds(devCityButton).Contains(mx, my) {
		Apply(s.state, DevNewCity{})
		return true
	}
	if d.buttonBounds(devHurryButton).Contains(mx, my) {
		Apply(s.state, DevHurryRivals{})
		return true
	}
	if d.buttonBounds(devFastButton).Contains(mx, my) {
		d.fast = !d.fast
		return true
	}
	if d.buttonBounds(devTechButton).Contains(mx, my) {
		Apply(s.state, DevNextTech{})
		return true
	}
	if d.buttonBounds(devBuildCityButton).Contains(mx, my) {
		Apply(s.state, DevFinishCityBuilding{})
		return true
	}
	if d.buttonBounds(devFinishBattalionButton).Contains(mx, my) {
		Apply(s.state, DevFinishCityBattalion{})
		return true
	}
	if d.buttonBounds(devSendBattalionButton).Contains(mx, my) {
		Apply(s.state, DevSendCityBattalion{})
		return true
	}
	if d.placing {
		world := s.camera.ToWorld(mx, my)
		x, y := unitsAtWorld(float64(world.X), float64(world.Y))
		Apply(s.state, DevSpawnRobot{X: x, Y: y})
		return true
	}
	return false
}

func distinctWorldSeed(previous, candidate int64) int64 {
	if candidate != previous {
		return candidate
	}
	return candidate%math.MaxInt32 + 1
}

// resetWorld deals the region again on a seed and saves it at once, so
// the base that was is gone from the database too. What the scene held
// of the old region - the picked cell, an open menu, a pipe in hand, the
// mites - goes with it.
func (d *devTools) resetWorld(s *playScene, seed int64) {
	Apply(s.state, DevResetWorld{Seed: seed})
	d.placing = false
	s.picked, s.pickedThing, s.armed = false, "", ""
	s.pickedSquad = 0
	s.pickedRobot, s.robotPage = 0, 0
	s.clearPickedUnit()
	s.closeRadial()
	s.laying = pipeLaying{}
	s.ordering = 0
	s.techPlacing = ""
	s.closeTechCallout()
	s.expanded = map[string]bool{}
	s.mites = newMiteField()
	s.fx = newFxField()
	s.costs = newSpendingField()
	s.saveNow()
}

// unitsAtWorld undoes project: the world units under a projected point.
func unitsAtWorld(px, py float64) (x, y float64) {
	ux, uy := unproject(float32(px), float32(py))
	return float64(ux), float64(uy)
}

func (d *devTools) draw(s *playScene, screen *golib.Screen) {
	if !devOpen {
		return
	}
	swell := "hold a swell"
	if s.state.Fog.Held {
		swell = "let the swell go"
	}
	robot := "place robots"
	if d.placing {
		robot = "placing: click ground"
	}
	labels := []string{
		devSwellButton:           swell,
		devRobotButton:           robot,
		devResetButton:           "reset world",
		devReplayButton:          "replay seed",
		devVisitButton:           "rivals: next visit",
		devHurryButton:           "rivals: stop waiting",
		devCityButton:            "rivals: new city",
		devFastButton:            fmt.Sprintf("fast forward x%d", devFastTicks),
		devTechButton:            "next schematics",
		devBuildCityButton:       "finish city build",
		devFinishBattalionButton: "finish battalion",
		devSendBattalionButton:   "send battalion",
	}
	lit := []bool{
		devSwellButton:           s.state.Fog.Held,
		devRobotButton:           d.placing,
		devResetButton:           false,
		devReplayButton:          false,
		devVisitButton:           len(s.state.Parties) > 0,
		devHurryButton:           false,
		devCityButton:            len(s.state.Cities) >= cityLimit,
		devFastButton:            d.fast,
		devTechButton:            false,
		devBuildCityButton:       false,
		devFinishBattalionButton: false,
		devSendBattalionButton:   false,
	}
	for i, label := range labels {
		rect := d.buttonBounds(i)
		fill := buttonColor
		if lit[i] || rect.Contains(s.mouse.X, s.mouse.Y) {
			fill = buttonHoverColor
		}
		screen.DrawRectangle(rect, fill)
		screen.DrawRectangleOutline(rect, 1, buttonEdgeColor)
		screen.DrawText(label, rect.X+8, rect.Y+3, 10, panelTextColor, uiText)
	}
	screen.DrawText(fmt.Sprintf("dev   seed %d", s.state.Seed),
		devTitle.X+devTitle.Width, devTitle.Y+10, 12, panelDimColor, uiText)
	if cityIDs := sortedCityIDs(s.state); len(cityIDs) > 0 {
		screen.DrawText(fmt.Sprintf("city %d (oldest)", cityIDs[0]),
			devTitle.X+devTitle.Width+102, devTitle.Y+11,
			10, panelDimColor, uiText)
	}
}
