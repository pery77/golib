package main

import (
	"math"
	"testing"

	"golib"
)

// loadTheLevel reads the real level, assets/models/level.glb, as main does.
func loadTheLevel(t *testing.T) *level {
	t.Helper()
	l, err := loadLevel(levelModel)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestTheModelsLoad(t *testing.T) {
	for _, m := range []*golib.Model{levelModel, characterModel, coinModel, flagModel} {
		if err := m.Err(); err != nil {
			t.Error(err)
		}
	}
	// The character is drawn standing on its feet, about 0.9 m tall.
	lowest, upmost := characterModel.Bounds()
	if math.Abs(float64(lowest.Y)) > 0.001 || upmost.Y < 0.8 || upmost.Y > 1.0 {
		t.Errorf("the character's bounds are %v to %v, want its feet at 0 and about 0.9 m tall", lowest, upmost)
	}
}

func TestTheLevelHasWhatTheGameNeeds(t *testing.T) {
	l := loadTheLevel(t)
	if len(l.coins) < 10 || len(l.blocks) < 10 || len(l.checkpoints) == 0 {
		t.Errorf("the level has %d coins, %d blocks and %d checkpoints; want a level's worth", len(l.coins), len(l.blocks), len(l.checkpoints))
	}
	for name, at := range map[string]golib.Vector3{"the start": l.start, "the goal": l.goal} {
		if top, ok := l.groundBelow(at); !ok || at.Y-top > 0.05 {
			t.Errorf("%s, at %v, doesn't stand on a block", name, at)
		}
	}
	for i, at := range l.checkpoints {
		if top, ok := l.groundBelow(at); !ok || at.Y-top > 0.05 {
			t.Errorf("checkpoint %d, at %v, doesn't stand on a block", i+1, at)
		}
	}
	// A block turned a quarter turn in Blender keeps its shape: the long low
	// blocks run along Z.
	long := 0
	for _, b := range l.blocks {
		if size := b.upmost.Sub(b.lowest); size.Z > 2 && size.X < 1.2 {
			long++
		}
	}
	if long == 0 {
		t.Error("no block runs along Z: the quarter turns of the long blocks were lost")
	}
}

// reach is how far, across the ground, a running jump carries the character
// from a block's edge to a block rise meters higher (or lower, if negative);
// 0 when the jump can't get that high.
func reach(rise float32) float32 {
	const v, g = jumpSpeed, gravity
	under := v*v - 2*g*rise
	if under < 0 {
		return 0
	}
	airTime := (v + float32(math.Sqrt(float64(under)))) / g
	return runSpeed*airTime + 2*playerRadius
}

// gap is the distance across the ground between two blocks' tops: 0 when
// they touch or overlap.
func gap(a, b box) float32 {
	dx := max(0, max(a.lowest.X-b.upmost.X, b.lowest.X-a.upmost.X))
	dz := max(0, max(a.lowest.Z-b.upmost.Z, b.lowest.Z-a.upmost.Z))
	return float32(math.Hypot(float64(dx), float64(dz)))
}

// TestTheFlagCanBeReached searches the jumps the character can make, from
// the block under the start, and checks that the flag's block is among the
// blocks they reach, with room to spare: each jump must be 15% shorter than
// the longest the character can make.
func TestTheFlagCanBeReached(t *testing.T) {
	l := loadTheLevel(t)
	under := func(at golib.Vector3) int {
		for i, b := range l.blocks {
			if at.X >= b.lowest.X && at.X <= b.upmost.X && at.Z >= b.lowest.Z && at.Z <= b.upmost.Z && math.Abs(float64(at.Y-b.upmost.Y)) < 0.05 {
				return i
			}
		}
		return -1
	}
	from, to := under(l.start), under(l.goal)
	if from < 0 || to < 0 {
		t.Fatalf("no block under the start (%d) or the goal (%d)", from, to)
	}
	seen := map[int]bool{from: true}
	queue := []int{from}
	for len(queue) > 0 {
		a := queue[0]
		queue = queue[1:]
		for b := range l.blocks {
			if seen[b] {
				continue
			}
			rise := l.blocks[b].upmost.Y - l.blocks[a].upmost.Y
			if gap(l.blocks[a], l.blocks[b]) <= 0.85*reach(rise) {
				seen[b] = true
				queue = append(queue, b)
			}
		}
	}
	if !seen[to] {
		t.Errorf("the flag can't be reached from the start: %d of %d blocks can", len(seen), len(l.blocks))
	}
}

// TestEveryCoinCanBeReached checks that each coin hangs where a jump from
// some block can touch it: no higher than a full jump over that block's top,
// and no farther across than a running jump.
func TestEveryCoinCanBeReached(t *testing.T) {
	l := loadTheLevel(t)
	highest := float32(jumpSpeed)*jumpSpeed/(2*gravity) + playerHeight/2 + coinReach
	for i, c := range l.coins {
		ok := false
		for _, b := range l.blocks {
			above := c.Y - b.upmost.Y
			flat := box{lowest: golib.Vector3{X: c.X, Z: c.Z}, upmost: golib.Vector3{X: c.X, Z: c.Z}}
			if above > 0 && above < 0.9*highest && gap(b, flat) < reach(0)/2 {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("coin %d, at %v, is out of reach", i+1, c)
		}
	}
}
