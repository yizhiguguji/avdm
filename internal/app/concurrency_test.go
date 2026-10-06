package app

import (
	"fmt"
	"sync"
	"testing"
)

// TestConcurrentStateAccessRace hammers the guarded shared state
// (currentDevice + cfg Last* fields + config persistence) from many goroutines
// at once. Run with -race to prove the App.mu discipline is complete. HOME is
// redirected to a temp dir so the real ~/.adm/config.json is never touched.
func TestConcurrentStateAccessRace(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := &App{cfg: defaultConfig()}

	const workers = 8
	const iterations = 200
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				switch (w + j) % 6 {
				case 0:
					a.setCurrentDevice(&ActiveDevice{Serial: fmt.Sprintf("emulator-%d", j%4)})
				case 1:
					_ = a.currentDeviceSnapshot()
				case 2:
					a.clearCurrentDeviceIf(fmt.Sprintf("emulator-%d", j%4))
				case 3:
					a.rememberAPKSourceForGUI(fmt.Sprintf("apk-%d", j))
				case 4:
					_ = a.lastAPKSource()
				case 5:
					a.saveConfigQuietly()
				}
			}
		}(w)
	}
	wg.Wait()
}
