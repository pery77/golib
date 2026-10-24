package main

import "golib"

// The stone lift: it waits at each floor and goes to the other, carries
// whoever stands on it, and waits on its way down while anybody stands
// under it. golib moves its body in the PhysicsWorld with SetTransform, and
// carries a CharacterController standing on it at its next Move.

// transform is where the lift is now.
func (l *lift) transform() golib.Transform {
	return golib.Transform{Position: l.spec.center.Add(golib.Vector3{Y: l.height}), Scale: l.spec.size}
}

// top is the height of the lift's floor.
func (l *lift) top() float32 {
	return l.spec.center.Y + l.spec.size.Y/2 + l.height
}

// bottom is the height of its underside.
func (l *lift) bottom() float32 {
	return l.spec.center.Y - l.spec.size.Y/2 + l.height
}

// over says whether a point is over the lift's floor, or under it.
func (l *lift) over(at golib.Vector3, margin float32) bool {
	return abs(at.X-l.spec.center.X) <= l.spec.size.X/2+margin && abs(at.Z-l.spec.center.Z) <= l.spec.size.Z/2+margin
}

// blocked says whether a character of height stands under the lift, where
// the lift coming down would land on it.
func (l *lift) blocked(feet golib.Vector3, height, radius float32) bool {
	return l.over(feet, radius) && feet.Y < l.bottom() && feet.Y+height > l.bottom()-0.15
}

// stepLift waits and moves the lift, and returns how far up it moved. It
// waits on its way down while somebody stands under it.
func (w *world) stepLift(dt float32, happened *events) float32 {
	l := &w.lift
	var moved float32
	switch l.state {
	case liftDown, liftUp:
		if l.wait.Tick(dt) {
			l.state = map[liftState]liftState{liftDown: liftRising, liftUp: liftSinking}[l.state]
		}
		return 0
	case liftRising:
		moved = min(liftSpeed*dt, l.spec.rise-l.height)
	case liftSinking:
		if w.underLift() {
			return 0
		}
		moved = -min(liftSpeed*dt, l.height)
	}
	l.height += moved
	happened.liftMoving = true
	if l.state == liftRising && l.height >= l.spec.rise {
		l.state, happened.liftStopped = liftUp, true
		l.wait.Start(liftWait)
	} else if l.state == liftSinking && l.height <= 0 {
		l.state, happened.liftStopped = liftDown, true
		l.wait.Start(liftWait)
	}
	w.physics.SetTransform(l.body, l.transform())
	return moved
}

// underLift says whether anybody stands where the lift comes down.
func (w *world) underLift() bool {
	if w.lift.blocked(w.player.body.Position, playerHeight, playerRadius) {
		return true
	}
	for _, g := range w.guards {
		if w.lift.blocked(g.body.Position, playerHeight, playerRadius) {
			return true
		}
	}
	return false
}
