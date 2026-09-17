package main

import (
	"math"
	"testing"

	"golib"
)

const dt = 1.0 / 60 // the step golib.Run passes to Update

// quietWorld returns a new world whose first wave never starts, so tests can
// place enemies themselves.
func quietWorld() world {
	golib.SetRandomSeed(1)
	w := newWorld()
	w.breakLeft = 1e6
	return w
}

// run steps the world for seconds with the same controls.
func run(w *world, c controls, seconds float32) {
	for range int(seconds * 60) {
		w.step(c, dt)
	}
}

func near(got, want, tolerance float32) bool {
	return math.Abs(float64(got-want)) <= float64(tolerance)
}

func TestShipFliesAtItsSpeed(t *testing.T) {
	w := quietWorld()
	start := w.ship.x
	run(&w, controls{moveX: 1}, 2)
	if !near(w.ship.vx, shipSpeed, 1) {
		t.Errorf("after two seconds flying right, speed is %v, want %v", w.ship.vx, shipSpeed)
	}
	if moved := w.ship.x - start; moved < shipSpeed*1.5 || moved > shipSpeed*2 {
		t.Errorf("in two seconds the ship flew %v pixels, want between %v and %v", moved, shipSpeed*1.5, shipSpeed*2)
	}
}

func TestFlyingDiagonallyIsNotFaster(t *testing.T) {
	w := quietWorld()
	run(&w, controls{moveX: 1, moveY: 1}, 2)
	if speed := length(w.ship.vx, w.ship.vy); !near(speed, shipSpeed, 1) {
		t.Errorf("flying diagonally, speed is %v, want %v", speed, shipSpeed)
	}
}

func TestShipStaysInTheArena(t *testing.T) {
	w := quietWorld()
	run(&w, controls{moveX: -1, moveY: -1}, 20)
	if w.ship.x != shipRadius || w.ship.y != shipRadius {
		t.Errorf("after flying into the top-left corner, the ship is at %v, %v, want %v, %v", w.ship.x, w.ship.y, shipRadius, shipRadius)
	}
	run(&w, controls{moveX: 1, moveY: 1}, 30)
	if w.ship.x != worldWidth-shipRadius || w.ship.y != worldHeight-shipRadius {
		t.Errorf("after flying into the bottom-right corner, the ship is at %v, %v", w.ship.x, w.ship.y)
	}
}

func TestHoldingFireShootsAtTheFireRate(t *testing.T) {
	w := quietWorld()
	fired := 0
	for range 60 {
		before := len(w.shots)
		w.step(controls{fire: true, aimX: 0, aimY: 1}, dt)
		if len(w.shots) > before {
			fired++
		}
	}
	want := int(math.Ceil(1 / fireCooldown))
	if fired < want-1 || fired > want {
		t.Errorf("holding fire for a second shot %d times, want %d", fired, want)
	}
	for _, b := range w.shots {
		if b.vy <= 0 || math.Abs(float64(b.vx)) > float64(b.vy)/10 {
			t.Fatalf("aiming down, a shot flies at %v, %v", b.vx, b.vy)
		}
	}
}

func TestAimStaysWithoutAimInput(t *testing.T) {
	w := quietWorld()
	w.step(controls{aimX: -1}, dt)
	run(&w, controls{}, 1)
	if !near(w.ship.angle, math.Pi, 0.001) && !near(w.ship.angle, -math.Pi, 0.001) {
		t.Errorf("after aiming left and letting go, the angle is %v, want pi", w.ship.angle)
	}
}

func TestShipAimsWhereItFliesWithNothingToAimWith(t *testing.T) {
	w := quietWorld()
	w.step(controls{moveY: 1}, dt)
	if !near(w.ship.angle, math.Pi/2, 0.001) {
		t.Errorf("flying down with no aim, the angle is %v, want pi/2", w.ship.angle)
	}
}

func TestShotsDestroyAScoutAndScore(t *testing.T) {
	w := quietWorld()
	w.enemies = append(w.enemies, w.newEnemy(scout, w.ship.x+150, w.ship.y))
	for i := 0; i < 60 && len(w.enemies) > 0; i++ {
		w.ship.hurt = 1 // the scout may fire back; this test is about shooting it
		w.step(controls{fire: true, aimX: 1}, dt)
	}
	if len(w.enemies) != 0 {
		t.Fatal("the scout in front of the ship survived a second of fire")
	}
	if w.score != enemyKinds[scout].points || w.kills != 1 {
		t.Errorf("score %d and kills %d, want %d and 1", w.score, w.kills, enemyKinds[scout].points)
	}
}

func TestPointsGrowWithTheWave(t *testing.T) {
	w := quietWorld()
	w.wave = 3
	w.enemies = append(w.enemies, w.newEnemy(gunship, 100, 100))
	w.destroyEnemy(0, true)
	if want := 3 * enemyKinds[gunship].points; w.score != want {
		t.Errorf("a gunship in wave 3 scored %d, want %d", w.score, want)
	}
}

func TestEnemiesTakeAsManyHitsAsTheirHull(t *testing.T) {
	for kind, k := range enemyKinds {
		w := quietWorld()
		w.enemies = append(w.enemies, w.newEnemy(kind, 500, 500))
		for hit := 1; hit <= k.hull; hit++ {
			e := w.enemies[0]
			w.shots = append(w.shots, shot{x: e.x, y: e.y, radius: shotRadius, life: 1})
			w.hitEnemies()
			if hit < k.hull && len(w.enemies) != 1 {
				t.Fatalf("a %s broke after %d hits, want %d", k.name, hit, k.hull)
			}
		}
		if len(w.enemies) != 0 {
			t.Errorf("a %s survived %d hits", k.name, k.hull)
		}
	}
}

func TestEnemyShotHurtsOnceThenTheShipIsSafeForAWhile(t *testing.T) {
	w := quietWorld()
	s := w.ship
	w.enemyShots = []shot{
		{x: s.x, y: s.y, radius: 5, life: 2},
		{x: s.x + 1, y: s.y, radius: 5, life: 2},
	}
	w.step(controls{}, dt)
	if w.ship.hull != shipHull-1 {
		t.Fatalf("two bullets at once left the hull at %d, want %d", w.ship.hull, shipHull-1)
	}
	if !w.ship.hitThisWave || w.ship.hurt <= 0 {
		t.Error("a hit ship isn't marked as hit, or can be hurt again at once")
	}
	w.enemyShots = []shot{{x: w.ship.x, y: w.ship.y, radius: 5, life: 2}}
	w.step(controls{}, dt)
	if w.ship.hull != shipHull-1 {
		t.Errorf("a bullet right after a hit took the hull to %d", w.ship.hull)
	}
	w.enemyShots = nil
	run(&w, controls{}, hurtTime)
	w.enemyShots = []shot{{x: w.ship.x, y: w.ship.y, radius: 5, life: 2}}
	w.step(controls{}, dt)
	if w.ship.hull != shipHull-2 {
		t.Errorf("a bullet after the safe time left the hull at %d, want %d", w.ship.hull, shipHull-2)
	}
}

func TestTheGameEndsWithTheLastHullPoint(t *testing.T) {
	w := quietWorld()
	w.ship.hull = 1
	w.enemyShots = []shot{{x: w.ship.x, y: w.ship.y, radius: 5, life: 2}}
	w.step(controls{}, dt)
	if w.ship.alive || !w.over {
		t.Fatal("the ship survived losing its last hull point")
	}
	run(&w, controls{fire: true, moveX: 1}, 1)
	if len(w.shots) != 0 {
		t.Error("a destroyed ship still fires")
	}
	if !near(w.overTime, 1, 0.001) {
		t.Errorf("overTime is %v after a second, want about 1", w.overTime)
	}
}

func TestDashingPassesThroughBullets(t *testing.T) {
	w := quietWorld()
	w.step(controls{moveX: 1, dash: true}, dt)
	if w.ship.dashLeft <= 0 {
		t.Fatal("dash didn't start")
	}
	w.enemyShots = []shot{{x: w.ship.x, y: w.ship.y, radius: 5, life: 2}}
	w.step(controls{moveX: 1}, dt)
	if w.ship.hull != shipHull {
		t.Errorf("a bullet hurt the ship while it dashed: hull %d", w.ship.hull)
	}
}

func TestDashMovesFarAndRecharges(t *testing.T) {
	w := quietWorld()
	start := w.ship.x
	w.step(controls{moveX: 1, dash: true}, dt)
	run(&w, controls{moveX: 1}, dashTime)
	if moved := w.ship.x - start; moved < dashSpeed*dashTime*0.9 {
		t.Errorf("a dash moved the ship %v pixels, want at least %v", moved, dashSpeed*dashTime*0.9)
	}
	w.step(controls{moveX: 1, dash: true}, dt)
	if w.ship.dashLeft > 0 {
		t.Error("a second dash started before the first recharged")
	}
	run(&w, controls{}, dashCooldown)
	w.step(controls{dash: true}, dt)
	if w.ship.dashLeft <= 0 {
		t.Error("no dash after recharging")
	}
	if w.ship.dashX != 1 || w.ship.dashY != 0 {
		// Standing still, the ship dashes where it aims, which is still right.
		t.Errorf("standing still, the dash goes %v, %v, want 1, 0", w.ship.dashX, w.ship.dashY)
	}
}

func TestRammingScoutBreaksAndHurts(t *testing.T) {
	w := quietWorld()
	w.enemies = append(w.enemies, w.newEnemy(scout, w.ship.x+5, w.ship.y))
	w.step(controls{}, dt)
	if len(w.enemies) != 0 {
		t.Error("a scout that rammed the ship is still there")
	}
	if w.ship.hull != shipHull-1 {
		t.Errorf("ramming left the hull at %d, want %d", w.ship.hull, shipHull-1)
	}
	if w.score != enemyKinds[scout].points/2 {
		t.Errorf("a rammed scout scored %d, want half of %d", w.score, enemyKinds[scout].points)
	}
}

func TestRepairKitsHealUpToFullAndExpire(t *testing.T) {
	w := quietWorld()
	w.ship.hull = 3
	w.repairs = []repairKit{{x: w.ship.x, y: w.ship.y, life: repairLifetime}, {x: w.ship.x + 2, y: w.ship.y, life: repairLifetime}}
	w.step(controls{}, dt)
	if w.ship.hull != 5 || len(w.repairs) != 0 {
		t.Fatalf("two kits left the hull at %d with %d kits left, want 5 and 0", w.ship.hull, len(w.repairs))
	}
	w.repairs = []repairKit{{x: w.ship.x, y: w.ship.y, life: repairLifetime}}
	w.step(controls{}, dt)
	if w.ship.hull != shipHull {
		t.Errorf("a kit took the hull past full, to %d", w.ship.hull)
	}

	w.repairs = []repairKit{{x: 100, y: 100, life: repairLifetime}}
	run(&w, controls{}, repairLifetime+dt)
	if len(w.repairs) != 0 {
		t.Error("a repair kit outlived its lifetime")
	}
}

func TestShotsVanishAtTheArenaEdge(t *testing.T) {
	w := quietWorld()
	w.shots = []shot{{x: 5, y: 500, vx: -shotSpeed, radius: shotRadius, life: 10}}
	w.enemyShots = []shot{{x: worldWidth - 5, y: 500, vx: 400, radius: 5, life: 10}}
	run(&w, controls{}, 0.1)
	if len(w.shots)+len(w.enemyShots) != 0 {
		t.Error("bullets flew out of the arena")
	}
}

func TestSessionKeepsTheBestResult(t *testing.T) {
	s := &session{}
	s.record(&world{score: 500, wave: 3})
	s.record(&world{score: 200, wave: 4})
	if s.bestScore != 500 || s.bestWave != 4 || s.newBest {
		t.Errorf("best %d, wave %d, new best %v; want 500, 4, false", s.bestScore, s.bestWave, s.newBest)
	}
	s.record(&world{score: 900, wave: 2})
	if s.bestScore != 900 || !s.newBest {
		t.Errorf("best %d, new best %v; want 900, true", s.bestScore, s.newBest)
	}
}
