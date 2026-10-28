package main

import (
	"math"

	"golib"
)

const unitPickSlackPx = 4

type unitSelection struct {
	kind ThingType
	id   int64
}

type worldUnitHit struct {
	selection unitSelection
	visible   golib.Rectangle
	hit       golib.Rectangle
	depth     float32
	layer     int
}

func (s *playScene) unitAtScreen(
	mx, my float32,
	enemiesOnly bool,
) (worldUnitHit, bool) {
	var best worldUnitHit
	found := false
	consider := func(hit worldUnitHit) {
		if !hit.hit.Contains(mx, my) {
			return
		}
		if !found || hit.drawsInFrontOf(best) {
			best, found = hit, true
		}
	}
	if !enemiesOnly {
		for _, id := range sortedRobotIDs(s.state) {
			r := s.state.Robots[id]
			gx, gy := project(float32(r.X), float32(r.Y))
			visible, ok := robotModel(r.Kind).screenBounds(
				s.camera, golib.Vector2{X: gx, Y: gy}, r.Facing, s.zoom,
			)
			if !ok {
				continue
			}
			consider(worldUnitHit{
				selection: unitSelection{kind: TypeRobot, id: r.ID},
				visible:   visible,
				hit:       expandUnitHit(visible),
				depth:     gy,
			})
		}
	}
	for _, id := range sortedEnemyIDs(s.state) {
		e := s.state.Enemies[id]
		if !selectableEnemyVehicle(e.Kind) {
			continue
		}
		gx, gy := project(float32(e.X), float32(e.Y))
		model, _ := rivalVehicleModel(e.Kind)
		visible, ok := model.screenBounds(
			s.camera, golib.Vector2{X: gx, Y: gy}, e.Facing, s.zoom,
		)
		if !ok {
			continue
		}
		consider(worldUnitHit{
			selection: unitSelection{kind: ThingType(e.Kind), id: e.ID},
			visible:   visible,
			hit:       expandUnitHit(visible),
			depth:     gy,
			layer:     1,
		})
	}
	return best, found
}

func selectableEnemyVehicle(kind EnemyKind) bool {
	switch kind {
	case EnemyScout, EnemyCrawler, EnemyRaider, EnemyArtillery,
		EnemyCityCrawler:
		return true
	}
	return false
}

func expandUnitHit(rect golib.Rectangle) golib.Rectangle {
	return golib.Rectangle{
		X:      rect.X - unitPickSlackPx,
		Y:      rect.Y - unitPickSlackPx,
		Width:  rect.Width + 2*unitPickSlackPx,
		Height: rect.Height + 2*unitPickSlackPx,
	}
}

func (hit worldUnitHit) drawsInFrontOf(other worldUnitHit) bool {
	if hit.layer != other.layer {
		return hit.layer > other.layer
	}
	if hit.depth != other.depth {
		return hit.depth > other.depth
	}
	return hit.selection.id > other.selection.id
}

func (s *playScene) hoveredUnit() (worldUnitHit, bool) {
	if s.unitPointerOverControl() {
		return worldUnitHit{}, false
	}
	if s.techPlacing != "" || s.laying.on || s.assigningRobot != 0 ||
		s.radial || s.dev.placing {
		return worldUnitHit{}, false
	}
	if s.ordering != 0 {
		return s.unitAtScreen(s.mouse.X, s.mouse.Y, true)
	}
	return s.unitAtScreen(s.mouse.X, s.mouse.Y, false)
}

func (s *playScene) unitPointerOverControl() bool {
	mx, my := s.mouse.X, s.mouse.Y
	if robotPanelButtonRect().Contains(mx, my) {
		return true
	}
	if s.robotsOpen && robotPanelRect().Contains(mx, my) {
		return true
	}
	if s.picked && s.ordering == 0 && !s.robotsOpen &&
		s.inspectionPanel().contains(mx, my) {
		return true
	}
	if s.techCallout != "" && s.techPlacing == "" &&
		techCalloutBounds(s).Contains(mx, my) {
		return true
	}
	if techPending(s.state) != "" && s.techBadgeHolds(mx, my) {
		return true
	}
	if devTitle.Contains(mx, my) {
		return true
	}
	if devOpen {
		for i := 0; i < 12; i++ {
			if devButtonBounds(i).Contains(mx, my) {
				return true
			}
		}
	}
	if _, ok := s.squadMarkAt(mx, my); ok {
		return true
	}
	for i := range squadSlots(s.state) {
		if i >= squadKeys {
			break
		}
		if squadBoxRect(i).Contains(mx, my) {
			return true
		}
	}
	return false
}

func (s *playScene) unitBounds(selection unitSelection) (golib.Rectangle, bool) {
	if selection.kind == TypeRobot {
		r, ok := s.state.Robots[selection.id]
		if !ok {
			return golib.Rectangle{}, false
		}
		gx, gy := project(float32(r.X), float32(r.Y))
		return robotModel(r.Kind).screenBounds(
			s.camera, golib.Vector2{X: gx, Y: gy}, r.Facing, s.zoom,
		)
	}
	e, ok := s.state.Enemies[selection.id]
	if !ok || ThingType(e.Kind) != selection.kind ||
		!selectableEnemyVehicle(e.Kind) {
		return golib.Rectangle{}, false
	}
	gx, gy := project(float32(e.X), float32(e.Y))
	model, _ := rivalVehicleModel(e.Kind)
	return model.screenBounds(
		s.camera, golib.Vector2{X: gx, Y: gy}, e.Facing, s.zoom,
	)
}

func (s *playScene) selectedUnitThing() (Thing, int, int, bool) {
	selection := s.pickedUnit
	if selection.id == 0 {
		return Thing{}, 0, 0, false
	}
	if selection.kind == TypeRobot {
		r, ok := s.state.Robots[selection.id]
		if !ok {
			return Thing{}, 0, 0, false
		}
		col, row := robotCell(r)
		return robotThing(s.state, r), col, row, true
	}
	e, ok := s.state.Enemies[selection.id]
	if !ok || ThingType(e.Kind) != selection.kind ||
		!selectableEnemyVehicle(e.Kind) {
		return Thing{}, 0, 0, false
	}
	return enemyThing(s.state, e),
		int(math.Floor(e.X / buildingCell)),
		int(math.Floor(e.Y / buildingCell)), true
}

func (s *playScene) syncPickedUnit() {
	if s.pickedUnit.id == 0 {
		return
	}
	thing, col, row, ok := s.selectedUnitThing()
	if !ok {
		s.picked = false
		s.pickedThing = ""
		s.pickedUnit = unitSelection{}
		s.pickedRobot = 0
		return
	}
	s.picked = true
	s.pickedThing = thing.ID
	s.pickedCol, s.pickedRow = col, row
}

func (s *playScene) selectUnit(hit worldUnitHit) {
	s.picked = true
	s.pickedSquad = 0
	s.pickedUnit = hit.selection
	s.pickedRobot = 0
	s.robotPage = 0
	s.armed = ""
	s.closeRadial()
	s.syncPickedUnit()
	s.au.ui(1)
}

func (s *playScene) clearPickedUnit() {
	s.pickedUnit = unitSelection{}
}

func drawUnitOutline(
	screen *golib.Screen,
	camera *golib.Camera,
	rect golib.Rectangle,
	color golib.Color,
) {
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	screen.SetCamera(nil)
	screen.DrawRectangleOutline(rect, 1, color)
	screen.SetCamera(camera)
}
