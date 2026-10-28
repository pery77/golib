package main

import (
	"math"
	"testing"

	"golib"
)

func countKinds(kinds []int) [len(enemyKinds)]int {
	var counts [len(enemyKinds)]int
	for _, k := range kinds {
		counts[k]++
	}
	return counts
}

func TestWavesGrowAndBringNewEnemies(t *testing.T) {
	first := countKinds(waveEnemies(1))
	if first != [len(enemyKinds)]int{scout: 4} {
		t.Errorf("wave 1 has %v, want 4 scouts only", first)
	}
	firstGunship, firstHeavy := 0, 0
	for wave := 1; wave <= 30; wave++ {
		kinds := waveEnemies(wave)
		counts := countKinds(kinds)
		want := [len(enemyKinds)]int{
			scout:   min(3+wave, 16),
			gunship: min(max(0, wave-1), 10),
			heavy:   min(max(0, (wave-2)/2), 5),
		}
		if counts != want {
			t.Errorf("wave %d has %v, want %v", wave, counts, want)
		}
		if next := waveEnemies(wave + 1); len(next) < len(kinds) {
			t.Errorf("wave %d has %d enemies, fewer than wave %d's %d", wave+1, len(next), wave, len(kinds))
		}
		if counts[gunship] > 0 && firstGunship == 0 {
			firstGunship = wave
		}
		if counts[heavy] > 0 && firstHeavy == 0 {
			firstHeavy = wave
		}
	}
	if firstGunship != 2 || firstHeavy != 4 {
		t.Errorf("gunships join in wave %d and heavies in wave %d, want 2 and 4", firstGunship, firstHeavy)
	}
}

func TestDifficultyGrowsUpToALimit(t *testing.T) {
	if waveSpeedScale(1) != 1 || waveFireScale(1) != 1 {
		t.Errorf("wave 1 scales speed by %v and fire by %v, want 1 and 1", waveSpeedScale(1), waveFireScale(1))
	}
	for wave := 1; wave < 60; wave++ {
		if waveSpeedScale(wave+1) < waveSpeedScale(wave) || waveFireScale(wave+1) < waveFireScale(wave) ||
			spawnInterval(wave+1) > spawnInterval(wave) || groupSize(wave+1) < groupSize(wave) {
			t.Fatalf("wave %d is easier than wave %d", wave+1, wave)
		}
	}
	if waveSpeedScale(60) != speedGrowthMax || waveFireScale(60) != fireGrowthMax || spawnInterval(60) != spawnEveryMin {
		t.Errorf("wave 60: speed %v, fire %v, spawn interval %v; want the limits", waveSpeedScale(60), waveFireScale(60), spawnInterval(60))
	}
}

func TestTheFirstWaveStartsAfterTheBreak(t *testing.T) {
	golib.SetRandomSeed(1)
	w := newWorld()
	if w.fighting() || w.wave != 0 {
		t.Fatal("a new game starts fighting, before the break")
	}
	run(&w, controls{}, firstBreak-0.1)
	if w.wave != 0 {
		t.Fatal("the first wave started before the break ended")
	}
	run(&w, controls{}, 0.2)
	if w.wave != 1 || !w.fighting() {
		t.Fatalf("after the break, wave %d, fighting %v; want 1, true", w.wave, w.fighting())
	}
	if len(w.warps) == 0 {
		t.Error("no enemies warp in when the wave starts")
	}
	if w.enemiesLeft() != len(waveEnemies(1)) {
		t.Errorf("%d enemies left, want %d", w.enemiesLeft(), len(waveEnemies(1)))
	}
}

func TestSpawnsAreFarFromTheShipAndInTheArena(t *testing.T) {
	golib.SetRandomSeed(7)
	w := newWorld()
	spots := []golib.Vector2{
		{X: worldWidth / 2, Y: worldHeight / 2},
		{},
		{X: worldWidth, Y: worldHeight},
		{X: worldWidth / 2},
	}
	for range 50 {
		spots = append(spots, golib.Vector2{X: golib.RandomFloat(0, worldWidth), Y: golib.RandomFloat(0, worldHeight)})
	}
	for _, spot := range spots {
		w.ship.position = spot
		for range 20 {
			at := w.spawnSpot()
			if at.Distance(w.ship.position) < spawnMinDistance {
				t.Fatalf("with the ship at %v, an enemy spawns at %v, too close", spot, at)
			}
			if at.X < spawnMargin || at.Y < spawnMargin || at.X > worldWidth-spawnMargin || at.Y > worldHeight-spawnMargin {
				t.Fatalf("with the ship at %v, an enemy spawns at %v, outside the arena", spot, at)
			}
		}
	}
}

func TestSpawnedGroupsStayInTheArena(t *testing.T) {
	golib.SetRandomSeed(3)
	w := newWorld()
	w.breakLeft = 0
	w.wave = 20
	w.queue = waveEnemies(20)
	for len(w.queue) > 0 {
		w.spawnGroup()
	}
	if len(w.warps) != len(waveEnemies(20)) {
		t.Fatalf("%d warps for %d enemies", len(w.warps), len(waveEnemies(20)))
	}
	for _, wp := range w.warps {
		r := enemyKinds[wp.kind].radius
		if wp.position.X < r || wp.position.Y < r || wp.position.X > worldWidth-r || wp.position.Y > worldHeight-r {
			t.Errorf("a %s warps in at %v, outside the arena", enemyKinds[wp.kind].name, wp.position)
		}
	}
}

func TestClearingAWaveRepairsAndRewards(t *testing.T) {
	golib.SetRandomSeed(1)
	w := newWorld()
	w.breakLeft, w.wave = 0, 2
	w.ship.hull = 3
	w.step(controls{}, dt)
	if w.fighting() {
		t.Fatal("a wave with no enemies left didn't end")
	}
	if w.ship.hull != 4 {
		t.Errorf("clearing a wave left the hull at %d, want 4", w.ship.hull)
	}
	if want := 2 * flawlessBonus; w.score != want || w.lastBonus != want {
		t.Errorf("a flawless wave 2 scored %d, with bonus %d; want %d", w.score, w.lastBonus, want)
	}
	run(&w, controls{}, waveBreak+0.1)
	if w.wave != 3 || !w.fighting() {
		t.Errorf("after the break, wave %d, fighting %v; want 3, true", w.wave, w.fighting())
	}
}

func TestAHitLosesTheFlawlessBonus(t *testing.T) {
	golib.SetRandomSeed(1)
	w := newWorld()
	w.breakLeft, w.wave = 0, 1
	w.ship.hitThisWave = true
	w.step(controls{}, dt)
	if w.score != 0 || w.lastBonus != 0 {
		t.Errorf("a wave with a hit scored a bonus of %d", w.lastBonus)
	}
}

func TestAWaveDoesntEndWhileEnemiesAreLeft(t *testing.T) {
	golib.SetRandomSeed(1)
	w := newWorld()
	w.breakLeft, w.wave = 0, 1
	untouchable(&w)
	w.warps = []warp{{kind: scout, position: golib.Vector2{X: 100, Y: 100}, left: warpTime}}
	w.step(controls{}, dt)
	if !w.fighting() {
		t.Error("the wave ended while an enemy was warping in")
	}
}

// TestTheFirstWavesCanBeCleared plays the game with a simple pilot that can't
// be hurt: it flies at the nearest enemy and shoots it. It checks that waves
// start, bring their enemies in, end and lead to the next.
func TestTheFirstWavesCanBeCleared(t *testing.T) {
	golib.SetRandomSeed(5)
	w := newWorld()
	for elapsed := float32(0); elapsed < 300 && w.wave < 4; elapsed += dt {
		untouchable(&w)
		c := controls{}
		best := float32(math.MaxFloat32)
		for _, e := range w.enemies {
			if d := e.position.Distance(w.ship.position); d < best {
				best = d
				c.aim = e.position.Sub(w.ship.position)
			}
		}
		if len(w.enemies) > 0 {
			c.fire = true
			if best > 300 {
				c.move = c.aim.Normalize()
			}
		}
		w.step(c, dt)
	}
	if w.wave < 4 {
		t.Fatalf("after five minutes the pilot is in wave %d with %d enemies left, want wave 4", w.wave, w.enemiesLeft())
	}
	if w.kills != len(waveEnemies(1))+len(waveEnemies(2))+len(waveEnemies(3)) {
		t.Errorf("clearing three waves took %d kills, want %d", w.kills, len(waveEnemies(1))+len(waveEnemies(2))+len(waveEnemies(3)))
	}
}
