package fyles

import (
	"errors"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestWithActivity(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, widget.NewLabel("content"))

	ran := false
	err := withActivity(w, "quick", "Opening archive...", func() error {
		ran = true
		return nil
	})
	if err != nil || !ran {
		t.Errorf("quick work: ran=%v err=%v", ran, err)
	}

	want := errors.New("boom")
	err = withActivity(w, "slow", "Opening archive...", func() error {
		time.Sleep(3 * activityDelay) // long enough for the dialog to appear
		return want
	})
	if err != want {
		t.Errorf("slow work returned %v", err)
	}
	fyne.DoAndWait(func() {
		if w.Canvas().Overlays().Top() != nil {
			t.Error("dialog left on screen after the work finished")
		}
	})
}
