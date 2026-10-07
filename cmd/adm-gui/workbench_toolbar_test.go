package main

import (
	core "adm/internal/app"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
)

func toolbarTestEntries() []core.DeviceEntry {
	return []core.DeviceEntry{
		{Key: "phone", Label: "Pixel Phone", Active: &core.ActiveDevice{Serial: "SERIAL-A", State: "device", Details: map[string]string{"model": "Pixel_9"}}},
		{Key: "live", Label: "Online AVD", AVD: &core.AVD{Name: "Live_AVD"}, Running: true, Active: &core.ActiveDevice{Serial: "emulator-5554", State: "device", IsEmulator: true}},
		{Key: "stopped", Label: "Stopped AVD", AVD: &core.AVD{Name: "Stopped_AVD"}},
		{Key: "starting", Label: "Starting AVD", AVD: &core.AVD{Name: "Starting_AVD"}, Running: true},
		{Key: "offline", Label: "Offline AVD", AVD: &core.AVD{Name: "Offline_AVD"}, Running: true, Active: &core.ActiveDevice{State: "offline", IsEmulator: true}},
		{Key: "unauthorized", Label: "Unauthorized Phone", Active: &core.ActiveDevice{State: "unauthorized"}},
	}
}

func TestActionPlanExplicitUnusableSelectionNeverExpands(t *testing.T) {
	entries := toolbarTestEntries()
	for _, selected := range [][]core.DeviceEntry{{entries[2]}, nil} {
		plan := makeActionPlan(actionMirror, TargetSelection{Entries: selected, Explicit: true}, entries, true)
		if len(plan.Entries) != 0 || !plan.Explicit {
			t.Fatalf("explicit unusable/missing selection expanded: %+v", plan)
		}
	}
	plan := makeActionPlan(actionMirror, TargetSelection{}, entries, true)
	if len(plan.Entries) != 2 || plan.Explicit {
		t.Fatalf("unselected mirror should use all ready devices: %+v", plan)
	}
}

func TestActionPlanSeparatesTargetsAndSkipReasons(t *testing.T) {
	entries := toolbarTestEntries()
	plan := makeActionPlan(actionStart, TargetSelection{Entries: entries, Explicit: true}, entries, false)
	if len(plan.Entries) != 1 || plan.Entries[0].Key != "stopped" || len(plan.Skipped) != 5 {
		t.Fatalf("wrong start plan: %+v", plan)
	}
	if strings.Join(plan.avdNames(), ",") != "Stopped_AVD" || !strings.Contains(plan.details(), "模拟器已启动") || !strings.Contains(plan.details(), "真机无法启动模拟器") {
		t.Fatalf("plan must identify targets and explain exclusions: %s", plan.details())
	}
	stop := makeActionPlan(actionStop, TargetSelection{Entries: entries, Explicit: true}, entries, false)
	if strings.Join(stop.keys(), ",") != "live,starting,offline" {
		t.Fatalf("stop must include starting and offline emulators without including phones: %+v", stop)
	}
}

func TestPrimaryTargetRequiresExactlyOneSelectedReadyDevice(t *testing.T) {
	entries := toolbarTestEntries()
	for _, selected := range [][]core.DeviceEntry{entries[:2], {entries[0], entries[2]}, {entries[2]}} {
		plan := makeActionPlan(actionSetTarget, TargetSelection{Entries: selected, Explicit: true}, entries, false)
		if len(plan.Entries) != 0 {
			t.Fatalf("ambiguous/unusable target accepted: %+v", plan)
		}
	}
	plan := makeActionPlan(actionSetTarget, TargetSelection{Entries: entries[:1], Explicit: true}, entries, false)
	if len(plan.Entries) != 1 || plan.Entries[0].Key != "phone" {
		t.Fatalf("single ready target rejected: %+v", plan)
	}
}

func TestWallFilteringAndStateSummary(t *testing.T) {
	entries := toolbarTestEntries()
	if got := filterWallEntries(entries, "serial-a", wallStateReady); len(got) != 1 || got[0].Key != "phone" {
		t.Fatalf("serial search failed: %+v", got)
	}
	if got := filterWallEntries(entries, "PIXEL_9", wallStateAll); len(got) != 1 {
		t.Fatalf("case-insensitive model search failed: %+v", got)
	}
	if got := filterWallEntries(entries, "", wallStateStopped); len(got) != 1 || got[0].Key != "stopped" {
		t.Fatalf("starting/offline devices must not be stopped: %+v", got)
	}
	want := "共 6 项 · 在线 2 · 未启动 1 · 启动中 1 · 异常 2"
	if got := controlSummaryText(entries); got != want {
		t.Fatalf("non-exclusive or wrong summary: %s", got)
	}
}

func TestWorkbenchToolbarFitsOneRowAndRestoresActions(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	objects := make([]fyne.CanvasObject, 8)
	for i := range objects {
		objects[i] = &canvas.Rectangle{}
	}
	layout := workbenchToolbarLayout{}
	// A 1280px window with a 380px tool panel leaves more than 700px.
	for _, width := range []float32{760, 320, 760} {
		layout.Layout(objects, fyne.NewSize(width, 36))
		for i, object := range objects {
			if object.Visible() && (object.Position().Y != 0 || object.Position().X+object.Size().Width > width) {
				t.Fatalf("button %d escaped one-row toolbar at %.0fpx: %v %v", i, width, object.Position(), object.Size())
			}
		}
		for _, index := range []int{0, 2, 7} {
			if !objects[index].Visible() {
				t.Fatalf("scan, view and overflow must remain visible: %d", index)
			}
		}
		if width == 760 {
			for i, object := range objects {
				if !object.Visible() {
					t.Fatalf("all common actions should return at normal width: %d", i)
				}
			}
		}
	}
}

func TestUnselectedStartDoesNotUseDefaultScope(t *testing.T) {
	plan := makeActionPlan(actionStart, TargetSelection{}, toolbarTestEntries(), false)
	if len(plan.Entries) != 0 || plan.Default || !strings.Contains(plan.summary(), "未选择") {
		t.Fatalf("unselected start expanded or reported all devices: %+v", plan)
	}
}

func TestTargetSelectionKeepsExplicitStaleKeys(t *testing.T) {
	g := &GUIApp{entries: toolbarTestEntries(), controlSelected: map[string]bool{"disappeared": true}}
	plan := g.planSelectedAction(actionMirror, true)
	if !plan.Explicit || plan.Default || len(plan.Entries) != 0 {
		t.Fatalf("stale selection must not silently mirror other devices: %+v", plan)
	}
}

func TestRailActionSupportsKeyboardActivation(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	calls := 0
	button := newWorkbenchRailButton("安装", nil, func() { calls++ })
	var focusable fyne.Focusable = button
	focusable.FocusGained()
	focusable.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	focusable.FocusLost()
	if calls != 1 || button.Text != "安装" {
		t.Fatalf("labeled rail action must support keyboard activation: calls=%d label=%q", calls, button.Text)
	}
}
