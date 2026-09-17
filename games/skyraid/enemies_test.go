package main

import (
	"math"
	"testing"
)

// distanceToShip returns how far enemy i is from the ship.
func distanceToShip(w *world, i int) float32 {
	e := w.enemies[i]
	return float32(math.Sqrt(float64(distanceSquared(e.x, e.y, w.ship.x, w.ship.y))))
}

// untouchable keeps the ship from being hurt, for tests about the enemies.
func untouchable(w *world) {
	w.ship.hurt = 1e6
}

func TestScoutsChaseTheShip(t *testing.T) {
	w := quietWorld()
	untouchable(&w)
	w.enemies = append(w.enemies, w.newEnemy(scout, w.ship.x-600, w.ship.y-100))
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
			w.enemies = append(w.enemies, w.newEnemy(kind, w.ship.x+start, w.ship.y))
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
	w.enemies = append(w.enemies, w.newEnemy(gunship, w.ship.x-500, w.ship.y))
	limit := firstShotDelay + enemyKinds[gunship].fireEvery + 0.1
	for elapsed := float32(0); elapsed < limit && len(w.enemyShots) == 0; elapsed += dt {
		w.step(controls{}, dt)
	}
	if len(w.enemyShots) != enemyKinds[gunship].volley {
		t.Fatalf("a gunship in range fired %d bullets, want a volley of %d", len(w.enemyShots), enemyKinds[gunship].volley)
	}
	for _, b := range w.enemyShots {
		if b.vx <= 0 {
			t.Errorf("a gunship to the left of the ship fired a bullet at %v, %v, away from it", b.vx, b.vy)
		}
	}
	// The middle bullet of the fan flies straight at the ship, from wherever
	// the gunship has circled to.
	middle := w.enemyShots[len(w.enemyShots)/2]
	toX, toY := normalize(w.ship.x-middle.x, w.ship.y-middle.y)
	flyX, flyY := normalize(middle.vx, middle.vy)
	if toX*flyX+toY*flyY < 0.999 {
		t.Errorf("the middle bullet flies along %v, %v, but the ship is along %v, %v", flyX, flyY, toX, toY)
	}
}

func TestFarEnemiesHoldFire(t *testing.T) {
	w := quietWorld()
	untouchable(&w)
	w.enemies = append(w.enemies, w.newEnemy(heavy, w.ship.x+1500, w.ship.y))
	run(&w, controls{}, firstShotDelay+enemyKinds[heavy].fireEvery+0.1)
	if len(w.enemyShots) != 0 {
		t.Errorf("a heavy %v pixels away fired", distanceToShip(&w, 0))
	}
}

func TestEnemiesHoldFireOnceTheShipIsGone(t *testing.T) {
	w := quietWorld()
	w.enemies = append(w.enemies, w.newEnemy(scout, w.ship.x+400, w.ship.y))
	w.ship.alive = false
	w.over = true
	run(&w, controls{}, 6)
	if len(w.enemyShots) != 0 {
		t.Error("an enemy fired at a destroyed ship")
	}
}

func TestVolleysAreFansAroundTheAim(t *testing.T) {
	w := quietWorld()
	e := w.newEnemy(heavy, 100, 100)
	e.angle = 0
	w.enemyFire(&e)
	k := enemyKinds[heavy]
	if len(w.enemyShots) != k.volley {
		t.Fatalf("a heavy fired %d bullets, want %d", len(w.enemyShots), k.volley)
	}
	first, last := w.enemyShots[0], w.enemyShots[k.volley-1]
	spread := angleOf(last.vx, last.vy) - angleOf(first.vx, first.vy)
	if want := k.fan * float32(k.volley-1); !near(spread, want, 0.001) {
		t.Errorf("the fan spreads %v radians, want %v", spread, want)
	}
	if !near(angleOf(first.vx, first.vy), -angleOf(last.vx, last.vy), 0.001) {
		t.Error("the fan isn't centered on the aim")
	}
}

func TestEnemiesKeepApart(t *testing.T) {
	w := quietWorld()
	untouchable(&w)
	w.enemies = append(w.enemies,
		w.newEnemy(scout, w.ship.x+2000, w.ship.y),
		w.newEnemy(scout, w.ship.x+2001, w.ship.y),
	)
	run(&w, controls{}, 1)
	a, b := w.enemies[0], w.enemies[1]
	d := float32(math.Sqrt(float64(distanceSquared(a.x, a.y, b.x, b.y))))
	if d < 2*enemyKinds[scout].radius {
		t.Errorf("two scouts that started on top of each other are %v pixels apart after a second", d)
	}
}

func TestWarpsTurnIntoEnemies(t *testing.T) {
	w := quietWorld()
	w.warps = []warp{{kind: gunship, x: 300, y: 300, left: warpTime}}
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
