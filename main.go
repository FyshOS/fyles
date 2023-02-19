//go:generate fyne bundle -o bundled.go Icon.png

package main

import (
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"

	"github.com/fyne-io/fyles/pkg/fyles"
)

const winTitle = "Fyles"

func main() {
	a := app.NewWithID("io.fyne.fyles")
	a.SetIcon(resourceIconPng)
	w := a.NewWindow(winTitle)

	path, _ := os.Getwd()
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	current := storage.NewFileURI(path)
	ui := &fylesUI{win: w, filter: filterHidden()}
	tools := ui.makeToolbar()
	ui.items = panel.NewFylesPanel(ui.itemTapped, w)
	tapper := newDirTapPanel(ui)
	ui.fileScroll = container.NewScroll(container.NewMax(tapper, ui.items))
	fileTree := ui.makeFilesPanel(current)
	ui.setDirectory(current)
	mainSplit := container.NewHSplit(fileTree, ui.fileScroll)
	mainSplit.Offset = 0.35

	w.SetContent(container.NewBorder(tools, nil, nil, nil, mainSplit))
	w.Resize(fyne.NewSize(640, 360))
	w.ShowAndRun()
}
