//go:generate fyne bundle -o bundled.go Icon.png

package main

import (
	"os"

	"github.com/fyshos/fyles/pkg/fyles"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
)

const winTitle = "Fyles"

func main() {
	a := app.NewWithID("com.fyshos.fyles")
	a.SetIcon(resourceIconPng)
	w := a.NewWindow(winTitle)
	w.SetPadded(false)

	panels := []fyne.CanvasObject{}
	addPanel := func(path string) {
		current := storage.NewFileURI(path)
		item := makePanel(current, w)
		panels = append(panels,
			container.NewStack(
				canvas.NewRectangle(theme.BackgroundColor()),
				item))
	}
	path, _ := os.Getwd()
	for i := 1; i < len(os.Args); i++ {
		addPanel(os.Args[i])
	}

	if len(panels) == 0 {
		addPanel(path)
	}

	bg := canvas.NewRectangle(theme.OverlayBackgroundColor())
	w.SetContent(container.NewStack(bg,
		container.NewGridWithColumns(len(panels), panels...)))
	w.Resize(fyne.NewSize(float32(15+(540*len(panels))), 310))

	changes := make(chan fyne.Settings)
	go func() {
		for range changes {
			bg.FillColor = theme.OverlayBackgroundColor()
			bg.Refresh()

			panelColor := theme.BackgroundColor()
			for _, p := range panels {
				bg := p.(*fyne.Container).Objects[0]
				bg.(*canvas.Rectangle).FillColor = panelColor
				bg.Refresh()
			}
		}
	}()
	a.Settings().AddChangeListener(changes)
	w.ShowAndRun()
}

func makePanel(dir fyne.URI, w fyne.Window) fyne.CanvasObject {
	ui := &fylesUI{win: w, filter: filterHidden()}
	tools := ui.makeToolbar()
	ui.items = fyles.NewFylesPanel(ui.itemTapped, w)
	ui.items.Filter = ui.filter
	tapper := newDirTapPanel(ui)
	ui.fileScroll = container.NewScroll(container.NewStack(tapper, ui.items))
	ui.fileTree = ui.makeFilesPanel(dir)
	ui.setDirectory(dir)
	mainSplit := container.NewHSplit(ui.fileTree, ui.fileScroll)
	mainSplit.Offset = 0.3
	return container.NewBorder(tools, nil, nil, nil, mainSplit)
}
