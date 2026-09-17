package main

import (
	"testing"

	"golib"
)

// distanceToShip returns how far enemy i is from the ship.
func distanceToShip(w *world, i int) float32 {
	return w.enemies[i].position.Distance(w.ship.position)
}

// at returns the arena point the ship's distance x, y away.
func (w *world) at(x, y float32) golib.Vector2 {
	return w.ship.position.Add(golib.Vector2{X: x, Y: y})
}

// untouchable keeps the ship from being hurt, for tests about the enemies.
func untouchable(w *world) {
	w.ship.hurt = 1e6
}

func TestScoutsChaseTheShip(t *testing.T) {
	w := quietWorld()
	untouchable(&w)
	w.enemies = append(w.enemies, w.newEnemy(scout, w.at(-600, -100)))
	before := distanceToShip(&w, 0)
	run(&w, controls{}, 1)
	if after := distanceToShip(&w, 0); after > before-120 {
		t.Errorf("in a second a scout got from %v to %v pixels of the ship, want at least 120 closer", before, after)
	}
}

func TestShootersKeepTheirDistance(t *testing.T) {
	for _, kind := range []int{gunship, heavy} {
		k := enemyKinds[kind]
		for _, start := range []float32{120, 900} {
			w := quietWorld()
			untouchable(&w)
			w.enemies = append(w.enemies, w.newEnemy(kind, w.at(start, 0)))
			run(&w, controls{}, 12)
			d := distanceToShip(&w, 0)
			if d < k.keepAway*0.7 || d > k.keepAway*1.3 {
				t.Errorf("a %s that started %v pixels away is %v pixels away after 12 seconds, want about %v", k.name, start, d, k.keepAway)
			}
		}
	}
}

func TestEnemiesFireAtTheShip(t *testing.T) {
	w := quietWorld()
	untouchable(&w)
	w.enemies = append(w.enemies, w.newEnemy(gunship, w.at(-500, 0)))
	limit := firstShotDelay + enemyKinds[gunship].fireEvery + 0.1
	for elapsed := float32(0); elapsed < limit && len(w.enemyShots) == 0; elapsed += dt {
		w.step(controls{}, dt)
	}
	if len(w.enemyShots) != enemyKinds[gunship].volley {
		t.Fatalf("a gunship in range fired %d bullets, want a volley of %d", len(w.enemyShots), enemyKinds[gunship].volley)
	}
	for _, b := range w.enemyShots {
		if b.velocity.X <= 0 {
			t.Errorf("a gunship to the left of the ship fired a bullet at %v, away from it", b.velocity)
		}
	}
	// The middle bullet of the fan flies straight at the ship, from wherever
	// the gunship has circled to.
	middle := w.enemyShots[len(w.enemyShots)/2]
	to := w.ship.position.Sub(middle.position).Normalize()
	fly := middle.velocity.Normalize()
	if to.Dot(fly) < 0.999 {
		t.Errorf("the middle bullet flies along %v, but the ship is along %v", fly, to)
	}
}

func TestFarEnemiesHoldFire(t *testing.T) {
	w := quietWorld()
	untouchable(&w)
	w.enemies = append(w.enemies, w.newEnemy(heavy, w.at(1500, 0)))
	run(&w, controls{}, firstShotDelay+enemyKinds[heavy].fireEvery+0.1)
	if len(w.enemyShots) != 0 {
		t.Errorf("a heavy %v pixels away fired", distanceToShip(&w, 0))
	}
}

func TestEnemiesHoldFireOnceTheShipIsGone(t *testing.T) {
	w := quietWorld()
	w.enemies = append(w.enemies, w.newEnemy(scout, w.at(400, 0)))
	w.ship.alive = false
	w.over = true
	run(&w, controls{}, 6)
	if len(w.enemyShots) != 0 {
		t.Error("an enemy fired at a destroyed ship")
	}
}

func TestVolleysAreFansAroundTheAim(t *testing.T) {
	w := quietWorld()
	e := w.newEnemy(heavy, golib.Vector2{X: 100, Y: 100})
	e.angle = 0
	w.enemyFire(&e)
	k := enemyKinds[heavy]
	if len(w.enemyShots) != k.volley {
		t.Fatalf("a heavy fired %d bullets, want %d", len(w.enemyShots), k.volley)
	}
	first, last := w.enemyShots[0], w.enemyShots[k.volley-1]
	spread := last.velocity.Angle() - first.velocity.Angle()
	if want := k.fan * float32(k.volley-1); !near(spread, want, 0.001) {
		t.Errorf("the fan spreads %v degrees, want %v", spread, want)
	}
	if !near(first.velocity.Angle(), -last.velocity.Angle(), 0.001) {
		t.Error("the fan isn't centered on the aim")
	}
}

func TestEnemiesKeepApart(t *testing.T) {
	w := quietWorld()
	untouchable(&w)
	w.enemies = append(w.enemies,
		w.newEnemy(scout, w.at(2000, 0)),
		w.newEnemy(scout, w.at(2001, 0)),
	)
	run(&w, controls{}, 1)
	a, b := w.enemies[0], w.enemies[1]
	d := a.position.Distance(b.position)
	if d < 2*enemyKinds[scout].radius {
		t.Errorf("two scouts that started on top of each other are %v pixels apart after a second", d)
	}
}

func TestWarpsTurnIntoEnemies(t *testing.T) {
	w := quietWorld()
	w.warps = []warp{{kind: gunship, position: golib.Vector2{X: 300, Y: 300}, left: warpTime}}
	run(&w, controls{}, warpTime-0.1)
	if len(w.enemies) != 0 {
		t.Fatal("an enemy arrived before its warp ended")
	}
	run(&w, controls{}, 0.2)
	if len(w.enemies) != 1 || len(w.warps) != 0 || w.enemies[0].kind != gunship {
		t.Fatalf("after the warp: %d enemies, %d warps, want 1 gunship and none", len(w.enemies), len(w.warps))
	}
	if w.enemies[0].cooldown < firstShotDelay-0.2 {
		t.Errorf("a new enemy can fire in %v seconds, want at least %v", w.enemies[0].cooldown, firstShotDelay)
	}
}
