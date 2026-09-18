package golib

// Timer counts seconds down: a cooldown between shots, the wait before the
// next enemy, how long a power-up lasts. Keep it in the game's state, tick it
// in Update, and it reports the update it reaches zero:
//
//	type playScene struct {
//		spawn    golib.Timer
//		cooldown golib.Timer
//	}
//
//	func newPlayScene() *playScene {
//		return &playScene{spawn: golib.NewRepeatingTimer(2)} // an enemy every 2 seconds
//	}
//
//	func (s *playScene) Update(input *golib.Input, dt float32) {
//		if s.spawn.Tick(dt) {
//			s.spawnEnemy()
//		}
//		s.cooldown.Tick(dt)
//		if input.KeyDown(golib.KeySpace) && !s.cooldown.Running() {
//			s.shoot()
//			s.cooldown.Start(0.25) // no shot for a quarter of a second
//		}
//	}
//
// The zero Timer is stopped, so a game can leave one until it needs it, and
// Tick counts nothing until Start runs it.
type Timer struct {
	left   float32 // seconds left; 0 or less means stopped
	period float32 // the seconds it was given, for the next round and Progress
	repeat bool
}

// NewTimer returns a timer that runs for seconds and then stops, such as how
// long a shield lasts. Zero or less makes a stopped timer.
func NewTimer(seconds float32) Timer {
	return Timer{left: max(seconds, 0), period: max(seconds, 0)}
}

// NewRepeatingTimer returns a timer that finishes every seconds and starts
// over, such as the wait between enemies, keeping the beat when an update
// passes the end.
func NewRepeatingTimer(seconds float32) Timer {
	return Timer{left: max(seconds, 0), period: max(seconds, 0), repeat: true}
}

// Tick counts dt seconds off the timer and reports whether it reached zero in
// this update: once for a timer from NewTimer, which then stops, and every
// round for one from NewRepeatingTimer, which starts over. A stopped timer
// counts nothing and reports false, so Update can tick every timer it has.
// One update finishes a timer at most once.
func (t *Timer) Tick(dt float32) bool {
	if t.left <= 0 {
		return false
	}
	t.left -= dt
	if t.left > 0 {
		return false
	}
	if t.repeat && t.period > 0 {
		// Carry the time past the end into the next round, so a timer that
		// finishes every 2 seconds keeps finishing on the second.
		t.left += t.period * float32(int(-t.left/t.period)+1)
	} else {
		t.left = 0
	}
	return true
}

// Start runs the timer for seconds from now, whatever it was doing: a
// cooldown starts again on every shot. Zero or less stops it. A repeating
// timer keeps repeating, with the new number of seconds.
func (t *Timer) Start(seconds float32) {
	t.left, t.period = max(seconds, 0), max(seconds, 0)
}

// Stop stops the timer without letting it finish, such as when the player
// dies: Tick reports false until Start runs it again.
func (t *Timer) Stop() {
	t.left = 0
}

// Running reports whether the timer has time left. A cooldown is ready again
// when it isn't running.
func (t *Timer) Running() bool {
	return t.left > 0
}

// Left returns the seconds left, and 0 when the timer isn't running: draw it
// to count down.
func (t *Timer) Left() float32 {
	return max(t.left, 0)
}

// Progress returns how far the timer has come, from 0 when it started to 1
// when it finished, for bars, fades and anything else that grows with time. A
// stopped timer is 1.
func (t *Timer) Progress() float32 {
	if t.period <= 0 || t.left <= 0 {
		return 1
	}
	return 1 - t.left/t.period
}
