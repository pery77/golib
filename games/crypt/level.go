package main

import (
	"errors"
	"fmt"
	"sort"

	"golib"
)

// level is what the rules need to know about the crypt made in Blender by
// sources/models/level.py: where things are, read from the level model's
// markers. Drawing the crypt is drawing levelModel; the walls are solid
// because world.go adds the model to a golib.PhysicsWorld.
type level struct {
	start       golib.Vector3
	startYaw    float32
	exit        golib.Vector3
	checkpoints []checkpointSpec
	doors       []boxSpec
	crates      []boxSpec
	keys        []boxSpec // keys lying in the open; only their name and place matter
	lift        liftSpec
	guards      []guardSpec
	torches     []golib.Vector3
}

// checkpointSpec is a place the player comes back to after a guard grabs
// them, once they have reached it.
type checkpointSpec struct {
	room     string
	position golib.Vector3
	yaw      float32
}

// boxSpec is a door, a crate or a key: its name, its middle and its size.
type boxSpec struct {
	name   string
	center golib.Vector3
	size   golib.Vector3
	key    bool // a crate with a key inside
}

// liftSpec is the stone lift: its middle and size at the bottom of its
// shaft, and how far up it goes.
type liftSpec struct {
	center golib.Vector3
	size   golib.Vector3
	rise   float32
}

// guardSpec is where a guard starts and the points it patrols, in order.
type guardSpec struct {
	name  string
	start golib.Vector3
	route []golib.Vector3
}

// loadLevel reads the level from model's markers (see level.py for what
// each kind of marker is).
func loadLevel(model *golib.Model) (*level, error) {
	if err := model.Err(); err != nil {
		return nil, err
	}
	l := &level{}
	type routePoint struct {
		order int
		at    golib.Vector3
	}
	routes := map[string][]routePoint{}
	guardRoutes := map[string]string{}
	foundStart, foundExit, foundLift := false, false, false
	for _, m := range model.Markers() {
		switch m.Properties.String("kind") {
		case "start":
			l.start, l.startYaw, foundStart = m.Position, m.Rotation.Yaw, true
		case "exit":
			l.exit, foundExit = m.Position, true
		case "checkpoint":
			l.checkpoints = append(l.checkpoints, checkpointSpec{room: m.Properties.String("room"), position: m.Position, yaw: m.Rotation.Yaw})
		case "door":
			l.doors = append(l.doors, boxSpec{name: m.Name, center: m.Position, size: m.Scale})
		case "crate":
			l.crates = append(l.crates, boxSpec{name: m.Name, center: m.Position, size: m.Scale, key: m.Properties.Int("key") == 1})
		case "key":
			l.keys = append(l.keys, boxSpec{name: m.Name, center: m.Position})
		case "lift":
			l.lift, foundLift = liftSpec{center: m.Position, size: m.Scale, rise: m.Properties.Float("rise")}, true
		case "guard":
			l.guards = append(l.guards, guardSpec{name: m.Name, start: m.Position})
			guardRoutes[m.Name] = m.Properties.String("route")
		case "patrol":
			route := m.Properties.String("route")
			routes[route] = append(routes[route], routePoint{m.Properties.Int("order"), m.Position})
		case "torch":
			l.torches = append(l.torches, m.Position)
		}
	}
	for i := range l.guards {
		g := &l.guards[i]
		points := routes[guardRoutes[g.name]]
		if len(points) == 0 {
			return nil, fmt.Errorf("level: the guard %q has no patrol points with route %q", g.name, guardRoutes[g.name])
		}
		sort.Slice(points, func(a, b int) bool { return points[a].order < points[b].order })
		for _, p := range points {
			g.route = append(g.route, p.at)
		}
	}
	switch {
	case !foundStart:
		return nil, errors.New("level: no empty with kind = \"start\"")
	case !foundExit:
		return nil, errors.New("level: no empty with kind = \"exit\"")
	case !foundLift:
		return nil, errors.New("level: no empty with kind = \"lift\"")
	case len(l.checkpoints) == 0:
		return nil, errors.New("level: no checkpoints")
	}
	return l, nil
}

// keysInLevel is how many keys the crypt holds: in the open and in crates.
func (l *level) keysInLevel() int {
	n := len(l.keys)
	for _, c := range l.crates {
		if c.key {
			n++
		}
	}
	return n
}

// theLevel is the crypt, read once from levelModel by main, and by TestMain
// in the tests. Not in its declaration: package variables are set before
// assets.go's init hands golib the assets embedded in a dist or web build,
// so reading the model there finds no file in those builds.
var theLevel *level
