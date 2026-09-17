package main

import (
	"math"

	"golib"
)

// Wave tuning.
const (
	firstBreak       = 2.0  // seconds before the first wave
	waveBreak        = 3.0  // seconds between waves
	flawlessBonus    = 500  // points, times the wave number, for a wave cleared without a hit
	spawnEvery       = 1.8  // seconds between groups in wave 1
	spawnEveryMin    = 0.7  // seconds between groups, at the most
	spawnEveryStep   = 0.1  // seconds fewer between groups every wave
	speedGrowth      = 0.05 // enemy speed added every wave, as a share of wave 1's
	speedGrowthMax   = 1.5  // the most enemy speed grows to, as a multiple of wave 1's
	fireGrowth       = 0.08 // enemy fire rate added every wave, as a share of wave 1's
	fireGrowthMax    = 2.0  // the most enemy fire rate grows to
	spawnMinDistance = 800  // pixels from the ship, at least, so enemies warp in off the screen
	spawnMargin      = 120  // pixels from the arena's edge, at least
	groupSpread      = 70   // pixels between the enemies of a group and its middle, at most
)

// waveEnemies returns the enemy kinds of a wave, in the order they warp in:
// scouts first, gunships from wave 2 and heavies from wave 4, mixed in.
func waveEnemies(wave int) []int {
	scouts := min(3+wave, 16)
	gunships := min(max(0, wave-1), 10)
	heavies := min(max(0, (wave-2)/2), 5)
	total := scouts + gunships + heavies
	kinds := make([]int, 0, total)
	for len(kinds) < total {
		// Deal them out like cards: a few scouts, then a gunship, and a heavy
		// once the rest are thinning out.
		for range 2 {
			if scouts > 0 {
				kinds = append(kinds, scout)
				scouts--
			}
		}
		if gunships > 0 {
			kinds = append(kinds, gunship)
			gunships--
		}
		if heavies > 0 && scouts <= heavies*2 {
			kinds = append(kinds, heavy)
			heavies--
		}
		if scouts == 0 && gunships == 0 && heavies > 0 {
			kinds = append(kinds, heavy)
			heavies--
		}
	}
	return kinds
}

// groupSize returns how many enemies warp in together in a wave.
func groupSize(wave int) int {
	return min(1+wave/2, 5)
}

// spawnInterval returns the seconds between groups in a wave.
func spawnInterval(wave int) float32 {
	return max(spawnEveryMin, spawnEvery-spawnEveryStep*float32(wave-1))
}

// waveSpeedScale returns how much faster enemies fly in a wave than in wave 1.
func waveSpeedScale(wave int) float32 {
	return min(speedGrowthMax, 1+speedGrowth*float32(max(0, wave-1)))
}

// waveFireScale returns how much more often enemies fire in a wave than in
// wave 1.
func waveFireScale(wave int) float32 {
	return min(fireGrowthMax, 1+fireGrowth*float32(max(0, wave-1)))
}

// fighting reports whether a wave is on, rather than the break before one.
func (w *world) fighting() bool {
	return w.breakLeft == 0
}

// enemiesLeft returns how many enemies of the wave are still to beat: those
// flying, warping in or still to come.
func (w *world) enemiesLeft() int {
	return len(w.enemies) + len(w.warps) + len(w.queue)
}

// updateWaves runs the break between waves, warps in the wave's enemies group
// by group, and ends the wave when they are all destroyed.
func (w *world) updateWaves(dt float32) {
	if w.over {
		return
	}
	if !w.fighting() {
		w.breakLeft = max(0, w.breakLeft-dt)
		if w.fighting() {
			w.startWave()
		}
		return
	}
	if len(w.queue) > 0 {
		w.spawnTimer -= dt
		if w.spawnTimer <= 0 {
			w.spawnTimer += spawnInterval(w.wave)
			w.spawnGroup()
		}
		return
	}
	if len(w.enemies) == 0 && len(w.warps) == 0 {
		w.endWave()
	}
}

// startWave starts the next wave.
func (w *world) startWave() {
	w.wave++
	w.queue = waveEnemies(w.wave)
	w.spawnTimer = 0
	w.ship.hitThisWave = false
	w.lastBonus = 0
}

// endWave rewards a cleared wave and starts the break before the next one.
func (w *world) endWave() {
	if !w.ship.hitThisWave {
		w.lastBonus = flawlessBonus * w.wave
		w.score += w.lastBonus
	}
	w.ship.hull = min(shipHull, w.ship.hull+1)
	w.breakLeft = waveBreak
	waveClearedSound.Play()
}

// spawnGroup starts warping in the next group of the wave, together at a spot
// away from the ship.
func (w *world) spawnGroup() {
	count := min(groupSize(w.wave), len(w.queue))
	x, y := w.spawnSpot()
	for n := range count {
		k := enemyKinds[w.queue[n]]
		angle := 2 * math.Pi * float32(n) / float32(count)
		dx, dy := direction(angle)
		spread := float32(0)
		if count > 1 {
			spread = groupSpread
		}
		w.warps = append(w.warps, warp{
			kind: w.queue[n],
			x:    clamp(x+dx*spread, k.radius, worldWidth-k.radius),
			y:    clamp(y+dy*spread, k.radius, worldHeight-k.radius),
			left: warpTime,
		})
	}
	w.queue = w.queue[count:]
	warpSound.Play()
}

// spawnSpot returns a random place in the arena at least spawnMinDistance
// from the ship. The arena is big enough that one of a few tries always is;
// the last resort is the corner farthest from the ship.
func (w *world) spawnSpot() (float32, float32) {
	for range 30 {
		x := golib.RandomFloat(spawnMargin, worldWidth-spawnMargin)
		y := golib.RandomFloat(spawnMargin, worldHeight-spawnMargin)
		if distanceSquared(x, y, w.ship.x, w.ship.y) >= spawnMinDistance*spawnMinDistance {
			return x, y
		}
	}
	x, y := float32(spawnMargin), float32(spawnMargin)
	if w.ship.x < worldWidth/2 {
		x = worldWidth - spawnMargin
	}
	if w.ship.y < worldHeight/2 {
		y = worldHeight - spawnMargin
	}
	return x, y
}
