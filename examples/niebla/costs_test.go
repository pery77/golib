package main

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestPlacementsAndProductionRecordTheirCosts(t *testing.T) {
	s := newGame()
	s.Deliveries = 1
	col, row := groundNearCore()
	Apply(s, MarkBuilding{Kind: BuildingCharger, Col: col, Row: row})
	if len(s.Costs) != 1 {
		t.Fatalf("marking recorded %d expenses, want one", len(s.Costs))
	}
	x, y := cellCenterUnits(col, row)
	wantLilac, wantOil := buildingCost(BuildingCharger)
	got := s.Costs[0]
	if got.Source != costPlacement || got.ID != 0 ||
		got.X != x || got.Y != y || got.Lilac != wantLilac ||
		got.Oil != wantOil || got.Continuous {
		t.Errorf("placement receipt = %+v, want charger cost at its cell", got)
	}

	s.Costs = nil
	factoryID := s.NextID
	s.NextID++
	s.Buildings[factoryID] = Building{
		ID: factoryID, Kind: BuildingWarFactory, Col: col, Row: row,
	}
	s.Raids.LegacyRepairUnlocked = true
	Apply(s, QueueMechanic{Building: factoryID})
	if len(s.Costs) != 1 {
		t.Fatalf("mechanic production recorded %d expenses, want one", len(s.Costs))
	}
	x, y = cellCenterUnits(col, row)
	got = s.Costs[0]
	if got.Source != costBuilding || got.ID != factoryID ||
		got.X != x || got.Y != y || got.Lilac != mechanicCostLilac ||
		got.Oil != mechanicCostOil || got.Continuous {
		t.Errorf("production receipt = %+v, want mechanic cost at its factory", got)
	}
}

func TestMechanicRepairsRecordEveryCost(t *testing.T) {
	s := newGame()
	s.Stock = Stock{Oil: 500, Lilac: 1000}
	col, row := groundNearCore()
	buildingID := s.NextID
	s.NextID++
	s.Buildings[buildingID] = Building{
		ID: buildingID, Kind: BuildingGuard, Col: col, Row: row,
		Damage: 20,
	}
	mechanic := Robot{
		ID: 100, Kind: RobotRepair, X: 340, Y: 250, Tank: 50,
	}
	s.mend(buildingID, &mechanic)

	repair := repairPerSecond / 60
	wantLilac, wantRepairOil := buildingRepairCost(BuildingGuard, repair)
	wantMechanicOil := repair * repairOilPerPoint
	if len(s.Costs) != 2 {
		t.Fatalf("repair recorded %d expenses, want two", len(s.Costs))
	}
	buildingX, buildingY := cellCenterUnits(col, row)
	foundBuildingCost, foundMechanicCost := false, false
	for _, got := range s.Costs {
		switch got.Source {
		case costBuilding:
			foundBuildingCost = true
			if got.ID != buildingID || got.X != buildingX ||
				got.Y != buildingY ||
				math.Abs(got.Lilac-wantLilac) > 1e-9 ||
				math.Abs(got.Oil-wantRepairOil) > 1e-9 ||
				!got.Continuous {
				t.Errorf("building repair receipt = %+v", got)
			}
		case costRobot:
			foundMechanicCost = true
			if got.ID != mechanic.ID || got.X != mechanic.X ||
				got.Y != mechanic.Y ||
				math.Abs(got.Oil-wantMechanicOil) > 1e-9 ||
				got.Lilac != 0 || !got.Continuous {
				t.Errorf("mechanic fuel receipt = %+v", got)
			}
		default:
			t.Errorf("unexpected repair receipt = %+v", got)
		}
	}
	if !foundBuildingCost || !foundMechanicCost {
		t.Errorf("repair receipts include building=%v mechanic=%v",
			foundBuildingCost, foundMechanicCost)
	}
}

func TestOrdinaryRobotFuelUseHasNoCostReceipts(t *testing.T) {
	s := newGame()
	for _, kind := range []RobotKind{
		RobotBuilder, RobotWorker, RobotCombat, RobotRepair,
	} {
		r := Robot{
			ID: int64(kind[0]), Kind: kind, X: 100, Y: 100,
			Tank: robotTankLiters, Carry: 1, Cargo: TypeOil,
			PostCol: -1, PostRow: -1,
		}
		stepRobot(s, &r)
		if len(s.Costs) != 0 {
			t.Errorf("%s fuel burn recorded a cost: %+v", kind, s.Costs)
		}
		s.refill(&r)
		if len(s.Costs) != 0 {
			t.Errorf("%s refill recorded a cost: %+v", kind, s.Costs)
		}
	}
}

func TestContinuousCostsAppearAsOneSecondTotals(t *testing.T) {
	f := newSpendingField()
	event := CostReceipt{
		Source: costRobot, ID: 8, X: 20, Y: 30,
		Oil: 0.25, Continuous: true,
	}
	f.update([]CostReceipt{event}, 0.5)
	if len(f.numbers) != 0 {
		t.Fatal("a partial second produced a floating cost")
	}
	f.update([]CostReceipt{event}, 0.5)
	if len(f.numbers) != 1 {
		t.Fatalf("one second produced %d floating costs, want one", len(f.numbers))
	}
	if math.Abs(f.numbers[0].amount-0.5) > 1e-9 ||
		f.numbers[0].unit != "L" {
		t.Errorf("floating cost = %+v, want 0.5 L", f.numbers[0])
	}
}

func TestProtectorCostsAppearAsFourSecondTotals(t *testing.T) {
	f := newSpendingField()
	event := CostReceipt{
		Source: costProtector, ID: 8, X: 20, Y: 30,
		Oil: 0.125, Continuous: true,
	}
	for second := 0; second < 3; second++ {
		f.update([]CostReceipt{event}, 1)
		if len(f.numbers) != 0 {
			t.Fatalf("second %d produced a floating cost", second+1)
		}
	}

	f.update([]CostReceipt{event}, 1)
	if len(f.numbers) != 1 {
		t.Fatalf(
			"four seconds produced %d floating costs, want one",
			len(f.numbers),
		)
	}
	if math.Abs(f.numbers[0].amount-0.5) > 1e-9 ||
		f.numbers[0].unit != "L" {
		t.Errorf("floating cost = %+v, want 0.5 L", f.numbers[0])
	}
}

func TestSmallCostsKeepTheirPrecision(t *testing.T) {
	if got, want := spendingWords(0.25, "L"), "-0.25 L"; got != want {
		t.Errorf("small expense = %q, want %q", got, want)
	}
}

func TestWriteCostShotState(t *testing.T) {
	path := os.Getenv("NIEBLA_COST_SHOT_STATE")
	if path == "" {
		t.Skip("set NIEBLA_COST_SHOT_STATE to write a cost-effects shot state")
	}

	s := newGame()
	col, row := groundNearCore()
	s.raise(BuildingWarFactory, col+2, row)
	factory, _ := buildingAt(s, col+2, row)
	s.raise(BuildingGuard, col, row)
	guard, _ := buildingAt(s, col, row)
	guard.Damage = 40
	s.Buildings[guard.ID] = guard
	s.raise(BuildingProtector, col+4, row)
	protector, _ := buildingAt(s, col+4, row)
	protector.Oil = 100
	s.Buildings[protector.ID] = protector

	mechanicID := s.spawnRobot(
		RobotRepair,
		float64(col*buildingCell+buildingCell/2),
		float64(row*buildingCell+buildingCell/2),
	)
	mechanic := s.Robots[mechanicID]
	mechanic.Factory = factory.ID
	angle := float64(mechanic.ID) * goldenAngle
	mechanic.X += math.Cos(angle) * 11
	mechanic.Y += math.Sin(angle) * 11
	s.Robots[mechanic.ID] = mechanic

	data, err := json.MarshalIndent(map[string]any{"state": s}, "", "  ")
	if err != nil {
		t.Fatalf("the state doesn't marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
