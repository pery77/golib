package main

import (
	"testing"

	"golib"
)

// The sound is view and its randomness is golib's, but its doors can be
// pinned all the same: a shot that just left a gun, in the view's reach,
// spends the small arms' cooldown; one out of earshot doesn't; and the
// ore's loops live only where the view stands.

func cameraOver(s *playScene, x, y float64, zoom float32) {
	s.zoom = zoom
	s.camera.Zoom = zoom
	px, py := project(float32(x), float32(y))
	s.camera.Target = golib.Vector2{X: px, Y: py}
	s.camera.Snap()
}

func TestANewBulletNearTheViewSpendsTheGunsCooldown(t *testing.T) {
	s := newPlayScene(nil)
	cameraOver(s, 1000, 1000, 8)
	s.state.fire(Shot{
		Kind: ShotBullet, FromX: 1010, FromY: 1000, ToX: 1100, ToY: 1000,
	})
	s.au.update(s, 1)
	if s.au.gunWait != gunCooldown {
		t.Fatalf("the shot left a gun the view hears, but the guns' cooldown is %d, want %d", s.au.gunWait, gunCooldown)
	}
}

func TestABulletOutOfEarshotLeavesTheGunsCold(t *testing.T) {
	s := newPlayScene(nil)
	cameraOver(s, 1000, 1000, 8)
	s.state.fire(Shot{
		Kind: ShotBullet, FromX: 60000, FromY: 60000, ToX: 60100, ToY: 60000,
	})
	s.au.update(s, 1)
	if s.au.gunWait != 0 {
		t.Fatalf("a shot beyond the view's reach spent the guns' cooldown: %d", s.au.gunWait)
	}
}

func TestAShellWhistlesThroughTheDescendingHalfOfItsFlight(t *testing.T) {
	s := newPlayScene(nil)
	cameraOver(s, 1000, 1000, 8)
	s.state.fire(Shot{
		Kind: ShotShell, FromX: 700, FromY: 1000, ToX: 1000, ToY: 1000,
	})
	closeID := s.state.NextID - 1
	incoming := s.state.Shots[closeID]
	incoming.X, incoming.Y = 850, 1000
	s.state.Shots[closeID] = incoming
	s.state.fire(Shot{
		Kind: ShotShell, FromX: 400, FromY: 1000, ToX: 1000, ToY: 1000,
	})
	farID := s.state.NextID - 1
	s.au.update(s, 1)
	if !s.au.whistled[closeID] {
		t.Fatal("a shell entering its descending half didn't whistle")
	}
	if s.au.whistled[farID] {
		t.Fatal("a shell still climbing whistled too soon")
	}
	if _, ok := s.au.whistleSlots[closeID]; !ok {
		t.Fatal("descending shell has no independent whistle")
	}
	incoming.X = 940
	s.state.Shots[closeID] = incoming
	s.au.update(s, 1)
	if _, ok := s.au.whistleSlots[closeID]; !ok {
		t.Fatal("the whistle stopped before the shell landed")
	}
	delete(s.state.Shots, closeID)
	s.au.update(s, 1)
	if _, ok := s.au.whistleSlots[closeID]; ok {
		t.Fatal("the whistle kept playing after the shell landed")
	}
}

func TestTheOreSoundsOnlyWhereTheViewStands(t *testing.T) {
	s := newPlayScene(nil)
	s.zoom = 8
	var pool, vein Deposit
	for i := range land.deposits {
		switch d := land.deposits[i]; d.Kind {
		case kindOil:
			if pool.Kind == 0 {
				pool = d
			}
		case kindLilac:
			if vein.Kind == 0 {
				vein = d
			}
		}
	}
	if pool.Kind == 0 || vein.Kind == 0 {
		t.Fatal("the region holds no pool or vein to listen to")
	}

	px, py := cellCenterUnits(pool.HeartCol, pool.HeartRow)
	cameraOver(s, px, py, 8)
	s.au.update(s, 1)
	if !s.au.oilBed.Looping() {
		t.Fatal("over a pool with oil, its bed is not looping")
	}

	vx, vy := cellCenterUnits(vein.HeartCol, vein.HeartRow)
	cameraOver(s, vx, vy, 8)
	s.au.tinkIn = tinkSoonest
	s.au.update(s, 1)
	if !s.au.ring.Looping() {
		t.Fatal("over a vein with ore, its resonance is not looping")
	}
	if s.au.tinkIn != tinkSoonest-1 {
		t.Fatalf("over a vein with ore, the pings didn't tick: tinkIn %d", s.au.tinkIn)
	}

	cameraOver(s, 60000, 60000, 8)
	s.au.update(s, 1)
	if s.au.oilBed.Looping() {
		t.Fatal("far from every pool, the bed is still looping")
	}
	if s.au.ring.Looping() {
		t.Fatal("far from every vein, the resonance is still looping")
	}
	if s.au.tinkIn != tinkSoonest {
		t.Fatalf("far from every vein, the pings didn't stand down: tinkIn %d", s.au.tinkIn)
	}
}

func TestTheOilDropsKeepToTheGround(t *testing.T) {
	if got := dropHush(zoomOfStop(zoomIn)); got < 1 {
		t.Fatalf("on the ground the drops sound at %g, want whole voice", got)
	}
	if got := dropHush(zoomOfStop(zoomIn - 1)); got > 0 {
		t.Fatalf("one notch up the drops still sound at %g, want silence", got)
	}
	if got := dropHush(zoomOfStop(zoomOut)); got > 0 {
		t.Fatalf("over the whole region the drops sound at %g, want silence", got)
	}
}

func TestTheMineralRingSwellComesAndGoes(t *testing.T) {
	if got := ringSwell(0); got < 0.99 {
		t.Fatalf("the swell starts at %g, want 1", got)
	}
	if got := ringSwell(ringCycle / 2); got > 0.01 {
		t.Fatalf("half a cycle in the ring still sounds at %g, want silence", got)
	}
	if got := ringSwell(ringCycle); got < 0.99 {
		t.Fatalf("a whole cycle in the swell is at %g, want 1", got)
	}
}

func TestAudioFallsWithViewDistanceAndZoom(t *testing.T) {
	s := newPlayScene(nil)
	x, y := float64(2500), float64(2500)
	previous := float32(0)
	for _, zoom := range []float32{1, 2, 8, 32} {
		cameraOver(s, x, y, zoom)
		got := s.au.audible(s, x, y, shellVolume)
		if got < previous {
			t.Fatalf("a world sound at zoom %g has volume %g, after %g",
				zoom, got, previous)
		}
		previous = got
	}
	for _, zoom := range []float32{2, 8, 32} {
		cameraOver(s, x, y, zoom)
		previous = s.au.audible(s, x, y, gunVolume)
		for _, distance := range []float64{200, 400, 600, 800} {
			cameraOver(s, x+distance, y, zoom)
			got := s.au.audible(s, x, y, gunVolume)
			if got > previous {
				t.Fatalf("gun grew louder at zoom %g with the camera %g meters away: %g after %g",
					zoom, distance, got, previous)
			}
			previous = got
		}
	}
	cameraOver(s, x, y, 32)
	near := s.au.audible(s, x+30, y, gunVolume)
	distant := s.au.audible(s, x+800, y, gunVolume)
	if distant >= near*0.05 {
		t.Fatalf("a gun 800 meters away is still too loud: %g after %g", distant, near)
	}
	shell := s.au.audibleWithFalloff(
		s, x+800, y, shellVolume, shellReach, shellFalloff,
	)
	if shell <= distant {
		t.Fatalf("a cannon lost its extra reach: %g versus gun %g", shell, distant)
	}
	cameraOver(s, x, y, 1)
	if got := s.au.audibleWithFalloff(
		s, x+1900, y, shellVolume, shellReach, shellFalloff,
	); got < 0.1 {
		t.Fatalf("artillery 1900 meters away vanished at the farthest zoom: %g", got)
	}
	cameraOver(s, 0, 0, 8)
	if got := s.au.audible(s, x, y, shellVolume); got != 0 {
		t.Fatalf("an off-screen world sound has volume %g", got)
	}
}

func TestAnUnseenShellLandingStillSoundsAtItsDestination(t *testing.T) {
	s := newPlayScene(nil)
	cameraOver(s, 2500, 2500, 8)
	s.state.fire(Shot{
		Kind: ShotShell, FromX: 2000, FromY: 2500,
		ToX: 2500, ToY: 2500,
	})
	id := s.state.NextID - 1
	s.au.update(s, 1)
	delete(s.state.Shots, id)
	s.au.update(s, 1)
	if len(s.au.known) != 0 {
		t.Fatal("landed shell is still tracked as flying")
	}
}

func TestBuildingCollapseAudioFollowsDistanceAndCause(t *testing.T) {
	s := newPlayScene(nil)
	cameraOver(s, 1000, 1000, 8)
	destroyed := BuildingDeath{
		Kind: BuildingFactory, Cause: BuildingDestroyed,
		X: 1000, Y: 1000,
	}
	destroyedVolume := s.au.buildingCollapseVolume(s, destroyed)
	destroyed.Cause = BuildingDemolished
	demolishedVolume := s.au.buildingCollapseVolume(s, destroyed)
	if demolishedVolume >= destroyedVolume || demolishedVolume <= 0 {
		t.Fatalf("manual demolition volume %g, violent destruction %g",
			demolishedVolume, destroyedVolume)
	}
	destroyed.X, destroyed.Y = 50000, 50000
	destroyed.Cause = BuildingDestroyed
	if got := s.au.buildingCollapseVolume(s, destroyed); got != 0 {
		t.Fatalf("a distant collapse has volume %g, want silence", got)
	}
}
