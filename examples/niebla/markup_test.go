package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"golib"
)

func TestParseMarkupPlain(t *testing.T) {
	spans := parseMarkup("no colors here", panelTextColor)
	if len(spans) != 1 || spans[0].text != "no colors here" {
		t.Errorf("plain text parsed into %v, want one span", spans)
	}
	if spans[0].color != panelTextColor {
		t.Errorf("plain text took color %v, want the base one", spans[0].color)
	}
}

func TestParseMarkupColorsSpans(t *testing.T) {
	spans := parseMarkup("an [oil]oil pool[/] holds", panelTextColor)
	want := []markupSpan{
		{"an ", panelTextColor},
		{"oil pool", oilColor},
		{" holds", panelTextColor},
	}
	if len(spans) != len(want) {
		t.Fatalf("parsed %v, want %v", spans, want)
	}
	for i, w := range want {
		if spans[i] != w {
			t.Errorf("span %d is %v, want %v", i, spans[i], w)
		}
	}
}

func TestParseMarkupNests(t *testing.T) {
	spans := parseMarkup("[dim]a [oil]b[/] c[/]", panelTextColor)
	want := []markupSpan{
		{"a ", panelDimColor},
		{"b", oilColor},
		{" c", panelDimColor},
	}
	if len(spans) != len(want) {
		t.Fatalf("parsed %v, want %v", spans, want)
	}
	for i, w := range want {
		if spans[i] != w {
			t.Errorf("span %d is %v, want %v", i, spans[i], w)
		}
	}
}

func TestParseMarkupKeepsUnknownTags(t *testing.T) {
	spans := parseMarkup("[nope]x", panelTextColor)
	if len(spans) != 1 || spans[0].text != "[nope]x" {
		t.Errorf("an unknown tag parsed into %v, want it printed as it is", spans)
	}
}

func TestParseMarkupUnclosed(t *testing.T) {
	spans := parseMarkup("a [oil]b", panelTextColor)
	want := []markupSpan{
		{"a ", panelTextColor},
		{"b", oilColor},
	}
	if len(spans) != len(want) {
		t.Fatalf("parsed %v, want %v", spans, want)
	}
	for i, w := range want {
		if spans[i] != w {
			t.Errorf("span %d is %v, want %v", i, spans[i], w)
		}
	}
}

func TestTooltipLayoutRowsAndCards(t *testing.T) {
	s := newGame()
	camera := golib.NewCamera(float32(screenWidth), float32(screenHeight))
	coreCellCol, coreCellRow := tileCell(coreCol, coreRow)
	panel := tooltipLayout(s, camera, coreCellCol, coreCellRow, map[string]bool{})
	titles := 0
	for _, r := range panel.rows {
		if r.title {
			titles++
		}
	}
	if titles != 1 {
		t.Fatalf("a cell of the core laid %d card titles, want 1", titles)
	}
	open := map[string]bool{"core@12,12": true}
	panel = tooltipLayout(s, camera, coreCellCol, coreCellRow, open)
	details := 0
	for _, r := range panel.rows {
		if r.title {
			if r.summary != "r = 800 m" {
				t.Errorf("the core card's headline is %q, want \"r = 800 m\"", r.summary)
			}
		} else if !r.header && r.button == "" {
			details++
		}
	}
	if details != 9 {
		t.Errorf("the expanded core card shows %d details, want 9", details)
	}
	if !panel.contains(panel.x+1, panel.y+1) || panel.contains(panel.x-1, panel.y) {
		t.Errorf("contains answers wrongly around the panel's edges")
	}
	thing, ok := panel.cardAt(panel.x+10, panel.y+tooltipPad+textRow+titleRow/2)
	if !ok || thing.Type != TypeCore {
		t.Errorf("cardAt the title found %v, %v, want the core thing", thing, ok)
	}
	if _, ok := panel.cardAt(panel.x+10, panel.y+tooltipPad+textRow/2); ok {
		t.Errorf("cardAt the header found a card, want none")
	}
}

func TestPrimaryCardsStartOpen(t *testing.T) {
	camera := golib.NewCamera(float32(screenWidth), float32(screenHeight))
	s := newGame()
	addWorker(s)
	// The core's card starts open with no clicks: its details are there.
	coreCellCol, coreCellRow := tileCell(coreCol, coreRow)
	panel := tooltipLayout(s, camera, coreCellCol, coreCellRow, map[string]bool{})
	details := 0
	for _, r := range panel.rows {
		if !r.header && !r.title && r.button == "" {
			details++
		}
	}
	if details != 9 {
		t.Errorf("the core's card starts with %d details shown, want 9", details)
	}

	// A deposit's card starts open, button included, with no clicks.
	col, row, ok := nearestTileOf(kindOil)
	if !ok {
		t.Fatal("the region has no oil to test with")
	}
	oilCellCol, oilCellRow := tileCell(col, row)
	panel = tooltipLayout(s, camera, oilCellCol, oilCellRow, map[string]bool{})
	if panel.findButton(buttonSend) == nil {
		t.Error("the oil patch's card starts folded, want it open with its button")
	}

	// A robot alone on a cell starts open; the same robot on a deposit
	// starts folded, for the deposit is the card that opens.
	r := s.Robots[1]
	gcol, grow := groundNearCore()
	r.X, r.Y = cellCenterUnits(gcol, grow)
	s.Robots[1] = r
	panel = tooltipLayout(s, camera, gcol, grow, map[string]bool{})
	if len(panel.rows) < 3 {
		t.Fatal("the lone robot's card starts folded, want it open")
	}
	oilCol, oilRow, _ := nearestTileOf(kindOil)
	cx, cy := tileCenterUnits(oilCol, oilRow)
	r.X, r.Y = cx, cy
	s.Robots[1] = r
	oilCellCol, oilCellRow = robotCell(r)
	panel = tooltipLayout(s, camera, oilCellCol, oilCellRow, map[string]bool{})
	for _, thing := range panel.rows {
		if thing.title && thing.open && thing.thing.Type == TypeRobot {
			t.Error("the robot's card starts open beside a deposit, want it folded")
		}
	}
}

func TestTooltipOffersRobotButtons(t *testing.T) {
	camera := golib.NewCamera(float32(screenWidth), float32(screenHeight))
	s := newGame()
	addWorker(s)
	addWorker(s)
	col, row, ok := nearestTileOf(kindOil)
	if !ok {
		t.Fatal("the region has no oil to test with")
	}
	// The card belongs to the whole patch, so its ID carries the patch's
	// top corner tile, not the tile clicked.
	patch, _ := depositAt(col, row)
	open := map[string]bool{fmt.Sprintf("oil@%d,%d", patch.Col, patch.Row): true}
	cellCol, cellRow := tileCell(col, row)
	panel := tooltipLayout(s, camera, cellCol, cellRow, open)
	if thing, label, ok := panel.buttonAt(panel.x+1, panel.y+1); ok {
		t.Errorf("the panel's corner hit %v's %q button, want nothing", thing, label)
	}
	button := panel.findButton(buttonSend)
	if button == nil {
		t.Fatal("the expanded oil card offers no send button")
	}
	mid := golib.Vector2{X: button.bx + button.bw/2, Y: button.by + button.bh/2}
	if _, label, ok := panel.buttonAt(mid.X, mid.Y); !ok || label != buttonSend {
		t.Errorf("buttonAt its own button gave %q, %v", label, ok)
	}
	// Sending once leaves room for a second worker on the same patch.
	Apply(s, SendRobot{Col: col, Row: row})
	panel = tooltipLayout(s, camera, cellCol, cellRow, open)
	if panel.findButton(buttonSend) == nil {
		t.Error("the deposit stopped offering another available robot")
	}
	if panel.findButton(buttonRecall) != nil {
		t.Error("the deposit offers a group recall instead of individual cards")
	}
	worker := postRobots(s, col, row)[0]
	robotPanel := tooltipLayoutForRobot(
		s, camera, cellCol, cellRow, open, worker.ID,
	)
	if robotPanel.findButton(buttonRecall) == nil ||
		robotPanel.findButton(buttonBackToDeposit) == nil {
		t.Error("a worker's card lacks individual recall or return buttons")
	}
	portrait := panel.rows[0]
	for _, row := range panel.rows {
		if row.workers {
			portrait = row
			break
		}
	}
	if len(portrait.portraits) != 1 {
		t.Fatalf("the deposit shows %d portraits after one send, want 1",
			len(portrait.portraits))
	}
	mid = golib.Vector2{
		X: portrait.portraits[0].area.X + portrait.portraits[0].area.Width/2,
		Y: portrait.portraits[0].area.Y + portrait.portraits[0].area.Height/2,
	}
	if picked, ok := panel.robotAt(mid.X, mid.Y); !ok || picked.ID != worker.ID {
		t.Errorf("portrait hit returned %+v, %v, want robot %d", picked, ok, worker.ID)
	}
	for len(postRobots(s, col, row)) < portraitPageSize+1 {
		id := s.spawnRobot(RobotWorker, 0, 0)
		r := s.Robots[id]
		r.PostCol, r.PostRow = col, row
		s.Robots[id] = r
	}
	panel = tooltipLayout(s, camera, cellCol, cellRow, open)
	portrait = tooltipRow{}
	for _, row := range panel.rows {
		if row.workers {
			portrait = row
			break
		}
	}
	if portrait.pages != 2 || len(portrait.portraits) != portraitPageSize {
		t.Errorf("the robot portrait page has %d pages and %d portraits, want 2 and %d",
			portrait.pages, len(portrait.portraits), portraitPageSize)
	}
	_, label, ok := panel.buttonAt(
		portrait.nextPage.X+portrait.nextPage.Width/2,
		portrait.nextPage.Y+portrait.nextPage.Height/2,
	)
	if !ok || label != buttonPortraitNext {
		t.Errorf("the next-page control returned %q, %v", label, ok)
	}
	// A dry deposit has nobody to send.
	drained := newGame()
	drained.Drain[depositKey(patch)] = 0
	panel = tooltipLayout(drained, camera, cellCol, cellRow, open)
	if panel.findButton(buttonSend) != nil {
		t.Error("a dry deposit still offers a robot button")
	}
}

func TestWarFactoryCardWaitsForRepairProtocolBeforeOfferingMechanic(t *testing.T) {
	s := newGame()
	s.Stock = Stock{Oil: 1000, Lilac: 2500}
	col, row := groundNearCore()
	home := raised(t, s, BuildingWarFactory, col, row)
	camera := golib.NewCamera(float32(screenWidth), float32(screenHeight))
	hasMechanicDetails := func(panel tooltip) bool {
		for _, row := range panel.rows {
			if row.detail.Label == "mechanic cost" {
				return true
			}
		}
		return false
	}
	hasRepairPrice := func(panel tooltip) bool {
		for _, row := range panel.rows {
			if row.detail.Label == "repair price" {
				return true
			}
		}
		return false
	}
	panel := tooltipLayout(s, camera, col, row, map[string]bool{})
	if panel.findButton(buttonMechanic) != nil || hasMechanicDetails(panel) {
		t.Fatal("the war factory reveals a mechanic before its schematics arrive")
	}
	Apply(s, QueueMechanic{Building: home.ID})
	if s.Buildings[home.ID].Work > 0 ||
		s.Stock != (Stock{Oil: 1000, Lilac: 2500}) {
		t.Fatal("a locked mechanic order changed the factory or its stores")
	}

	s.Tech[techRepairID] = false
	panel = tooltipLayout(s, camera, col, row, map[string]bool{})
	mechanic := panel.findButton(buttonMechanic)
	if mechanic == nil || mechanic.disabled {
		t.Fatal("the repair protocol did not enable an affordable mechanic")
	}
	if !hasMechanicDetails(panel) || !hasRepairPrice(panel) {
		t.Fatal("the war factory did not show the repair cost details")
	}
	s.Stock.Lilac = mechanicCostLilac - 1
	panel = tooltipLayout(s, camera, col, row, map[string]bool{})
	mechanic = panel.findButton(buttonMechanic)
	if mechanic == nil || !mechanic.disabled {
		t.Fatal("the unaffordable mechanic button is missing or enabled")
	}
	if len(mechanic.costs) != 2 || !mechanic.costs[0].missing ||
		mechanic.costs[1].missing {
		t.Fatal("the mechanic price did not mark only the missing lilac")
	}
	if panel.buttonRowAt(
		mechanic.bx+mechanic.bw/2,
		mechanic.by+mechanic.bh/2,
	) != nil {
		t.Fatal("the unaffordable mechanic button still accepts clicks")
	}
	s.Stock.Lilac = 2500
	Apply(s, QueueMechanic{Building: home.ID})
	panel = tooltipLayout(s, camera, col, row, map[string]bool{})
	mechanic = panel.findButton(buttonMechanic)
	if mechanic == nil || !mechanic.disabled {
		t.Fatal("the mechanic button should stay visible and disabled while busy")
	}
	runTicks(s, mechanicBuildTicks)
	panel = tooltipLayout(s, camera, col, row, map[string]bool{})
	mechanic = panel.findButton(buttonMechanic)
	if mechanic == nil || !mechanic.disabled {
		t.Fatal("the mechanic button should stay disabled after filling its slot")
	}
	if trooper := panel.findButton(buttonTrooper); trooper == nil ||
		trooper.disabled {
		t.Fatal("a full mechanic slot should not disable trooper production")
	}
	stock := s.Stock
	s.Stock = Stock{}
	panel = tooltipLayout(s, camera, col, row, map[string]bool{})
	trooper := panel.findButton(buttonTrooper)
	if trooper == nil || !trooper.disabled || len(trooper.costs) != 2 ||
		!trooper.costs[0].missing || !trooper.costs[1].missing {
		t.Fatal("the unaffordable trooper doesn't show both shortfalls")
	}
	s.Stock = stock
	for i := 0; i < squadSize; i++ {
		id := s.spawnRobot(RobotCombat, 0, 0)
		trooper := s.Robots[id]
		trooper.Squad = home.ID
		s.Robots[id] = trooper
	}
	panel = tooltipLayout(s, camera, col, row, map[string]bool{})
	trooper = panel.findButton(buttonTrooper)
	if trooper == nil || !trooper.disabled {
		t.Fatal("the full squad's trooper button is missing or enabled")
	}
}

func TestRobotFactoryButtonsShowTheirCostsAndAvailability(t *testing.T) {
	s := newGame()
	col, row := groundNearCore()
	home := raised(t, s, BuildingFactory, col, row)
	camera := golib.NewCamera(float32(screenWidth), float32(screenHeight))
	panel := tooltipLayout(s, camera, col, row, map[string]bool{})
	for _, label := range []string{buttonBuildBuilder, buttonBuildWorker} {
		button := panel.findButton(label)
		if button == nil || button.disabled {
			t.Fatalf("the affordable %q button is missing or disabled", label)
		}
		if len(button.costs) != 2 || button.costs[0].missing ||
			button.costs[1].missing {
			t.Errorf("%q does not show both affordable resource costs", label)
		}
	}

	s.Stock.Oil = 0
	panel = tooltipLayout(s, camera, col, row, map[string]bool{})
	button := panel.findButton(buttonBuildWorker)
	if button == nil || !button.disabled {
		t.Fatal("the unaffordable worker button is missing or enabled")
	}
	if len(button.costs) != 2 || button.costs[0].missing ||
		!button.costs[1].missing {
		t.Fatal("the worker price did not mark only the missing oil")
	}
	if panel.buttonRowAt(
		button.bx+button.bw/2,
		button.by+button.bh/2,
	) != nil {
		t.Fatal("the unaffordable worker button still accepts clicks")
	}

	s.Stock.Oil = 1000
	Apply(s, QueueRobot{Building: home.ID, Kind: RobotBuilder})
	panel = tooltipLayout(s, camera, col, row, map[string]bool{})
	for _, label := range []string{buttonBuildBuilder, buttonBuildWorker} {
		button := panel.findButton(label)
		if button == nil || !button.disabled {
			t.Errorf("the busy factory's %q button is missing or enabled", label)
		}
	}
	runTicks(s, factoryRobotTicks)
	panel = tooltipLayout(s, camera, col, row, map[string]bool{})
	for _, label := range []string{buttonBuildBuilder, buttonBuildWorker} {
		button := panel.findButton(label)
		if button == nil || button.disabled {
			t.Errorf("the idle factory's %q button is missing or disabled", label)
		}
	}
}

func TestDamagedBuildingCardShowsItsRemainingRepairCost(t *testing.T) {
	s := newGame()
	col, row := groundNearCore()
	building := raised(t, s, BuildingGuard, col, row)
	building.Damage = buildingHealthPoints / 2
	s.Buildings[building.ID] = building

	camera := golib.NewCamera(float32(screenWidth), float32(screenHeight))
	panel := tooltipLayout(s, camera, col, row, map[string]bool{})
	wantLilac, wantOil := buildingRepairCost(
		building.Kind, building.Damage,
	)
	want := costWords(wantLilac, wantOil)
	for _, row := range panel.rows {
		if row.detail.Label != "repair cost left" {
			continue
		}
		if row.detail.Value != want {
			t.Fatalf("the repair cost is %q, want %q", row.detail.Value, want)
		}
		return
	}
	t.Fatal("the damaged building card did not show its remaining repair cost")
}

func TestWriteRepairCostShotState(t *testing.T) {
	path := os.Getenv("NIEBLA_REPAIR_COST_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_REPAIR_COST_SHOT_STATE to write a fixture")
	}
	s := newGame()
	s.Tech[techRepairID] = true
	s.Stock = Stock{Oil: 500, Lilac: 1000}
	col, row := groundNearCore()
	building := raised(t, s, BuildingGuard, col, row)
	building.Damage = buildingHealthPoints / 2
	s.Buildings[building.ID] = building

	scene := newPlayScene(s)
	gx, gy := projectBuilding(building)
	click := scene.camera.ToScreen(golib.Vector2{X: gx, Y: gy})
	t.Logf("damaged guard card click: %.0f,%.0f", click.X, click.Y)
	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatalf("the repair cost state does not marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
