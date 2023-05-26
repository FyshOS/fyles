package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	xWidget "fyne.io/x/fyne/widget"
)

func (ui *fylesUI) setDirectory(u fyne.URI) {
	ui.pwd = u
	ui.itemTapped(nil)
	ui.items.SetDir(u)
	ui.fileScroll.ScrollToTop()
	ui.filePath.SetText(u.Path())
	ui.win.SetTitle(winTitle + " : " + u.Name())
}

func (ui *fylesUI) itemTapped(u fyne.URI) {
	if u == nil {
		return
	}

	listable, err := storage.CanList(u)
	if err == nil && listable {
		go func() {
			// show it is selected then change
			time.Sleep(time.Millisecond * 120)
			ui.setDirectory(u)
		}()
		return
	}
}

func (ui *fylesUI) makeFilesPanel(u fyne.URI) *xWidget.FileTree {
	vol := filepath.VolumeName(u.Path())
	if vol == "" {
		vol = "/"
	}
	root := storage.NewFileURI(vol)
	files := xWidget.NewFileTree(root)
	files.Filter = filterDir(ui.filter)
	files.Sorter = func(u1, u2 fyne.URI) bool {
		return u1.String() < u2.String() // Sort alphabetically
	}

	files.OnSelected = func(uid widget.TreeNodeID) {
		u, _ := storage.ParseURI(uid)
		ui.setDirectory(u)
	}

	openParent(files, u)
	files.Select(u.String())
	return files
}

func openParent(files *xWidget.FileTree, path fyne.URI) {
	parent, err := storage.Parent(path)
	if err != nil {
		return
	}

	if !files.IsBranchOpen(parent.String()) {
		openParent(files, parent)
		id := parent.String()
		if strings.LastIndexByte(id, filepath.Separator) == len(id)-1 {
			id = id[:len(id)-1]
		}
		files.OpenBranch(id)
	}
}

func (ui *fylesUI) makeToolbar() *fyne.Container {
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
