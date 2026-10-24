package main

import "golib"

// The animations' speeds: how fast each character moves, in meters per
// second, when the kit's walk and sprint play at their own speed. Faster or
// slower, they play faster or slower, so the feet keep up with the ground.
const (
	walkAnimationSpeed   = 1.5
	sprintAnimationSpeed = 4.6
	runsAbove            = walkSpeed + 0.25 // meters per second: faster than this, the walk turns into a sprint
	movesAbove           = 0.15             // meters per second: slower than this, it stands and idles
)

// pose is the animation a character's whole body should play, worked out
// from its own state, with the options to play it with. The swings, the
// punches and the nod play over it, on the torso, from the rules.
type pose struct {
	name    string
	options golib.AnimationOptions
}

// legsPose is what the legs of a character that is up and about do: they
// jump and fall in the air, and idle, walk or sprint on the ground, at the
// speed it moves.
func legsPose(c character) pose {
	if !c.onGround {
		if c.velocity.Y > 0 {
			return pose{animJump, golib.AnimationOptions{Once: true, Fade: 0.1}}
		}
		return pose{animFall, golib.AnimationOptions{Fade: 0.25}}
	}
	speed := c.speed()
	switch {
	case speed > runsAbove:
		return pose{animSprint, golib.AnimationOptions{Speed: speed / sprintAnimationSpeed, Fade: 0.25}}
	case speed > movesAbove:
		return pose{animWalk, golib.AnimationOptions{Speed: max(speed/walkAnimationSpeed, 0.5), Fade: 0.25}}
	}
	return pose{animIdle, golib.AnimationOptions{Fade: 0.3}}
}

// playerPose is what the player's whole body plays.
func playerPose(p *player) pose {
	if !p.alive() {
		return pose{animDie, golib.AnimationOptions{Once: true, Fade: 0.1}}
	}
	return legsPose(p.character)
}

// enemyPose is what an enemy's whole body plays.
func enemyPose(e *enemy) pose {
	if e.down() {
		return pose{animDie, golib.AnimationOptions{Once: true, Fade: 0.1}}
	}
	return legsPose(e.character)
}

// poseAll tells every character's Animator what its body plays now. Called
// in every step: playing what already plays changes nothing, and anything
// else fades in.
func (w *world) poseAll() {
	pp := playerPose(&w.player)
	w.player.animator.PlayWith(pp.name, pp.options)
	for _, e := range w.enemies {
		ep := enemyPose(e)
		e.animator.PlayWith(ep.name, ep.options)
	}
}
