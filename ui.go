package main

import (
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	xWidget "fyne.io/x/fyne/widget"
)

func (ui *fyles) setDirectory(u fyne.URI) {
	ui.pwd = u
	ui.itemTapped(nil)

	var items []fyne.CanvasObject
	parent, err := storage.Parent(u)
	if err == nil {
		up := &fileItem{parent: ui, name: "(Parent)", location: parent, dir: true}
		up.ExtendBaseWidget(up)
		items = append(items, up)
	}
	list, err := storage.List(u)
	if err != nil {
		fyne.LogError("Could not read dir", err)
	} else {
		for _, item := range list {
			if !ui.filter.Matches(item) {
				continue
			}

			dir, _ := storage.CanList(item)
			items = append(items, newFileItem(item, dir, ui))
		}
	}

	ui.items.Objects = items
	ui.items.Refresh()
	ui.fileScroll.ScrollToTop()
	ui.filePath.SetText(u.Path())
	ui.win.SetTitle(winTitle + " : " + u.Name())
}

func (ui *fyles) itemTapped(item *fileItem) {
	if ui.current != nil {
		ui.current.isCurrent = false
		ui.current.Refresh()
	}

	if item == nil {
		return
	}
	if item.dir {
		ui.setDirectory(item.location)
		return
	}

	ui.current = item
	item.isCurrent = true
	item.Refresh()
}

func (ui *fyles) makeFilesPanel(u fyne.URI) *xWidget.FileTree {
	root := storage.NewFileURI("/") // TODO windows :(
	files := xWidget.NewFileTree(root)
	files.Filter = filterDir(ui.filter)
	files.Sorter = func(u1, u2 fyne.URI) bool {
		return u1.String() < u2.String() // Sort alphabetically
	}

	files.OnSelected = func(uid widget.TreeNodeID) {
		u, _ := storage.ParseURI(uid)
		ui.setDirectory(u)
	}

	files.Select(u.String())
	return files
}

func (ui *fyles) makeToolbar() *fyne.Container {
	l := widget.NewLabel("")
	ui.filePath = l

	return container.NewBorder(nil, nil, widget.NewToolbar(
		widget.NewToolbarAction(theme.HomeIcon(), func() {
			home, err := os.UserHomeDir()
			if err != nil {
				return
			}
			ui.setDirectory(storage.NewFileURI(home))
		})), nil,
		container.NewHScroll(l))
}
