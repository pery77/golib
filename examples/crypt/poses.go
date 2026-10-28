package main

import "golib"

// The animations' speeds: how fast a character moves, in meters per second,
// when the kit's walk and sprint play at their own speed. Faster or slower,
// they play faster or slower, so the feet keep up with the ground.
const (
	walkAnimationSpeed   = 1.5
	sprintAnimationSpeed = 4.6
	runsAbove            = 2.0  // meters per second: faster than this, the walk turns into a sprint
	movesAbove           = 0.15 // meters per second: slower than this, it stands and idles
	fallsBelow           = -2.5 // meters per second down: faster than this, off the ground, it falls
)

// pose is the animation a character's whole body should play, with the
// options to play it with. Punches, pick-ups and a guard's reaching arms
// play over it, on the torso, from the rules.
type pose struct {
	name    string
	options golib.AnimationOptions
}

// legsPose is what the legs of a character do: they fall in the air, and
// idle, walk or sprint on the ground, at the speed it moves.
func legsPose(onGround bool, velocity golib.Vector3) pose {
	if !onGround && velocity.Y < fallsBelow {
		return pose{animFall, golib.AnimationOptions{Fade: 0.2}}
	}
	speed := flat(velocity).Length()
	switch {
	case speed > runsAbove:
		return pose{animSprint, golib.AnimationOptions{Speed: speed / sprintAnimationSpeed, Fade: 0.2}}
	case speed > movesAbove:
		return pose{animWalk, golib.AnimationOptions{Speed: max(speed/walkAnimationSpeed, 0.5), Fade: 0.2}}
	}
	return pose{animIdle, golib.AnimationOptions{Fade: 0.3}}
}

// playerPose is what the astronaut's whole body plays.
func (w *world) playerPose() pose {
	p := &w.player
	switch {
	case w.won:
		return pose{animCheer, golib.AnimationOptions{Fade: 0.2}}
	case p.caught.Running():
		return pose{animDie, golib.AnimationOptions{Once: true, Fade: 0.1}}
	}
	return legsPose(p.body.OnGround, p.velocity)
}

// guardPose is what a guard's whole body plays.
func guardPose(g *guard) pose {
	return legsPose(g.body.OnGround, g.velocity)
}

// poseAll tells every character's Animator what its body plays now. Called
// in every step: playing what already plays changes nothing, and anything
// else fades in.
func (w *world) poseAll() {
	pp := w.playerPose()
	w.player.animator.PlayWith(pp.name, pp.options)
	for _, g := range w.guards {
		gp := guardPose(g)
		g.animator.PlayWith(gp.name, gp.options)
	}
}
