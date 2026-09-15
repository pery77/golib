package golib

// Run advances every game in fixed steps, so a game behaves the same at any frame rate.
const (
	updatesPerSecond = 60

	// updateStep is the dt Run passes to Game.Update: the game time, in seconds,
	// that one update covers.
	updateStep = 1.0 / updatesPerSecond

	// maxUpdatesPerFrame caps the updates one frame can run. After a long pause,
	// such as while the window is dragged, the game skips the lost time instead
	// of fast-forwarding through it.
	maxUpdatesPerFrame = 5

	// stepTolerance lets a frame slightly shorter than a step still run one
	// update. Frame times jitter around 1/60 s; without it, frames would
	// alternate between zero and two updates.
	stepTolerance = updateStep / 8
)

// clock turns real frame times into a number of fixed updates.
type clock struct {
	pending float64 // real seconds not yet covered by updates; can be slightly negative
}

// advance adds a frame that took frameTime seconds and returns how many updates
// to run for it.
func (c *clock) advance(frameTime float64) int {
	if frameTime < 0 {
		frameTime = 0
	}
	c.pending += frameTime
	updates := int((c.pending + stepTolerance) / updateStep)
	if updates > maxUpdatesPerFrame {
		c.pending = 0
		return maxUpdatesPerFrame
	}
	c.pending -= float64(updates) * updateStep
	return updates
}
