package app

import (
	"strconv"
	"strings"
)

// Same outer-frame coordinates, 6-point gaps and uniform height as AX tiling.
type ExternalWindowFrame struct{ X, Y, Width, Height float64 }

type scrcpyFallbackPlan struct {
	placements map[string]*scrcpyWindowPlacement
	failures   map[string]error
}

// Plan only healthy phones, retaining individual failures for the batch opener.
func planScrcpyFallback(keys []string, entries []DeviceEntry, readFrame func(DeviceEntry) (ExternalWindowFrame, error), layout func([]ExternalWindowFrame) []*scrcpyWindowPlacement) *scrcpyFallbackPlan {
	byKey := make(map[string]DeviceEntry, len(entries))
	for _, entry := range entries {
		byKey[entry.Key] = entry
	}
	plan := &scrcpyFallbackPlan{
		placements: map[string]*scrcpyWindowPlacement{},
		failures:   map[string]error{},
	}
	var phoneKeys []string
	var frames []ExternalWindowFrame
	for _, key := range normalizedUniqueNames(keys) {
		entry := byKey[key]
		if entry.Active != nil && entry.Active.IsEmulator || entry.AVD != nil && entry.Running {
			plan.placements[key] = &scrcpyWindowPlacement{} // retain native emulator permission recovery
			continue
		}
		if entry.Active == nil || entry.Active.State != "device" {
			continue
		}
		frame, err := readFrame(entry)
		if err != nil {
			plan.failures[key] = err
			continue
		}
		phoneKeys = append(phoneKeys, key)
		frames = append(frames, frame)
	}
	if len(frames) > 0 {
		for i, placement := range layout(frames) {
			plan.placements[phoneKeys[i]] = placement
		}
	}
	return plan
}

func (p *scrcpyFallbackPlan) open(key string, open func(string, *scrcpyWindowPlacement) error) error {
	if err := p.failures[key]; err != nil {
		return err
	}
	return open(key, p.placements[key])
}

func parseMirrorDisplaySize(output string) (int, int) {
	width, height := 0, 0
	for _, line := range strings.Split(output, "\n") {
		_, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		w, h, found := strings.Cut(strings.TrimSpace(value), "x")
		if !found {
			continue
		}
		parsedW, _ := strconv.Atoi(w)
		parsedH, _ := strconv.Atoi(h)
		if parsedW > 0 && parsedH > 0 {
			width, height = parsedW, parsedH
		}
	}
	return width, height
}
