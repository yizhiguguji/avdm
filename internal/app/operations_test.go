package app

import (
	"slices"
	"testing"
)

func TestEmulatorLaunchArgsUseEmbeddedRealtimeMode(t *testing.T) {
	args := emulatorLaunchArgs("demo_test", 35554)
	required := []string{
		"-avd",
		"demo_test",
		"-use-keycode-forwarding",
		"-grpc",
		"35554",
		"-grpc-use-token",
		"-idle-grpc-timeout",
		"300",
	}
	for _, want := range required {
		if !slices.Contains(args, want) {
			t.Fatalf("launch args missing %q: %#v", want, args)
		}
	}
	if slices.Contains(args, "-qt-hide-window") {
		t.Fatalf("launch args should expose native emulator windows: %#v", args)
	}
}
