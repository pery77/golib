package main

import (
	"encoding/json"
	"os"
	"testing"

	"golib"
)

func TestRobotPanelGroupsWorkByRoleAndDeposit(t *testing.T) {
	s := newGame()
	oilCol, oilRow, ok := nearestTileOf(kindOil)
	if !ok {
		t.Fatal("the region has no oil")
	}
	lilacCol, lilacRow, ok := nearestTileOf(kindLilac)
	if !ok {
		t.Fatal("the region has no lilac")
	}
	workerID := addWorker(s)
	lilacID := addWorker(s)
	freeID := addWorker(s)
	Apply(s, AssignRobot{ID: 1, Col: oilCol, Row: oilRow})
	Apply(s, AssignRobot{ID: workerID, Col: oilCol, Row: oilRow})
	Apply(s, AssignRobot{ID: lilacID, Col: lilacCol, Row: lilacRow})
	mechanicID := s.spawnRobot(RobotRepair, 0, 0)
	mechanic := s.Robots[mechanicID]
	mechanic.Factory = 500
	s.Robots[mechanicID] = mechanic

	groups := robotPanelGroups(s)
	if len(groups) != 5 {
		t.Fatalf("the roster has %d groups, want builders, available workers, two deposits and mechanics",
			len(groups))
	}
	if groups[0].title != "BUILDERS · 1" ||
		groups[0].robots[0].ID != 1 {
		t.Errorf("the builder group is %+v", groups[0])
	}
	if groups[1].title != "UNASSIGNED WORKERS · 1" ||
		groups[1].robots[0].ID != freeID {
		t.Errorf("the available worker group is %+v", groups[1])
	}
	if groups[2].title[:3] != "OIL" ||
		len(groups[2].robots) != 1 || groups[2].robots[0].ID != workerID {
		t.Errorf("oil group is %+v", groups[2])
	}
	if groups[3].title[:5] != "LILAC" ||
		len(groups[3].robots) != 1 || groups[3].robots[0].ID != lilacID {
		t.Errorf("lilac group is %+v", groups[3])
	}
	if groups[4].title != "MECHANICS · 1" ||
		groups[4].robots[0].ID != mechanicID {
		t.Errorf("the mechanic group is %+v", groups[4])
	}
	if got := robotPanelActivity(s, s.Robots[workerID]); got != "mining oil" {
		t.Errorf("assigned worker activity is %q, want mining oil", got)
	}
}

func TestRobotPanelPagesStayInsideTheirPlate(t *testing.T) {
	state := newGame()
	for i := 0; i < 11; i++ {
		addWorker(state)
	}
	scene := newPlayScene(state)
	scene.robotsPicked = 2
	layout := robotPanelLayoutFor(scene)
	if layout.pages != 3 || len(layout.rows) != robotPanelPageSize {
		t.Fatalf("first page has %d rows over %d pages, want 5 over 3",
			len(layout.rows), layout.pages)
	}
	if !layout.hasSelected || layout.selected.ID != 2 {
		t.Fatal("the selected worker was omitted from the panel details")
	}
	for _, row := range layout.rows {
		if !layout.box.Contains(row.area.X, row.area.Y) ||
			!layout.box.Contains(
				row.area.X+row.area.Width-1,
				row.area.Y+row.area.Height-1,
			) {
			t.Errorf("robot row escaped the plate: %+v", row.area)
		}
	}
	scene.robotsPage = 2
	layout = robotPanelLayoutFor(scene)
	if len(layout.rows) != 2 {
		t.Fatalf("last page has %d rows, want 2", len(layout.rows))
	}
	if layout.close.Width == 0 || layout.assign.Width == 0 {
		t.Fatal("the panel has no close or assignment control")
	}
	trigger := robotPanelButtonRect()
	if squadBoxRect(0).Overlaps(trigger) {
		t.Fatal("the robot control overlaps the military squad strip")
	}
	if trigger.Y+trigger.Height >= robotPanelRect().Y {
		t.Fatal("the robot control overlaps its open panel")
	}
}

func TestRobotPanelButtonAndCloseTargetsAreClickableAreas(t *testing.T) {
	s := newPlayScene(newGame())
	layout := robotPanelLayoutFor(s)
	button := robotPanelButtonRect()
	if !button.Contains(button.Center().X, button.Center().Y) {
		t.Fatal("the roster button does not contain its center")
	}
	if !layout.close.Contains(layout.close.Center().X, layout.close.Center().Y) {
		t.Fatal("the close control does not contain its center")
	}
	if layout.box.Contains(layout.box.X-1, layout.box.Y) {
		t.Fatal("the panel contains a point outside its left edge")
	}
	if layout.pages != 1 || len(layout.rows) != 1 {
		t.Fatalf("a fresh roster has %d rows over %d pages, want one",
			len(layout.rows), layout.pages)
	}
}

func TestRosterOrderAssignsTheChosenRobotAtTheClickedDeposit(t *testing.T) {
	scene := newPlayScene(newGame())
	scene.assigningRobot = 1
	deposit := safePool(t)
	x, y := cellCenterUnits(deposit.HeartCol, deposit.HeartRow)
	sx, sy := project(float32(x), float32(y))
	point := scene.camera.ToScreen(golib.Vector2{X: sx, Y: sy})
	if !scene.assignRobotAtScreen(point.X, point.Y) {
		t.Fatal("the clicked pool did not take the selected builder")
	}
	builder := scene.state.Robots[1]
	col, row := cellTile(deposit.HeartCol, deposit.HeartRow)
	if !builder.hasPost() || builder.PostCol != col ||
		builder.PostRow != row {
		t.Fatalf("screen assignment gave the builder post %d,%d",
			builder.PostCol, builder.PostRow)
	}
}

func TestWriteRobotRosterShotState(t *testing.T) {
	path := os.Getenv("NIEBLA_ROBOT_ROSTER_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_ROBOT_ROSTER_SHOT_STATE to write the roster shot")
	}
	s := newGame()
	oilCol, oilRow, ok := nearestTileOf(kindOil)
	if !ok {
		t.Fatal("the region has no oil for the roster fixture")
	}
	lilacCol, lilacRow, ok := nearestTileOf(kindLilac)
	if !ok {
		t.Fatal("the region has no lilac for the roster fixture")
	}
	oilWorker := addWorker(s)
	lilacWorker := addWorker(s)
	addWorker(s)
	Apply(s, AssignRobot{ID: oilWorker, Col: oilCol, Row: oilRow})
	Apply(s, AssignRobot{ID: lilacWorker, Col: lilacCol, Row: lilacRow})
	mechanicID := s.spawnRobot(RobotRepair, 0, 0)
	mechanic := s.Robots[mechanicID]
	mechanic.Factory = 999
	s.Robots[mechanicID] = mechanic
	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatalf("the roster state does not marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
