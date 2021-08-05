//go:generate fyne bundle -o bundled.go Icon.png

package main

import (
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
)

func main() {
	a := app.NewWithID("io.fyne.fyles")
	a.SetIcon(resourceIconPng)
	w := a.NewWindow("Fyles")
	fileItemMin = fyne.NewSize(fileIconCellWidth, fileIconSize+fileTextSize+theme.Padding())

	path, _ := os.Getwd()
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	current := storage.NewFileURI(path)
	ui := &fyles{win: w, filter: filterHidden()}
	ui.items = container.NewGridWrap(fileItemMin)
	ui.fileScroll = container.NewScroll(ui.items)
	fileTree := ui.makeFilesPanel(current)
	ui.setDirectory(current)
	mainSplit := container.NewHSplit(fileTree, ui.fileScroll)
	mainSplit.Offset = 0.35

	w.SetContent(container.NewBorder(ui.makeToolbar(), nil, nil, nil, mainSplit))
	w.Resize(fyne.NewSize(640, 360))
	w.ShowAndRun()
}
