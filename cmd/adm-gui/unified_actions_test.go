package main

import (
	core "adm/internal/app"
	"reflect"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

func menuLabels(menu *fyne.Menu) []string {
	var labels []string
	for _, item := range menu.Items {
		if !item.IsSeparator {
			labels = append(labels, item.Label)
		}
	}
	return labels
}

func TestCompleteMenusKeepOrderAndSelectionScope(t *testing.T) {
	g := testWall(t)
	want := []string{"视图", "创建模拟器", "选择", "选中设备操作", "诊断"}
	if got := menuLabels(g.workbenchMoreMenu()); !reflect.DeepEqual(got, want) {
		t.Fatalf("wrong menu groups: %v", got)
	}
	// Both visible batch overflow and top menu must use the same complete model.
	top := g.workbenchMoreMenu().Items[3].ChildMenu
	if !reflect.DeepEqual(menuLabels(top), menuLabels(g.selectedDeviceMenu())) {
		t.Fatal("batch menus disagree")
	}
	g.window = g.app.NewWindow("选择范围")
	g.window.SetContent(g.controlGrid)
	g.window.Show()
	t.Cleanup(g.window.Close)
	g.entries = []core.DeviceEntry{{Key: "main", Active: &core.ActiveDevice{Serial: "PHONE", State: "device"}}}
	g.currentDeviceKey = "main"
	called := false
	g.requireSelection(func() { called = true })()
	if called {
		t.Fatal("selected-only menu silently used main device")
	}
}

func TestBatchBarPreservesFullActionLabelsAtNarrowWidths(t *testing.T) {
	g := testWall(t)
	bar := g.buildBatchActionBar()
	var row *fyne.Container
	visitCardObjects(bar, func(object fyne.CanvasObject) {
		if c, ok := object.(*fyne.Container); ok {
			if _, ok := c.Layout.(batchActionLayout); ok {
				row = c
			}
		}
	})
	if row == nil {
		t.Fatal("missing independent batch row")
	}
	labels := []string{"启动模拟器", "关闭模拟器", "删除模拟器", "清除选择", "更多操作"}
	for _, width := range []float32{1000, 480, 320, 1000} {
		row.Resize(fyne.NewSize(width, 36))
		row.Refresh()
		for i, object := range row.Objects {
			if i > 0 {
				var label string
				switch b := object.(type) {
				case *widget.Button:
					label = b.Text
				case *readableButton:
					label = b.Text
				}
				if label != labels[i-1] {
					t.Fatalf("action label changed: %q", label)
				}
			}
			if object.Visible() && object.Position().X+object.Size().Width > width+.1 {
				t.Fatalf("width %.0f: action clips: %v %v", width, object.Position(), object.Size())
			}
		}
		if !row.Objects[5].Visible() {
			t.Fatal("overflow became inaccessible")
		}
		if width == 1000 {
			for _, object := range row.Objects {
				if !object.Visible() {
					t.Fatal("full-width action did not return")
				}
			}
		}
	}
}

func TestBatchRowRetainsHeightWhenLogsToggle(t *testing.T) {
	g := testWall(t)
	g.busyLabel = widget.NewLabel("任务：空闲")
	g.toolSummary = widget.NewLabel("工具：已就绪")
	g.currentLabel = widget.NewLabel("主目标：未选择")
	g.progress = widget.NewActivity()
	g.logRail = g.buildLogRail()
	g.logCollapsed = true
	baseline := g.logPaneHeight(800)
	g.entries = []core.DeviceEntry{testWallEntry("one", "PHONE")}
	g.controlSelected = map[string]bool{"one": true}
	g.updateSelectedLabel()
	expanded := g.logPaneHeight(800)
	if expanded <= baseline || expanded < g.logRail.MinSize().Height {
		t.Fatal("batch row was squeezed into status height")
	}
	g.logCollapsed = false
	if g.logPaneHeight(800) < expanded+120 {
		t.Fatal("logs consumed the batch row")
	}
	g.clearWorkbenchSelection()
	if g.selectionActions.Visible() {
		t.Fatal("empty selection retained batch row")
	}
}

func TestDeviceMenuNamesScopeAndNeverOffersPhoneDeletion(t *testing.T) {
	g := testWall(t)
	entry := testWallEntry("phone", "PHONE")
	g.buildControlCard(entry, g.controlDensitySpec())
	labels := strings.Join(menuLabels(g.deviceCardMenu(g.controlCards[entry.Key])), "|")
	for _, want := range []string{"管理此设备", "重启此设备", "关闭此真机"} {
		if !strings.Contains(labels, want) {
			t.Fatal("missing scoped action: " + want)
		}
	}
	if strings.Contains(labels, "删除") || strings.Contains(labels, "…") {
		t.Fatal("phone deletion or abbreviated action exposed")
	}
	entry.AVD = &core.AVD{Name: "Demo_AVD"}
	entry.Active.IsEmulator = true
	g.controlCards[entry.Key].entry = entry
	if !strings.Contains(strings.Join(menuLabels(g.deviceCardMenu(g.controlCards[entry.Key])), "|"), "删除此模拟器") {
		t.Fatal("emulator lifecycle menu incomplete")
	}
}
