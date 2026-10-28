package main

import (
	"image"
	"testing"

	"golib"
)

func TestWorldSpriteOpaqueBoundsCoverEveryFacing(t *testing.T) {
	models := []worldSprite{
		coreWorkerModel,
		carrierModel,
		trooperModel,
		mechanicModel,
		rivalScoutModel,
		rivalCrawlerModel,
		rivalCityCrawlerModel,
		rivalRaiderModel,
		rivalArtilleryModel,
	}
	for _, model := range models {
		for facing := uint8(0); facing < 8; facing++ {
			bounds, ok := model.opaqueBounds(facing)
			if !ok || bounds.Empty() {
				t.Fatalf("%s has no opaque bounds facing %d", model.name, facing)
			}
			frame := image.Rect(0, 0, modelFrameWidth, modelFrameHeight)
			if !bounds.In(frame) {
				t.Errorf("%s bounds %v leave frame %v", model.name, bounds, frame)
			}
			if bounds == frame {
				t.Errorf("%s bounds include the whole transparent frame", model.name)
			}
		}
	}
}

func TestUnitPickingUsesSpriteBodiesAndTheFrontmostLayer(t *testing.T) {
	s := newGame()
	noRivals(s)
	scene := newPlayScene(s)
	x, y := tileCenterUnits(coreCol+5, coreRow+2)
	robotID := s.spawnRobot(RobotWorker, x, y)
	enemyID := s.NextID
	s.NextID++
	s.Enemies[enemyID] = Enemy{
		ID: enemyID, Kind: EnemyCrawler,
		X: x, Y: y, Facing: facingUpRight,
		Health: enemySpecOf(EnemyCrawler).health,
	}

	if _, alive := s.Robots[robotID]; !alive {
		t.Fatal("the overlapping robot disappeared")
	}
	var visible golib.Rectangle
	for _, zoom := range []float32{1, 8, 32} {
		scene.zoom = zoom
		scene.camera.Zoom = zoom
		var ok bool
		visible, ok = scene.unitBounds(unitSelection{
			kind: TypeCrawler, id: enemyID,
		})
		if !ok {
			t.Fatalf("the crawler has no screen bounds at zoom %g", zoom)
		}
		point := visible.Center()
		scene.mouse = point
		hovered, over := scene.hoveredUnit()
		if !over || hovered.selection != (unitSelection{
			kind: TypeCrawler, id: enemyID,
		}) {
			t.Fatalf("at zoom %g hover is %+v, %v", zoom,
				hovered.selection, over)
		}
		hit, found := scene.unitAtScreen(point.X, point.Y, false)
		if !found || hit.selection != (unitSelection{
			kind: TypeCrawler, id: enemyID,
		}) {
			t.Fatalf("at zoom %g the frontmost hit is %+v, %v",
				zoom, hit.selection, found)
		}
		if _, found := scene.enemyUnder(point.X, point.Y); !found {
			t.Fatalf("the squad target picker missed the sprite at zoom %g", zoom)
		}
		if _, found := scene.enemyUnder(
			visible.X-unitPickSlackPx-2, point.Y,
		); found {
			t.Fatalf("the squad target picker reached outside the hitbox at zoom %g",
				zoom)
		}
	}
}

func TestResidentCityCrawlerIsPickedAsAVehicle(t *testing.T) {
	s := newGame()
	noRivals(s)
	cityID := s.foundCity(3500, 3200, 0.4)
	crawlerID := cityCrawlerID(s, s.Cities[cityID])
	scene := newPlayScene(s)

	for _, zoom := range []float32{1, 8, 32} {
		scene.zoom = zoom
		scene.camera.Zoom = zoom
		visible, ok := scene.unitBounds(unitSelection{
			kind: TypeCityCrawler,
			id:   crawlerID,
		})
		if !ok {
			t.Fatalf("the city crawler has no screen bounds at zoom %g", zoom)
		}
		point := visible.Center()
		hit, found := scene.unitAtScreen(point.X, point.Y, true)
		if !found || hit.selection != (unitSelection{
			kind: TypeCityCrawler,
			id:   crawlerID,
		}) {
			t.Fatalf("at zoom %g the city crawler hit is %+v, %v",
				zoom, hit.selection, found)
		}
	}
}

func TestPickedUnitCardFollowsTheUnitAndClosesWhenItDies(t *testing.T) {
	s := newGame()
	noRivals(s)
	scene := newPlayScene(s)
	x, y := tileCenterUnits(coreCol+5, coreRow+2)
	id := s.spawnRobot(RobotWorker, x, y)
	r := s.Robots[id]
	r.Health = 31
	s.Robots[id] = r

	visible, ok := scene.unitBounds(unitSelection{kind: TypeRobot, id: id})
	if !ok {
		t.Fatal("the worker has no screen bounds")
	}
	hit, found := scene.unitAtScreen(
		visible.Center().X, visible.Center().Y, false,
	)
	if !found || hit.selection.id != id {
		t.Fatalf("the worker was not under its sprite: %+v, %v", hit, found)
	}
	scene.selectUnit(hit)

	r.X += 4 * buildingCell
	r.Y += 3 * buildingCell
	s.Robots[id] = r
	scene.syncPickedUnit()
	wantCol, wantRow := robotCell(r)
	panel := scene.inspectionPanel()
	if panel.col != wantCol || panel.row != wantRow {
		t.Errorf("worker card is at %d,%d, want %d,%d",
			panel.col, panel.row, wantCol, wantRow)
	}
	var card Thing
	for _, row := range panel.rows {
		if row.title {
			card = row.thing
		}
	}
	if card.Type != TypeRobot || card.Ref != id {
		t.Errorf("worker card is %+v, want robot %d", card, id)
	}

	delete(s.Robots, id)
	scene.syncPickedUnit()
	if scene.picked || scene.pickedUnit.id != 0 {
		t.Error("the worker's card stayed open after it disappeared")
	}
}

func TestPickedRivalCardFollowsTheVehicleAndShowsHealth(t *testing.T) {
	s := newGame()
	noRivals(s)
	scene := newPlayScene(s)
	x, y := tileCenterUnits(coreCol+5, coreRow+2)
	id := s.NextID
	s.NextID++
	s.Enemies[id] = Enemy{
		ID: id, Kind: EnemyScout,
		X: x, Y: y, Health: 17,
	}

	visible, ok := scene.unitBounds(unitSelection{
		kind: TypeScout, id: id,
	})
	if !ok {
		t.Fatal("the scout has no screen bounds")
	}
	hit, found := scene.unitAtScreen(
		visible.Center().X, visible.Center().Y, true,
	)
	if !found || hit.selection.id != id {
		t.Fatalf("the scout was not under its sprite: %+v, %v", hit, found)
	}
	scene.selectUnit(hit)

	panel := scene.inspectionPanel()
	var health string
	for _, row := range panel.rows {
		if row.detail.Label == "health" {
			health = row.detail.Value
		}
	}
	if health != "17 / 60" {
		t.Errorf("scout health is %q, want 17 / 60", health)
	}

	e := s.Enemies[id]
	e.X += 5 * buildingCell
	e.Y += 2 * buildingCell
	s.Enemies[id] = e
	scene.syncPickedUnit()
	wantCol := int(e.X / buildingCell)
	wantRow := int(e.Y / buildingCell)
	panel = scene.inspectionPanel()
	if panel.col != wantCol || panel.row != wantRow {
		t.Errorf("scout card is at %d,%d, want %d,%d",
			panel.col, panel.row, wantCol, wantRow)
	}

	delete(s.Enemies, id)
	scene.syncPickedUnit()
	if scene.picked || scene.pickedUnit.id != 0 {
		t.Error("the scout's card stayed open after it disappeared")
	}
}
