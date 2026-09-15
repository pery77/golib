package golib

import (
	"math/rand/v2"
	"os"
	"sync"
)

// shotSeed is the seed random numbers start from when golib shot runs a game,
// so the same shot gives the same pictures.
const shotSeed = 1

// random is the generator behind RandomInt and RandomFloat. It is seeded when
// the program starts, before main, so random numbers used to build the first
// scene repeat in golib shot too.
var random = struct {
	sync.Mutex
	generator *rand.Rand
}{generator: newGenerator(startSeed(os.Getenv))}

// startSeed returns the seed random numbers start from: shotSeed when golib
// shot runs the game, and a different seed on every other run.
func startSeed(getenv func(string) string) uint64 {
	if getenv(shotFramesEnv) != "" {
		return shotSeed
	}
	return rand.Uint64()
}

func newGenerator(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed))
}

// RandomInt returns a random whole number from low to high, both included:
//
//	roll := golib.RandomInt(1, 6) // a die
//
// The numbers differ on every run, except under golib shot, which always starts
// from the same seed so that screenshots repeat. Use RandomInt and RandomFloat
// rather than math/rand, or shots stop repeating. If high is less than low,
// the two swap.
func RandomInt(low, high int) int {
	if high < low {
		low, high = high, low
	}
	random.Lock()
	defer random.Unlock()
	return low + random.generator.IntN(high-low+1)
}

// RandomFloat returns a random number from low up to, but not including, high:
//
//	x := golib.RandomFloat(0, screen.Width())
//
// Like RandomInt, it repeats under golib shot.
func RandomFloat(low, high float32) float32 {
	random.Lock()
	defer random.Unlock()
	return low + random.generator.Float32()*(high-low)
}

// SetRandomSeed restarts RandomInt and RandomFloat from seed: the same seed
// gives the same numbers in the same order. Call it at the start of a test
// that uses random numbers, so the test repeats, or in a game that wants the
// same "random" level every time.
func SetRandomSeed(seed uint64) {
	random.Lock()
	defer random.Unlock()
	random.generator = newGenerator(seed)
}
