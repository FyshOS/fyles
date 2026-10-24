package fyles

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// activityDelay is how long work may take before an activity dialog appears.
const activityDelay = canvas.DurationStandard

// withActivity runs work on the calling goroutine, showing a spinner if it is slow.
// The calling goroutine must not be the UI thread, and the result is returned.
func withActivity(win fyne.Window, title, message string, work func() error) error {
	var mu sync.Mutex // the dialog is created and hidden on the UI thread, but the timer may fire anywhere
	var d dialog.Dialog
	var activity *widget.Activity
	finished := false

	timer := time.AfterFunc(activityDelay, func() {
		fyne.Do(func() {
			mu.Lock()
			defer mu.Unlock()
			if finished {
				return
			}

			activity = widget.NewActivity()
			activity.Start()
			d = dialog.NewCustomWithoutButtons(title,
				container.NewHBox(activity, widget.NewLabel(message)), win)
			d.Show()
		})
	})

	err := work()
	timer.Stop()
	fyne.DoAndWait(func() {
		mu.Lock()
		defer mu.Unlock()
		finished = true
		if d != nil {
			activity.Stop()
			d.Hide()
		}
	})
	return err
}
