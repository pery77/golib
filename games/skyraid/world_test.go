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

// moving returns controls that fly the ship the way x, y points.
func moving(x, y float32) controls {
	return controls{move: golib.Vector2{X: x, Y: y}}
}

// aiming returns controls that aim the ship the way x, y points.
func aiming(x, y float32) controls {
	return controls{aim: golib.Vector2{X: x, Y: y}}
}

func TestShipFliesAtItsSpeed(t *testing.T) {
	w := quietWorld()
	start := w.ship.position.X
	run(&w, moving(1, 0), 2)
	if !near(w.ship.velocity.X, shipSpeed, 1) {
		t.Errorf("after two seconds flying right, speed is %v, want %v", w.ship.velocity.X, shipSpeed)
	}
	if moved := w.ship.position.X - start; moved < shipSpeed*1.5 || moved > shipSpeed*2 {
		t.Errorf("in two seconds the ship flew %v pixels, want between %v and %v", moved, shipSpeed*1.5, shipSpeed*2)
	}
}

func TestFlyingDiagonallyIsNotFaster(t *testing.T) {
	w := quietWorld()
	run(&w, moving(1, 1), 2)
	if speed := w.ship.velocity.Length(); !near(speed, shipSpeed, 1) {
		t.Errorf("flying diagonally, speed is %v, want %v", speed, shipSpeed)
	}
}

func TestShipStaysInTheArena(t *testing.T) {
	w := quietWorld()
	run(&w, moving(-1, -1), 20)
	if w.ship.position != (golib.Vector2{X: shipRadius, Y: shipRadius}) {
		t.Errorf("after flying into the top-left corner, the ship is at %v, want %v, %v", w.ship.position, shipRadius, shipRadius)
	}
	run(&w, moving(1, 1), 30)
	if w.ship.position != (golib.Vector2{X: worldWidth - shipRadius, Y: worldHeight - shipRadius}) {
		t.Errorf("after flying into the bottom-right corner, the ship is at %v", w.ship.position)
	}
}

func TestHoldingFireShootsAtTheFireRate(t *testing.T) {
	w := quietWorld()
	fired := 0
	down := aiming(0, 1)
	down.fire = true
	for range 60 {
		before := len(w.shots)
		w.step(down, dt)
		if len(w.shots) > before {
			fired++
		}
	}
	want := int(math.Ceil(1 / fireCooldown))
	if fired < want-1 || fired > want {
		t.Errorf("holding fire for a second shot %d times, want %d", fired, want)
	}
	for _, b := range w.shots {
		if b.velocity.Y <= 0 || math.Abs(float64(b.velocity.X)) > float64(b.velocity.Y)/10 {
			t.Fatalf("aiming down, a shot flies at %v", b.velocity)
		}
	}
}

func TestAimStaysWithoutAimInput(t *testing.T) {
	w := quietWorld()
	w.step(aiming(-1, 0), dt)
	run(&w, controls{}, 1)
	if !near(abs(w.ship.angle), 180, 0.001) {
		t.Errorf("after aiming left and letting go, the angle is %v, want 180", w.ship.angle)
	}
}

func TestShipAimsWhereItFliesWithNothingToAimWith(t *testing.T) {
	w := quietWorld()
	w.step(moving(0, 1), dt)
	if !near(w.ship.angle, 90, 0.001) {
		t.Errorf("flying down with no aim, the angle is %v, want 90", w.ship.angle)
	}
}

func TestShotsDestroyAScoutAndScore(t *testing.T) {
	w := quietWorld()
	w.enemies = append(w.enemies, w.newEnemy(scout, w.ship.position.Add(golib.Vector2{X: 150})))
	right := aiming(1, 0)
	right.fire = true
	for i := 0; i < 60 && len(w.enemies) > 0; i++ {
		w.ship.hurt = 1 // the scout may fire back; this test is about shooting it
		w.step(right, dt)
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
	w.enemies = append(w.enemies, w.newEnemy(gunship, golib.Vector2{X: 100, Y: 100}))
	w.destroyEnemy(0, true)
	if want := 3 * enemyKinds[gunship].points; w.score != want {
		t.Errorf("a gunship in wave 3 scored %d, want %d", w.score, want)
	}
}

func TestEnemiesTakeAsManyHitsAsTheirHull(t *testing.T) {
	for kind, k := range enemyKinds {
		w := quietWorld()
		w.enemies = append(w.enemies, w.newEnemy(kind, golib.Vector2{X: 500, Y: 500}))
		for hit := 1; hit <= k.hull; hit++ {
			e := w.enemies[0]
			w.shots = append(w.shots, shot{position: e.position, radius: shotRadius, life: 1})
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
	at := w.ship.position
	w.enemyShots = []shot{
		{position: at, radius: 5, life: 2},
		{position: at.Add(golib.Vector2{X: 1}), radius: 5, life: 2},
	}
	w.step(controls{}, dt)
	if w.ship.hull != shipHull-1 {
		t.Fatalf("two bullets at once left the hull at %d, want %d", w.ship.hull, shipHull-1)
	}
	if !w.ship.hitThisWave || w.ship.hurt <= 0 {
		t.Error("a hit ship isn't marked as hit, or can be hurt again at once")
	}
	w.enemyShots = []shot{{position: w.ship.position, radius: 5, life: 2}}
	w.step(controls{}, dt)
	if w.ship.hull != shipHull-1 {
		t.Errorf("a bullet right after a hit took the hull to %d", w.ship.hull)
	}
	w.enemyShots = nil
	run(&w, controls{}, hurtTime)
	w.enemyShots = []shot{{position: w.ship.position, radius: 5, life: 2}}
	w.step(controls{}, dt)
	if w.ship.hull != shipHull-2 {
		t.Errorf("a bullet after the safe time left the hull at %d, want %d", w.ship.hull, shipHull-2)
	}
}

func TestAHitBreaksTheScreenUpAndItSettles(t *testing.T) {
	w := quietWorld()
	if w.glitch != 0 || w.danger() != 0 {
		t.Fatal("a new world already glitches, or its ship is already in danger")
	}
	w.enemyShots = []shot{{position: w.ship.position, radius: 5, life: 2}}
	w.step(controls{}, dt)
	if !near(w.glitch, glitchHit, glitchDecay*dt) {
		t.Errorf("a hit set the glitch to %v, want about %v", w.glitch, glitchHit)
	}
	run(&w, controls{}, 1)
	if w.glitch != 0 {
		t.Errorf("the glitch is %v once it has had time to fade, want 0", w.glitch)
	}

	// The lens strains on the last hull point, and lets go when the ship does.
	w.ship.hull = 1
	if w.danger() <= 0 {
		t.Error("the ship on its last hull point isn't in danger")
	}
	w.ship.alive = false
	if w.danger() != 0 {
		t.Error("a destroyed ship is still in danger")
	}
}

func TestTheGameEndsWithTheLastHullPoint(t *testing.T) {
	w := quietWorld()
	w.ship.hull = 1
	w.enemyShots = []shot{{position: w.ship.position, radius: 5, life: 2}}
	w.step(controls{}, dt)
	if w.ship.alive || !w.over {
		t.Fatal("the ship survived losing its last hull point")
	}
	flyingAndFiring := moving(1, 0)
	flyingAndFiring.fire = true
	run(&w, flyingAndFiring, 1)
	if len(w.shots) != 0 {
		t.Error("a destroyed ship still fires")
	}
	if !near(w.overTime, 1, 0.001) {
		t.Errorf("overTime is %v after a second, want about 1", w.overTime)
	}
}

// dashingRight flies right and asks for a dash.
func dashingRight() controls {
	c := moving(1, 0)
	c.dash = true
	return c
}

func TestDashingPassesThroughBullets(t *testing.T) {
	w := quietWorld()
	w.step(dashingRight(), dt)
	if w.ship.dashLeft <= 0 {
		t.Fatal("dash didn't start")
	}
	w.enemyShots = []shot{{position: w.ship.position, radius: 5, life: 2}}
	w.step(moving(1, 0), dt)
	if w.ship.hull != shipHull {
		t.Errorf("a bullet hurt the ship while it dashed: hull %d", w.ship.hull)
	}
}

func TestDashMovesFarAndRecharges(t *testing.T) {
	w := quietWorld()
	start := w.ship.position.X
	w.step(dashingRight(), dt)
	run(&w, moving(1, 0), dashTime)
	if moved := w.ship.position.X - start; moved < dashSpeed*dashTime*0.9 {
		t.Errorf("a dash moved the ship %v pixels, want at least %v", moved, dashSpeed*dashTime*0.9)
	}
	w.step(dashingRight(), dt)
	if w.ship.dashLeft > 0 {
		t.Error("a second dash started before the first recharged")
	}
	run(&w, controls{}, dashCooldown)
	w.step(controls{dash: true}, dt)
	if w.ship.dashLeft <= 0 {
		t.Error("no dash after recharging")
	}
	if w.ship.dashWay != (golib.Vector2{X: 1}) {
		// Standing still, the ship dashes where it aims, which is still right.
		t.Errorf("standing still, the dash goes %v, want 1, 0", w.ship.dashWay)
	}
}

func TestRammingScoutBreaksAndHurts(t *testing.T) {
	w := quietWorld()
	w.enemies = append(w.enemies, w.newEnemy(scout, w.ship.position.Add(golib.Vector2{X: 5})))
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
	w.repairs = []repairKit{
		{position: w.ship.position, life: repairLifetime},
		{position: w.ship.position.Add(golib.Vector2{X: 2}), life: repairLifetime},
	}
	w.step(controls{}, dt)
	if w.ship.hull != 5 || len(w.repairs) != 0 {
		t.Fatalf("two kits left the hull at %d with %d kits left, want 5 and 0", w.ship.hull, len(w.repairs))
	}
	w.repairs = []repairKit{{position: w.ship.position, life: repairLifetime}}
	w.step(controls{}, dt)
	if w.ship.hull != shipHull {
		t.Errorf("a kit took the hull past full, to %d", w.ship.hull)
	}

	w.repairs = []repairKit{{position: golib.Vector2{X: 100, Y: 100}, life: repairLifetime}}
	run(&w, controls{}, repairLifetime+dt)
	if len(w.repairs) != 0 {
		t.Error("a repair kit outlived its lifetime")
	}
}

func TestShotsVanishAtTheArenaEdge(t *testing.T) {
	w := quietWorld()
	w.shots = []shot{{position: golib.Vector2{X: 5, Y: 500}, velocity: golib.Vector2{X: -shotSpeed}, radius: shotRadius, life: 10}}
	w.enemyShots = []shot{{position: golib.Vector2{X: worldWidth - 5, Y: 500}, velocity: golib.Vector2{X: 400}, radius: 5, life: 10}}
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
