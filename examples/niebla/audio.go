package main

import (
	"math"

	"golib"
)

// The region's sound. All of it is view: the play scene owns the field,
// the state never hears of it, and its randomness is golib's. The world
// speaks where it happens — a shell sounds from the gun that fired it, a
// drip from the pool it left — and the view weighs every world sound by
// the distance from its source to the camera's center on the ground.
// The mineral ping and three ambience loops are synthesized by
// tools/soundgen; whistles and impacts are made in code.

const (
	uiClickVolume  = 0.8       // a press of the interface
	uiClickPitch   = 0.72      // a deeper press than the source recording
	placeVolume    = 0.7       // a site marked on the ground
	alertVolume    = 0.65      // a new rival report
	shellVolume    = 1.0       // the colony's artillery speaking
	shellFarVolume = 0.9       // a rival city's mobile artillery
	gunVolume      = 0.9       // small arms, the colony's and the rivals'
	gunCooldown    = 4         // ticks between two small-arm sounds at most
	burstVolume    = 1.0       // a shell landing
	whistleVolume  = 0.18      // a shell coming down where the view stands
	whistleCount   = 8         // independent incoming shells at once
	dripVolume     = 0.3       // an oil pool's bloop
	gurgleVolume   = 0.275     // its rarer, thicker cousin
	tinkVolume     = 0.32 / 24 // a lilac vein's crystal ping
	ringVolume     = 0.1       // the vein's resonance at its loudest
	dropHushZoom   = 28        // zoom where the oil drops fall silent
	dropHushSpan   = 4         // zooms of fade from that silence up
	ringCycle      = 20 * 60   // ticks of one ring's swell: 20 s
	tinkCrowdBoost = 0.15      // how much every extra audible vein lifts a ping
	tinkCrowdHurry = 0.5       // and how much it hurries the next one along
	windFarVolume  = 0.18      // the wind, the whole region in view
	windNearVolume = 0.04      // the wind, the ground in view
	poolBedVolume  = 1.0       // the pool's buried seethe, up close
	dripSoonest    = 120       // ticks between two drips at the least: 2 s
	dripLatest     = 420       // and at the most: 7 s
	gurgleSoonest  = 600       // ticks between gurgles: 10 s
	gurgleLatest   = 1500      // 25 s
	tinkSoonest    = 12        // ticks between two pings at the least: 0.2 s
	tinkLatest     = 75        // and at the most: 1.25 s
	nearZoomSpan   = 2.0       // zooms of glide from a whisper to a full world
	nearZoomFloor  = 0.35      // the weight the world keeps at the farthest stop
	audioReach     = 2800.0    // ordinary world max range, meters
	audioViewReach = 1.5       // view radii ordinary sounds carry
	shellReach     = 3800.0    // a cannon carries farther than small arms
	audioFalloff   = 3.2       // nearby sounds dominate the general mix
	shellFalloff   = 1.3       // cannon reports keep more of their distance
	audioHigh      = 2100.0    // cannon listener height at the farthest zoom
	audioFloor     = 0.02      // quieter than this and a sound doesn't start
)

const (
	buildingCrashVolume = 0.85 // a building coming apart
	demolitionVolume    = 0.6  // manual dismantling is less violent
)

type audioField struct {
	click    *golib.Sound
	place    *golib.Sound
	alert    *golib.Sound
	collapse *golib.Sound
	shell    *golib.Sound
	shellFar *golib.Sound
	burst    *golib.Sound
	whistles [whistleCount]*golib.Sound
	gunA     *golib.Sound
	gunB     *golib.Sound
	drip     *golib.Sound
	gurgle   *golib.Sound
	tink     *golib.Sound
	ring     *golib.Sound
	wind     *golib.Sound
	oilBed   *golib.Sound

	known        map[int64]Shot
	whistled     map[int64]bool
	whistleSlots map[int64]int
	gunWait      int
	dripIn       int
	gurgleIn     int
	tinkIn       int
}

func newAudioField() *audioField {
	a := &audioField{
		click: golib.NewSoundFile("sounds/click.ogg"),
		place: golib.NewSound(golib.SoundSpec{
			Wave: golib.WaveNoise, Frequency: 125, Slide: -240,
			Duration: 0.27, Attack: 0.002, Release: 0.24,
			Volume: 0.6,
		}),
		alert:    golib.NewSoundFile("sounds/alert.wav"),
		collapse: golib.NewSoundFile("sounds/building-collapse.wav"),
		shell:    golib.NewSoundFile("sounds/artillery-fire.ogg"),
		shellFar: golib.NewSoundFile("sounds/artillery-fire-distant.ogg"),
		burst: golib.NewSound(golib.SoundSpec{
			// The whump of a shell arriving: a wide, slow breath of
			// noise, deeper and longer than the framework's own burst.
			Wave: golib.WaveNoise, Frequency: 140, Slide: -100,
			Duration: 1.1, Attack: 0.002, Release: 0.95, Volume: 1,
		}),
		gunA:         golib.NewSoundFile("sounds/gun-a.ogg"),
		gunB:         golib.NewSoundFile("sounds/gun-b.ogg"),
		drip:         golib.NewSoundFile("sounds/oil-drip.ogg"),
		gurgle:       golib.NewSoundFile("sounds/oil-gurgle.ogg"),
		tink:         golib.NewSoundFile("sounds/mineral-tink.wav"),
		ring:         golib.NewSoundFile("sounds/mineral-ring.wav"),
		wind:         golib.NewSoundFile("sounds/wind-loop.ogg"),
		oilBed:       golib.NewSoundFile("sounds/oil-bed.ogg"),
		known:        map[int64]Shot{},
		whistled:     map[int64]bool{},
		whistleSlots: map[int64]int{},
		dripIn:       dripSoonest,
		gurgleIn:     gurgleSoonest,
		tinkIn:       tinkSoonest,
	}
	for i := range a.whistles {
		a.whistles[i] = golib.NewSound(golib.SoundSpec{
			Wave: golib.WaveSine, Frequency: 650, Slide: -55,
			Duration: 10, Attack: 0.05, Release: 0.4, Volume: 0.5,
		})
	}
	return a
}

// ui plays the interface's click; the pitch keeps each kind of press
// recognizable by ear alone.
func (a *audioField) ui(pitch float32) {
	a.click.PlayWith(uiClickVolume, pitch*uiClickPitch)
}

func (a *audioField) placed() {
	a.place.PlayWith(placeVolume, 1)
}

func (a *audioField) warning() {
	a.alert.PlayWith(alertVolume, 1)
}

func (a *audioField) buildingCollapsed(s *playScene, death BuildingDeath) {
	if volume := a.buildingCollapseVolume(s, death); volume >= audioFloor {
		a.collapse.PlayWith(volume, golib.RandomFloat(0.94, 1.06))
	}
}

func (a *audioField) buildingCollapseVolume(
	s *playScene,
	death BuildingDeath,
) float32 {
	base := buildingCrashVolume
	if death.Cause == BuildingDemolished {
		base *= demolitionVolume
	}
	return a.audible(s, death.X, death.Y, float64(base))
}

// nearness says how close the view stands to the ground: the weight
// ordinary world sounds carry. It never reaches nothing, and from zoom 3
// on it is whole.
func nearness(zoom float32) float32 {
	return nearZoomFloor +
		(1-nearZoomFloor)*golib.Clamp((zoom-1)/nearZoomSpan, 0, 1)
}

// dropHush says how much the oil drops sound at a zoom: whole voice on
// the ground, and silence as soon as the view rises past dropHushZoom,
// so the bloops keep to the closest stops whatever the view covers.
func dropHush(zoom float32) float32 {
	return golib.Clamp((zoom-dropHushZoom)/dropHushSpan, 0, 1)
}

// ringSwell is the ring's amplitude modulation: one slow, whole wave of
// ringCycle ticks, so the vein's resonance swells up, falls silent and
// swells again - a little while sounding every so often.
func ringSwell(ticks int64) float32 {
	phase := 2 * math.Pi * float64(ticks%ringCycle) / float64(ringCycle)
	return float32(0.5 + 0.5*math.Cos(phase))
}

// audible weighs an ordinary world sound by its distance from the view
// and the size of the ground currently on screen.
func (a *audioField) audible(s *playScene, x, y, base float64) float32 {
	center := s.camera.Center()
	rx, ry := unproject(center.X, center.Y)
	dx, dy := x-float64(rx), y-float64(ry)
	reach := math.Min(audioReach, audioViewRadius(s.zoom)*audioViewReach)
	if reach <= 0 {
		return 0
	}
	fall := 1 - math.Hypot(dx, dy)/reach
	if fall <= 0 {
		return 0
	}
	v := base * math.Pow(fall, audioFalloff) *
		float64(nearness(s.zoom))
	if v < audioFloor {
		return 0
	}
	return float32(math.Min(v, 1))
}

func audioViewRadius(zoom float32) float64 {
	if zoom <= 0 {
		return 0
	}
	halfWidth := float64(screenWidth) / (2 * float64(zoom))
	halfHeight := float64(screenHeight) / (2 * float64(zoom))
	x := halfWidth / float64(unitW)
	y := halfHeight / float64(unitH)
	return math.Sqrt(2 * (x*x + y*y))
}

// audibleWithFalloff keeps cannon reports on their longer, elevated range.
func (a *audioField) audibleWithFalloff(
	s *playScene, x, y, base, reach, falloff float64,
) float32 {
	center := s.camera.Center()
	rx, ry := unproject(center.X, center.Y)
	stop := golib.Clamp(
		float32(math.Log2(float64(s.zoom))), zoomOut, zoomIn,
	)
	height := audioHigh * (1 - float64(stop)/zoomIn)
	dx, dy := x-float64(rx), y-float64(ry)
	fall := 1 - math.Sqrt(dx*dx+dy*dy+height*height)/reach
	if fall <= 0 {
		return 0
	}
	v := base * math.Pow(fall, falloff)
	if v < audioFloor {
		return 0
	}
	return float32(math.Min(v, 1))
}

// update keeps the two loops breathing, drops the pool's bloops in from
// time to time, and learns of fired and landed shots the way the lights
// do, by comparing the state's with the ones it saw last.
func (a *audioField) update(s *playScene, ticks int) {
	a.wind.SetVolume(windFarVolume + (windNearVolume-windFarVolume)*nearness(s.zoom))
	a.wind.Loop()
	for _, death := range s.state.BuildingDeaths {
		a.buildingCollapsed(s, death)
	}

	if _, _, heard, _ := a.nearestDeposit(s, kindOil); heard > 0 {
		a.oilBed.SetVolume(float32(math.Sqrt(float64(heard))) * poolBedVolume)
		a.oilBed.Loop()
		hush := dropHush(s.zoom)
		a.dripIn -= ticks
		a.gurgleIn -= ticks
		if a.dripIn <= 0 {
			if v := heard * dripVolume * hush; v >= audioFloor {
				a.drip.PlayWith(v, golib.RandomFloat(0.7, 1.1))
			}
			a.dripIn = dripSoonest + golib.RandomInt(0, dripLatest-dripSoonest)
		}
		if a.gurgleIn <= 0 {
			if v := heard * gurgleVolume * hush; v >= audioFloor {
				a.gurgle.PlayWith(v, golib.RandomFloat(0.9, 1.1))
			}
			a.gurgleIn = gurgleSoonest + golib.RandomInt(0, gurgleLatest-gurgleSoonest)
		}
	} else {
		a.oilBed.SetVolume(0)
		a.oilBed.Stop()
	}

	if _, _, heard, crowd := a.nearestDeposit(s, kindLilac); heard > 0 {
		a.ring.SetVolume(heard * ringVolume * ringSwell(s.state.Ticks))
		a.ring.Loop()
		a.tinkIn -= ticks
		if a.tinkIn <= 0 {
			loud := heard * tinkVolume *
				min(1.4, 1+tinkCrowdBoost*float32(crowd-1))
			a.tink.PlayWith(loud, golib.RandomFloat(0.8, 1.3))
			hurry := 1 + tinkCrowdHurry*float32(crowd-1)
			a.tinkIn = int(float32(tinkSoonest+
				golib.RandomInt(0, tinkLatest-tinkSoonest)) / hurry)
		}
	} else {
		a.ring.Stop()
		a.tinkIn = tinkSoonest
	}

	a.gunWait = max(0, a.gunWait-ticks)
	for id, shot := range s.state.Shots {
		if _, old := a.known[id]; old {
			continue
		}
		a.fired(s, shot)
	}
	for _, id := range sortedShotIDs(s.state) {
		shot := s.state.Shots[id]
		if shot.Kind != ShotShell {
			continue
		}
		remaining := math.Hypot(shot.ToX-shot.X, shot.ToY-shot.Y)
		whole := math.Hypot(shot.ToX-shot.FromX, shot.ToY-shot.FromY)
		if remaining > whole/2 {
			continue
		}
		newWhistle := false
		if !a.whistled[id] {
			a.whistled[id] = true
			if slot, ok := a.freeWhistle(); ok {
				a.whistleSlots[id] = slot
				newWhistle = true
			}
		}
		if slot, ok := a.whistleSlots[id]; ok {
			fade := math.Min(1, remaining/(shellSpeed*0.35))
			v := a.audible(s, shot.X, shot.Y, whistleVolume*fade)
			a.whistles[slot].SetVolume(v)
			if newWhistle {
				a.whistles[slot].Play()
			}
		}
	}
	for id, shot := range a.known {
		if _, flying := s.state.Shots[id]; flying {
			continue
		}
		if slot, ok := a.whistleSlots[id]; ok {
			a.whistles[slot].Stop()
			delete(a.whistleSlots, id)
		}
		if shot.Kind != ShotShell {
			continue
		}
		if v := a.audible(s, shot.ToX, shot.ToY, burstVolume); v > 0 {
			a.burst.PlayWith(v, golib.RandomFloat(0.9, 1.1))
		}
	}
	sawWhistle := a.whistled
	a.known = make(map[int64]Shot, len(s.state.Shots))
	a.whistled = make(map[int64]bool, len(s.state.Shots))
	for id, shot := range s.state.Shots {
		a.known[id] = shot
		a.whistled[id] = sawWhistle[id]
	}
}

func (a *audioField) freeWhistle() (int, bool) {
	var used [whistleCount]bool
	for _, slot := range a.whistleSlots {
		used[slot] = true
	}
	for i := range used {
		if !used[i] {
			return i, true
		}
	}
	return 0, false
}

// fired sounds a shot leaving its gun: the colony's artillery has its own
// report, rival mobile artillery has the distant one, and small arms are
// throttled to one every few ticks so a battle doesn't turn into a rattle.
func (a *audioField) fired(s *playScene, shot Shot) {
	if shot.Kind == ShotShell {
		sound, base := a.shell, shellVolume
		if shot.Rival {
			sound, base = a.shellFar, shellFarVolume
		}
		if v := a.audibleWithFalloff(
			s, shot.FromX, shot.FromY, base, shellReach,
			shellFalloff,
		); v > 0 {
			sound.PlayWith(v, golib.RandomFloat(0.96, 1.04))
		}
		return
	}
	if a.gunWait > 0 {
		return
	}
	v := a.audible(s, shot.FromX, shot.FromY, gunVolume)
	if v <= 0 {
		return
	}
	a.gunWait = gunCooldown
	gun := a.gunA
	if shot.ID%2 == 0 {
		gun = a.gunB
	}
	gun.PlayWith(v, golib.RandomFloat(0.92, 1.08))
}

// nearestDeposit finds the deposit of a kind closest to the view's
// middle and how loudly it sounds there, in units, so the pools seethe
// and the veins sparkle where the ore is — never where it ran out. It
// also counts the deposits of the kind the view hears at all, so the
// veins' shimmer can grow with how much mineral is in earshot.
func (a *audioField) nearestDeposit(s *playScene, kind byte) (float64, float64, float32, int) {
	var bx, by float64
	heard := float32(0)
	crowd := 0
	for i := range land.deposits {
		d := land.deposits[i]
		if d.Kind != kind {
			continue
		}
		tcol, trow := cellTile(d.HeartCol, d.HeartRow)
		if remainingAt(s.state, tcol, trow) <= 0 {
			continue
		}
		x, y := cellCenterUnits(d.HeartCol, d.HeartRow)
		if v := a.audible(s, x, y, 1); v > 0 {
			crowd++
			if v > heard {
				heard, bx, by = v, x, y
			}
		}
	}
	return bx, by, heard, crowd
}

// stopLoops silences what keeps sounding on its own, for the way out of
// the region; whatever plays once is left to finish.
func (a *audioField) stopLoops() {
	a.wind.Stop()
	a.oilBed.Stop()
	a.ring.Stop()
	for _, sound := range a.whistles {
		sound.Stop()
	}
}
