package main

import (
	"encoding/json"
	"math"
	"testing"

	"golib"
)

func TestFacingFromMovementUsesScreenOctants(t *testing.T) {
	for want := uint8(0); want < 8; want++ {
		angle := float64(want) * math.Pi / 4
		screenX, screenY := math.Cos(angle), math.Sin(angle)
		projectedX := 2 * screenX / float64(unitW)
		projectedY := 2 * screenY / float64(unitH)
		dx := (projectedX + projectedY) / 2
		dy := (projectedY - projectedX) / 2

		if got := facingFromMovement(dx, dy); got != want {
			t.Errorf("screen octant %d maps to %d", want, got)
		}
	}
}

func TestGroundFacingProjectsAWorldYaw(t *testing.T) {
	center := golib.Vector2{X: 300, Y: 200}
	for facing := uint8(0); facing < 8; facing++ {
		tip := groundFacingPoint(center, 1, 0, unitW, facing)
		got := tip.Sub(center).Angle()
		want := facingAngle(facing)
		difference := math.Abs(float64(got - want))
		if difference > 180 {
			difference = 360 - difference
		}
		if difference > 0.01 {
			t.Errorf("facing %d projects at %.2f degrees, want %.2f",
				facing, got, want)
		}
	}
}

func TestGroundFacingUsesTheGroundAspectRatio(t *testing.T) {
	center := golib.Vector2{X: 300, Y: 200}
	forward := groundFacingPoint(center, 1, 0, unitW, facingRight).
		Sub(center)
	side := groundFacingPoint(center, 0, 1, unitW, facingRight).
		Sub(center)
	if math.Abs(float64(forward.X-unitW/float32(math.Sqrt2))) > 0.001 {
		t.Errorf("forward ground offset is %v, want %.3f", forward,
			unitW/float32(math.Sqrt2))
	}
	if math.Abs(float64(forward.Y)) > 0.001 {
		t.Errorf("forward ground offset has screen Y %.3f", forward.Y)
	}
	if math.Abs(float64(side.X)) > 0.001 {
		t.Errorf("side ground offset has screen X %.3f", side.X)
	}
	if math.Abs(float64(side.Y-unitH/float32(math.Sqrt2))) > 0.001 {
		t.Errorf("side ground offset is %v, want %.3f", side,
			unitH/float32(math.Sqrt2))
	}
}

func TestRobotFacingChangesWithMovementAndWaitsAtRest(t *testing.T) {
	s := newGame()
	r := Robot{X: 1000, Y: 1000, Facing: facingDown}

	if r.walkTowards(s, 1010, 990) {
		t.Fatal("robot arrived after one short step")
	}
	if r.Facing != facingRight {
		t.Fatalf("robot faces %d while moving right, want %d",
			r.Facing, facingRight)
	}

	if !r.walkTowards(s, r.X, r.Y) {
		t.Fatal("robot did not arrive at its current position")
	}
	if r.Facing != facingRight {
		t.Errorf("stationary robot turned to %d", r.Facing)
	}

	r.walkTowards(s, 1000, 1000)
	if r.Facing != facingLeft {
		t.Errorf("robot faces %d while moving left, want %d",
			r.Facing, facingLeft)
	}
}

func TestRivalFacingChangesWithMovementAndWaitsAtRest(t *testing.T) {
	s := &State{Enemies: map[int64]Enemy{}}
	e := Enemy{
		ID: 1, Kind: EnemyScout, X: 100, Y: 100,
		Facing: facingDown,
	}

	if s.driveParty([]Enemy{e}, 110, 90) {
		t.Fatal("vehicle arrived after one short step")
	}
	e = s.Enemies[e.ID]
	if e.Facing != facingRight {
		t.Fatalf("vehicle faces %d while moving right, want %d",
			e.Facing, facingRight)
	}

	s.driveParty([]Enemy{e}, e.X, e.Y)
	e = s.Enemies[e.ID]
	if e.Facing != facingRight {
		t.Errorf("stationary vehicle turned to %d", e.Facing)
	}

	s.driveParty([]Enemy{e}, 100, 100)
	e = s.Enemies[e.ID]
	if e.Facing != facingLeft {
		t.Errorf("vehicle faces %d while moving left, want %d",
			e.Facing, facingLeft)
	}
}

func TestFacingSavesAndDefaultsForLegacyStates(t *testing.T) {
	s := &State{
		Robots: map[int64]Robot{
			1: {ID: 1, Kind: RobotBuilder, Facing: facingUpLeft},
		},
		Enemies: map[int64]Enemy{
			2: {ID: 2, Kind: EnemyScout, Facing: facingDownRight},
		},
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var saved State
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Robots[1].Facing != facingUpLeft ||
		saved.Enemies[2].Facing != facingDownRight {
		t.Fatalf("facing changed after save: robot %d, vehicle %d",
			saved.Robots[1].Facing, saved.Enemies[2].Facing)
	}

	const legacy = `{"Robots":{"1":{"ID":1,"Kind":"core"}},` +
		`"Enemies":{"2":{"ID":2,"Kind":"scout"}}}`
	var old State
	if err := json.Unmarshal([]byte(legacy), &old); err != nil {
		t.Fatal(err)
	}
	if old.Robots[1].Facing != facingRight ||
		old.Enemies[2].Facing != facingRight {
		t.Errorf("old save defaults to robot %d and vehicle %d, want right",
			old.Robots[1].Facing, old.Enemies[2].Facing)
	}
}
