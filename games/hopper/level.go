package main

import (
	"errors"
	"fmt"

	"golib"
)

// The models, made once. The level is one model, made in Blender by
// sources/models/level.py; the rest come straight from Kenney's kit.
var (
	levelModel     = golib.NewModel("models/level.glb")
	characterModel = golib.NewModel("models/kit/character-oopi.glb")
	coinModel      = golib.NewModel("models/kit/coin-gold.glb")
	flagModel      = golib.NewModel("models/kit/flag.glb")
)

// kitFolder is where the kit's models are, by the names the level's blocks
// give in their "model" property.
const kitFolder = "models/kit/"

// fallDepth is how far below the lowest solid top a fall costs a life, in
// meters: deep enough to watch the character drop out of sight.
const fallDepth = 7

// box is a solid block: the box around it in the world, aligned with the
// axes.
type box struct {
	lowest, upmost golib.Vector3
}

// level is what the game needs to know about the level made in Blender.
// Drawing it is drawing levelModel, which shows everything in it; the rules
// only need the solid blocks and the empties.
type level struct {
	blocks      []box
	coins       []golib.Vector3 // the middle of each coin
	checkpoints []golib.Vector3 // where the character starts again after a fall
	start       golib.Vector3   // the character's feet at the start
	startYaw    float32         // where it faces at the start
	goal        golib.Vector3   // the foot of the flag at the end
	killY       float32         // below this height a fall costs a life
	lowestTop   float32         // the lowest top of a solid block
}

// loadLevel reads the level from model's markers. Each solid block is an
// empty with kind = "block" and model = the kit model it instances; its box
// is that model's Bounds, placed as the empty is. Coins, checkpoints, the
// start and the goal are empties of their own kinds.
func loadLevel(model *golib.Model) (*level, error) {
	if err := model.Err(); err != nil {
		return nil, err
	}
	l := &level{}
	measured := map[string]*golib.Model{}
	foundStart, foundGoal := false, false
	for _, m := range model.Markers() {
		place := golib.Transform{Position: m.Position, Rotation: m.Rotation, Scale: m.Scale}
		switch m.Properties.String("kind") {
		case "block":
			name := m.Properties.String("model")
			if name == "" {
				return nil, fmt.Errorf("level: the block %q has no \"model\" property: run export_level.py in Blender", m.Name)
			}
			kit := measured[name]
			if kit == nil {
				kit = golib.NewModel(kitFolder + name + ".glb")
				measured[name] = kit
			}
			if err := kit.Err(); err != nil {
				return nil, fmt.Errorf("level: the block %q: %w", m.Name, err)
			}
			lowest, upmost := kit.Bounds()
			l.blocks = append(l.blocks, placeBox(place, lowest, upmost))
		case "coin":
			l.coins = append(l.coins, m.Position)
		case "checkpoint":
			l.checkpoints = append(l.checkpoints, m.Position)
		case "start":
			l.start, l.startYaw, foundStart = m.Position, m.Rotation.Yaw, true
		case "goal":
			l.goal, foundGoal = m.Position, true
		}
	}
	switch {
	case !foundStart:
		return nil, errors.New("level: no empty with kind = \"start\"")
	case !foundGoal:
		return nil, errors.New("level: no flag with kind = \"goal\"")
	case len(l.blocks) == 0:
		return nil, errors.New("level: no solid blocks (kind = \"block\")")
	}
	l.lowestTop = l.blocks[0].upmost.Y
	for _, b := range l.blocks {
		l.lowestTop = min(l.lowestTop, b.upmost.Y)
	}
	l.killY = l.lowestTop - fallDepth
	return l, nil
}

// placeBox returns the box, aligned with the axes, around a model's bounds
// once placed: moved, turned and scaled. Blocks turned by quarter turns keep
// their exact shape.
func placeBox(place golib.Transform, lowest, upmost golib.Vector3) box {
	var b box
	for corner := range 8 {
		p := lowest
		if corner&1 != 0 {
			p.X = upmost.X
		}
		if corner&2 != 0 {
			p.Y = upmost.Y
		}
		if corner&4 != 0 {
			p.Z = upmost.Z
		}
		p = place.Child(golib.Transform{Position: p}).Position
		if corner == 0 {
			b = box{lowest: p, upmost: p}
			continue
		}
		b.lowest = golib.Vector3{X: min(b.lowest.X, p.X), Y: min(b.lowest.Y, p.Y), Z: min(b.lowest.Z, p.Z)}
		b.upmost = golib.Vector3{X: max(b.upmost.X, p.X), Y: max(b.upmost.Y, p.Y), Z: max(b.upmost.Z, p.Z)}
	}
	return b
}

// groundBelow returns the top of the highest block under the point at, at or
// below it, and whether there is one: where the character's shadow falls.
func (l *level) groundBelow(at golib.Vector3) (float32, bool) {
	best, found := float32(0), false
	for _, b := range l.blocks {
		if at.X < b.lowest.X || at.X > b.upmost.X || at.Z < b.lowest.Z || at.Z > b.upmost.Z {
			continue
		}
		if b.upmost.Y <= at.Y+0.01 && (!found || b.upmost.Y > best) {
			best, found = b.upmost.Y, true
		}
	}
	return best, found
}
