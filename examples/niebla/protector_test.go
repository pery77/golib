package main

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"golib"
)

func TestProtectorUpkeepAndRadiusFade(t *testing.T) {
	s := newGame()
	col, row := groundInTheFog()
	b := raised(t, s, BuildingProtector, col, row)
	centerX, centerY := cellCenterUnits(col, row)

	if b.Oil != protectorCostOil {
		t.Fatalf("a new protector holds %v L, want its initial charge of %v L",
			b.Oil, protectorCostOil)
	}
	if got := protectorRadiusTiles(b); got != protectorBubbleTiles {
		t.Fatalf("a charged protector has radius %v tiles, want %v",
			got, protectorBubbleTiles)
	}

	b.Oil = protectorOilCap * protectorRadiusFadeBelow / 2
	s.Buildings[b.ID] = b
	if got := protectorRadiusTiles(b); math.Abs(got-protectorBubbleTiles/2) > 1e-9 {
		t.Errorf("half of the fade reserve gives radius %v tiles, want %v",
			got, protectorBubbleTiles/2)
	}
	if !inSafeZone(s, centerX+protectorBubbleTiles*unitsPerTile/2, centerY) {
		t.Error("the reduced bubble doesn't shelter a point inside its radius")
	}
	if inSafeZone(s, centerX+protectorBubbleTiles*unitsPerTile, centerY) {
		t.Error("the reduced bubble still shelters a point beyond its radius")
	}

	b.Oil = 0
	s.Buildings[b.ID] = b
	if protectorRadiusTiles(b) != 0 || inSafeZone(s, centerX, centerY) {
		t.Error("an empty protector still has a bubble")
	}
	b.Oil = protectorOilPerSecond / 120
	s.Buildings[b.ID] = b
	Apply(s, Tick{})
	if got := s.Buildings[b.ID].Oil; got != 0 {
		t.Errorf("upkeep left an exhausted protector at %v L", got)
	}

	b.Oil = 30
	s.Buildings[b.ID] = b
	Apply(s, Tick{})
	if got := s.Buildings[b.ID].Oil;
		math.Abs(got-(30-protectorOilPerSecond/60)) > 1e-9 {
		t.Errorf("one tick leaves %v L, want %v", got,
			30-protectorOilPerSecond/60)
	}
	if len(s.Costs) != 1 || s.Costs[0].Source != costProtector ||
		math.Abs(s.Costs[0].Oil-protectorOilPerSecond/60) > 1e-9 {
		t.Errorf("protector cost receipts = %+v", s.Costs)
	}
}

func TestDemolishingProtectorDropsItsFuelOnlyOnce(t *testing.T) {
	s := newGame()
	col, row := groundNearCore()
	b := raised(t, s, BuildingProtector, col, row)
	b.Oil = 27
	s.Buildings[b.ID] = b

	demolishNow(t, s, b.ID)
	p, ok := pileAt(s, col, row)
	if !ok || p.Oil != b.Oil {
		t.Fatalf("demolition left %v L in its pile, want the held %v L",
			p.Oil, b.Oil)
	}
}

func TestProtectorOilIsDedicatedAndRobotsCanRefillIt(t *testing.T) {
	s := newGame()
	s.Stock.Oil = 0
	col, row := groundNearCore()
	b := raised(t, s, BuildingProtector, col, row)
	if oilTotal(s) != 0 || oilCap(s) != coreOilCap {
		t.Fatalf("the protector reserve is spendable: oil %v / %v",
			oilTotal(s), oilCap(s))
	}
	s.payOil(1)
	if got := s.Buildings[b.ID].Oil; got != protectorCostOil {
		t.Fatalf("colony spending took %v L from the protector reserve",
			protectorCostOil-got)
	}

	s.Stock.Oil = coreOilCap
	b.Oil = 0
	s.Buildings[b.ID] = b
	r := s.Robots[1]
	r.Carry, r.Cargo = 20, TypeOil
	r.X, r.Y = storeSpot(s, r)
	s.Robots[r.ID] = r
	if target := haulTank(s, r); target != b.ID {
		t.Fatalf("the robot chose tank %d instead of the empty protector %d",
			target, b.ID)
	}
	Apply(s, Tick{})
	if got := s.Buildings[b.ID].Oil; got < 19.9 {
		t.Fatalf("the robot left the protector at %v L, want its oil delivered",
			got)
	}
	if oilTotal(s) != coreOilCap {
		t.Errorf("the delivered reserve changed spendable oil to %v L",
			oilTotal(s))
	}
}

func TestProtectorCanBeFedByPipe(t *testing.T) {
	s := newGame()
	arriveAll(s)
	col, row := groundInTheFog()
	b := raised(t, s, BuildingProtector, col, row)
	b.Oil = 0
	s.Buildings[b.ID] = b
	s.Stock.Oil = coreOilCap

	if _, ok := canLayPipe(s, coreTank, b.ID, nil); !ok {
		t.Fatal("the core can't connect a pipe to a protector")
	}
	Apply(s, LayPipe{From: coreTank, To: b.ID})
	p, ok := pipeOut(s, coreTank)
	if !ok || p.To != b.ID {
		t.Fatal("the protector refused a pipe from the core")
	}
	p.Left = 0
	s.Pipes[p.ID] = p

	before := s.Stock.Oil
	runTicks(s, 60)
	if got := s.Buildings[b.ID].Oil; got <= 0 {
		t.Fatal("the pipe didn't deliver oil to the protector")
	}
	if s.Stock.Oil >= before {
		t.Error("the pipe didn't draw oil out of the core")
	}
	cx, cy := cellCenterUnits(col, row)
	if !inSafeZone(s, cx, cy) {
		t.Error("the bubble didn't recover when its pipe delivered oil")
	}

	other := raised(t, s, BuildingSilo, col+2, row)
	if !canJoin(s, b.ID, other.ID) {
		t.Error("a protector can't connect its oil tank to another building")
	}
}

func TestOldSavesSeedProtectorOilOnce(t *testing.T) {
	s := newGame()
	col, row := groundInTheFog()
	b := raised(t, s, BuildingProtector, col, row)
	b.Oil = 0
	s.Buildings[b.ID] = b
	s.Version = 0

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	delete(fields, "Version")
	data, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	var old State
	if err := json.Unmarshal(data, &old); err != nil {
		t.Fatal(err)
	}

	old.enterRegion()
	if got := old.Buildings[b.ID].Oil; got != protectorCostOil {
		t.Fatalf("an old protector starts with %v L, want %v",
			got, protectorCostOil)
	}
	if old.Version != stateVersion {
		t.Errorf("the migrated save has version %d, want %d",
			old.Version, stateVersion)
	}
	old.enterRegion()
	if got := old.Buildings[b.ID].Oil; got != protectorCostOil {
		t.Errorf("a second load changed the migrated charge to %v L", got)
	}
}

func TestWriteProtectorShotState(t *testing.T) {
	path := os.Getenv("NIEBLA_PROTECTOR_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_PROTECTOR_SHOT_STATE to write a pylon shot state")
	}
	s := newGame()
	noRivals(s)
	arriveAll(s)
	const col, row = 100, 60
	b := raised(t, s, BuildingProtector, col, row)
	b.Oil = protectorOilCap * protectorRadiusFadeBelow / 2
	s.Buildings[b.ID] = b

	camera := golib.NewCamera(float32(screenWidth), float32(screenHeight))
	camera.Bounds = regionOnScreen()
	camera.Zoom = zoomOfStop(zoomOut)
	camera.Snap()
	x, y := cellCenterUnits(col, row)
	px, py := project(float32(x), float32(y))
	point := camera.ToScreen(golib.Vector2{X: px, Y: py})
	t.Logf("protector cell click: %0.0f,%0.0f", point.X, point.Y)

	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatalf("the state doesn't marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
