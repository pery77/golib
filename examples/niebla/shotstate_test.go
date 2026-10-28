package main

import (
	"encoding/json"
	"os"
	"testing"

	"golib"
)

// TestWriteShotSquadState writes a region with two war factories and
// their troopers, the state a golib shot --save starts from: set
// NIEBLA_SHOT_STATE to the file to write and run the tests once, and
// the shot opens straight on the squads. Skipped otherwise, the way
// tests write nothing.
func TestWriteShotSquadState(t *testing.T) {
	path := os.Getenv("NIEBLA_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_SHOT_STATE to a file to write the squads' shot state to")
	}
	s := newGame()
	noRivals(s)
	home := squadOfTroopers(t, s, 4)
	col, row := groundNearCore()
	other := raised(t, s, BuildingWarFactory, col+2, row)
	x, y := cellCenterUnits(col+2, row)
	for i := 0; i < 2; i++ {
		r := s.Robots[s.spawnRobot(RobotCombat, x, y)]
		r.Squad = other.ID
		s.Robots[r.ID] = r
	}
	s.Squads[home.ID] = Squad{
		Home: home.ID, Order: OrderGuard,
		X: x - 400, Y: y + 300,
	}
	scene := newPlayScene(s)
	mark, _ := scene.squadMarkScreen(home.ID)
	panel := tooltipLayoutForSquad(scene, home.ID)
	if button := panel.findButton(buttonOrder); button != nil {
		t.Logf("squad mark: %.0f, %.0f", mark.X, mark.Y)
		t.Logf("give order button: %.0f, %.0f",
			button.bx+button.bw/2, button.by+button.bh/2,
		)
	}
	// The shot file is the store's shape: one saved name per key, and
	// resumeState reads the name "state".
	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatalf("the state doesn't marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func TestWriteShotUnitState(t *testing.T) {
	path := os.Getenv("NIEBLA_UNIT_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_UNIT_SHOT_STATE to write the units' shot state")
	}
	s := newGame()
	noRivals(s)
	x, y := parkCenter()
	workerID := s.spawnRobot(RobotWorker, x-200, y)
	s.spawnRobot(RobotCombat, x, y)
	col, row := groundNearCore()
	home := raised(t, s, BuildingWarFactory, col, row)
	silo := raised(t, s, BuildingSilo, col+2, row)
	s.hurtBuilding(silo.ID, 120)
	mechanicID := s.spawnRobot(RobotRepair, x+200, y)
	mechanic := s.Robots[mechanicID]
	mechanic.Factory = home.ID
	mechanic.Health = mechanicHealth * 0.6
	s.Robots[mechanicID] = mechanic
	scene := newPlayScene(s)
	worker := s.Robots[workerID]
	gx, gy := project(float32(worker.X), float32(worker.Y))
	if bounds, ok := robotModel(worker.Kind).screenBounds(
		scene.camera, golib.Vector2{X: gx, Y: gy}, worker.Facing, scene.zoom,
	); ok {
		center := bounds.Center()
		t.Logf("worker sprite center: %.0f,%.0f", center.X, center.Y)
	}
	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWriteShotVehicleState(t *testing.T) {
	path := os.Getenv("NIEBLA_VEHICLE_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_VEHICLE_SHOT_STATE to write the vehicles' shot state")
	}
	s := newGame()
	noRivals(s)
	x, y := parkCenter()
	kinds := []EnemyKind{
		EnemyScout, EnemyCrawler, EnemyRaider, EnemyArtillery,
		EnemyArtillery, EnemyArtillery, EnemyArtillery, EnemyArtillery,
	}
	for i, kind := range kinds {
		id := s.NextID
		s.NextID++
		column, row := float64(i%4), float64(i/4)
		s.Enemies[id] = Enemy{
			ID: id, Kind: kind,
			X: x + 32 + column*36, Y: y - 30 + row*74,
			Facing: uint8(i), Health: enemySpecOf(kind).health,
			Oil: 10,
		}
	}
	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWriteUnitDeathShotState(t *testing.T) {
	path := os.Getenv("NIEBLA_UNIT_DEATH_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_UNIT_DEATH_SHOT_STATE to write unit death shots")
	}
	s := newGame()
	noRivals(s)

	x, y := parkCenter()
	robotID := s.spawnRobot(RobotRepair, x-45, y+30)
	robot := s.Robots[robotID]
	robot.Health = 1
	s.Robots[robot.ID] = robot
	s.fire(Shot{
		Kind: ShotBullet, FromX: robot.X, FromY: robot.Y,
		ToX: robot.X, ToY: robot.Y,
		Robot: robot.ID, Damage: 2, Rival: true,
	})

	id := s.NextID
	s.NextID++
	e := Enemy{
		ID: id, Kind: EnemyArtillery,
		X: x + 65, Y: y + 30,
		Health: 1,
	}
	s.Enemies[id] = e
	s.fire(Shot{
		Kind: ShotBullet, FromX: e.X, FromY: e.Y,
		ToX: e.X, ToY: e.Y, Enemy: id, Damage: 2,
	})
	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWriteShotMovingArtilleryState(t *testing.T) {
	path := os.Getenv("NIEBLA_MOVING_ARTILLERY_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_MOVING_ARTILLERY_SHOT_STATE to write a moving artillery state")
	}
	s := newGame()
	noRivals(s)
	x, y := parkCenter()
	partyID := s.NextID
	s.NextID++
	id := s.NextID
	s.NextID++
	s.Parties[partyID] = Party{
		ID: partyID, Stage: StageApproach,
		CampX: x + 400, CampY: y - 400,
	}
	s.Enemies[id] = Enemy{
		ID: id, Kind: EnemyArtillery, Party: partyID,
		X: x, Y: y, Facing: facingRight,
		Health: enemySpecOf(EnemyArtillery).health,
	}
	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWriteShellTrailShotState(t *testing.T) {
	path := os.Getenv("NIEBLA_SHELL_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_SHELL_SHOT_STATE to write a shell shot state")
	}
	s := newGame()
	noRivals(s)
	const col, row = 100, 100
	raised(t, s, BuildingArtillery, col, row)
	fromX, fromY := cellCenterUnits(col, row)
	toX, toY := fromX+300, fromY-300
	muzzleX, muzzleY := shellLaunchPoint(fromX, fromY, toX, toY)
	s.fire(Shot{
		Kind:  ShotShell,
		FromX: muzzleX, FromY: muzzleY,
		ToX: toX, ToY: toY,
		Damage: artilleryShellDamage,
	})
	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWriteRobotPortraitShotState(t *testing.T) {
	path := os.Getenv("NIEBLA_ROBOT_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_ROBOT_SHOT_STATE to write the robot portrait shot state")
	}
	s := newGame()
	noRivals(s)
	col, row, ok := nearestTileOf(kindLilac)
	if !ok {
		t.Fatal("the region has no lilac for the robot portrait shot")
	}
	camera := golib.NewCamera(float32(screenWidth), float32(screenHeight))
	camera.Bounds = regionOnScreen()
	camera.Snap()
	patchX, patchY := tileCenterUnits(col, row)
	sx, sy := project(float32(patchX), float32(patchY))
	point := camera.ToScreen(golib.Vector2{X: sx, Y: sy})
	t.Logf("click the lilac patch at %.0f, %.0f", point.X, point.Y)
	world := camera.ToWorld(point.X, point.Y)
	cellCol, cellRow, _ := cellAtWorld(float64(world.X), float64(world.Y))
	parkX, parkY := parkCenter()
	workers := 0
	for _, robot := range s.Robots {
		if robot.Kind == RobotWorker {
			workers++
		}
	}
	for workers < 10 {
		s.spawnRobot(RobotWorker, parkX, parkY)
		workers++
	}
	for i := 0; i < 10; i++ {
		Apply(s, SendRobot{Col: col, Row: row})
	}
	panel := tooltipLayout(
		s, camera, cellCol, cellRow, map[string]bool{},
	)
	for _, row := range panel.rows {
		if !row.workers || len(row.portraits) == 0 {
			continue
		}
		portrait := row.portraits[0]
		t.Logf("click robot %d portrait at %.0f, %.0f",
			portrait.robot.ID,
			portrait.area.X+portrait.area.Width/2,
			portrait.area.Y+portrait.area.Height/2,
		)
		if row.pages > 1 {
			t.Logf("click next portrait page at %.0f, %.0f",
				row.nextPage.X+row.nextPage.Width/2,
				row.nextPage.Y+row.nextPage.Height/2,
			)
		}
		break
	}
	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestWriteShotRadialState writes the states the build menu's shots
// start from: NIEBLA_RADIAL_SHOT_STATE for the washed ring (the stores
// can pay nothing) and NIEBLA_RADIAL_FULL_SHOT_STATE for the paid one.
// Skipped otherwise, the way tests write nothing.
func TestWriteShotRadialState(t *testing.T) {
	washed := os.Getenv("NIEBLA_RADIAL_SHOT_STATE")
	full := os.Getenv("NIEBLA_RADIAL_FULL_SHOT_STATE")
	if washed == "" && full == "" {
		t.Skip("set NIEBLA_RADIAL_SHOT_STATE / NIEBLA_RADIAL_FULL_SHOT_STATE" +
			" to write the build menu's shot states")
	}
	write := func(path string, stock Stock) {
		s := newGame()
		noRivals(s)
		arriveAll(s)
		s.Stock = stock
		data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if washed != "" {
		write(washed, Stock{Oil: 300, Lilac: 40})
	}
	if full != "" {
		write(full, Stock{Oil: 600, Lilac: 1200})
	}
}

func TestWriteBuildingCollapseShotState(t *testing.T) {
	path := os.Getenv("NIEBLA_BUILDING_COLLAPSE_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_BUILDING_COLLAPSE_SHOT_STATE to write a collapse shot state")
	}
	s := newGame()
	noRivals(s)
	col, row := groundNearCore()
	building := raised(t, s, BuildingFactory, col, row)
	building.Damage = buildingHealth(building.Kind) - 1
	s.Buildings[building.ID] = building
	x, y := cellCenterUnits(col, row)
	s.fire(Shot{
		Kind: ShotBullet, FromX: x - 9, FromY: y,
		ToX: x, ToY: y, Building: building.ID,
		Damage: 2, Rival: true,
	})
	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
