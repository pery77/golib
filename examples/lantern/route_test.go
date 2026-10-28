package main

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"golib"
)

// keyRanges records when each key is held, update by update, as a
// golib shot --input script.
type keyRanges struct {
	held  map[string]int // key -> the update it went down
	parts []string
}

func (k *keyRanges) set(key string, down bool, update int) {
	start, held := k.held[key]
	switch {
	case down && !held:
		k.held[key] = update
	case !down && held:
		k.parts = append(k.parts, fmt.Sprintf("%s@%d-%d", key, start, update-1))
		delete(k.held, key)
	}
}

// TestPlayedWithTheCameraTheTempleCanBeEscaped plays the whole game as a
// player does, with the forward and turning keys only, through every gem to
// the exit, and logs the keys as a golib shot --input script (play scene
// updates start at update 2, after Enter on the title in update 1):
//
//	./golib test lantern --run TestPlayedWithTheCamera
func TestPlayedWithTheCameraTheTempleCanBeEscaped(t *testing.T) {
	w := newWorld()
	camera := newCamera(w)
	route := []golib.Vector3{
		w.gems[0].position, {X: -1.2, Z: 9.5}, w.gems[1].position, {X: -0.5, Z: 11}, {Z: 13.2}, {X: -5.4, Z: 14}, {X: -5.2, Z: 18},
		{X: -2, Z: 20}, w.gems[2].position, {X: 4.6, Z: 23}, {X: 4.6, Z: 26.6}, w.gems[3].position, {X: 2, Z: 26.8}, {Z: 27}, {Z: 29.5},
		w.gems[4].position, {X: -1.2, Z: 33}, {X: 1.2, Z: 36}, w.gems[5].position, {X: 0.5, Z: 38.4}, {Z: 39}, w.exit,
	}
	keys := &keyRanges{held: map[string]int{}}
	const offset = 1 // the play scene's first update is the game's second
	next := 0
	update := 0
	for ; update < 60*120 && !w.escaped; update++ {
		if next < len(route) && flatDistance(w.player.body.Position, route[next]) < 0.5 {
			next++
		}
		var c controls
		if next < len(route) {
			way := route[next].Sub(w.player.body.Position)
			want := golib.RotationTowards(golib.Vector3{X: way.X, Z: way.Z}).Yaw
			diff := float32(math.Remainder(float64(want-camera.Heading), 360))
			if diff > 4 {
				c.turn = 1
			} else if diff < -4 {
				c.turn = -1
			}
			if diff < 35 && diff > -35 {
				c.forward = 1
			}
		}
		keys.set("Up", c.forward > 0, update+1+offset)
		keys.set("Left", c.turn > 0, update+1+offset)
		keys.set("Right", c.turn < 0, update+1+offset)
		want := steer(c, camera, dt)
		w.step(want, dt)
		follow(camera, w, dt)
	}
	for _, key := range []string{"Up", "Left", "Right"} {
		keys.set(key, false, update+1+offset)
	}
	if !w.escaped {
		t.Fatalf("playing with the camera, the explorer didn't escape: %d gems, at %v, heading for point %d of the route", w.gemsTaken(), w.player.body.Position, next)
	}
	t.Logf("escaped after %d updates; script:\nEnter@1 %s", update, strings.Join(keys.parts, " "))
}
