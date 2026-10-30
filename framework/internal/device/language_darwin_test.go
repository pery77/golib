//go:build !js

package device

import (
	"sync"
	"testing"
)

// SystemLanguages drains its autorelease pool on the thread that made it,
// however many goroutines ask at once: draining it on another one crashed
// the program now and then.
func TestSystemLanguagesFromManyGoroutines(t *testing.T) {
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			for range 200 {
				SystemLanguages()
			}
		})
	}
	wg.Wait()
}
