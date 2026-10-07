package main

import (
	core "adm/internal/app"
	"fmt"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type entryTaskResult struct {
	Entry core.DeviceEntry
	Err   error
}

func executeEntryBatch(entries []core.DeviceEntry, operation func(core.DeviceEntry) error) []entryTaskResult {
	results := make([]entryTaskResult, len(entries))
	slots := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, entry := range entries {
		wg.Add(1)
		go func(i int, entry core.DeviceEntry) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			results[i] = entryTaskResult{Entry: entry, Err: operation(entry)}
		}(i, entry)
	}
	wg.Wait()
	return results
}

func (g *GUIApp) runEntryBatchAction(name string, entries []core.DeviceEntry, operation func(core.DeviceEntry) error) {
	targets := append([]core.DeviceEntry(nil), entries...)
	task := g.beginTask(name)
	g.appendLog("INFO", "开始：%s，%d 个目标", name, len(targets))
	go func() {
		results := executeEntryBatch(targets, operation)
		fyne.Do(func() {
			g.endTask(task)
			if g.closed {
				return
			}
			var failed []core.DeviceEntry
			var lines []string
			for _, result := range results {
				if result.Err != nil {
					failed = append(failed, result.Entry)
					lines = append(lines, fmt.Sprintf("失败：%s：%v", result.Entry.Label, result.Err))
					g.appendLog("ERROR", "%s：%s：%v", name, result.Entry.Label, result.Err)
				} else {
					g.appendLog("DONE", "%s：%s", name, result.Entry.Label)
				}
			}
			g.appendLog("INFO", "%s：成功 %d，失败 %d", name, len(results)-len(failed), len(failed))
			g.refreshAsync(false)
			if len(failed) == 0 {
				return
			}
			body := g.dialogMessageContent(fmt.Sprintf("成功 %d，失败 %d。重试仅针对以下失败目标：\n\n%s", len(results)-len(failed), len(failed), strings.Join(lines, "\n")), fyne.NewSize(540, 220))
			var d dialog.Dialog
			close := widget.NewButton("关闭", func() { d.Hide() })
			retry := widget.NewButton("仅重试失败目标", func() { d.Hide(); g.runEntryBatchAction(name+"（重试）", failed, operation) })
			retry.Importance = widget.HighImportance
			d = dialog.NewCustomWithoutButtons("批量任务结果", container.NewBorder(nil, container.NewHBox(close, retry), nil, nil, body), g.activeDialogWindow())
			d.Show()
		})
	}()
}
