package main

// The animations' paces: how fast the astronaut moves, in meters per second,
// when the kit's walk and sprint play at their own speed. Faster or slower,
// they play faster or slower, so the feet keep up with the ground.
const (
	walkPace     = 1.5
	sprintPace   = 4.6
	sprintsAbove = walkSpeed + 0.3 // meters per second: faster than this, the walk turns into a sprint
	movesAbove   = 0.15            // meters per second: slower than this, it stands and idles
)

// pose tells the astronaut's Animator what its whole body plays now, from
// its own state. Called in every step: playing what already plays changes
// nothing, and anything else fades in. The punches and pick-ups play over
// it, on the torso, from the rules.
func (w *world) pose() {
	p := &w.player
	a := p.animator
	switch {
	case w.outcome == timeUp:
		a.Stop(boneTorso)
		playOnce(a, clipDie, 0.1)
	case w.outcome == won:
		a.Stop(boneTorso)
		loop(a, clipYes, 1, 0.2)
	case p.splash.Running():
		loop(a, clipFall, 1, 0.1)
	case !p.onGround && p.velocity.Y > 0:
		playOnce(a, clipJump, 0.08)
	case !p.onGround:
		loop(a, clipFall, 1, 0.25)
	case p.speed() > sprintsAbove:
		loop(a, clipSprint, p.speed()/sprintPace, 0.2)
	case p.speed() > movesAbove:
		loop(a, clipWalk, max(p.speed()/walkPace, 0.6), 0.2)
	default:
		loop(a, clipIdle, 1, 0.3)
	}
}
