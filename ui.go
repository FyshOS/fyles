package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	xWidget "fyne.io/x/fyne/widget"
)

func (ui *fylesUI) setDirectory(u fyne.URI) {
	ui.pwd = u
	dirStr := u.String()
	if dirStr[len(dirStr)-1] == '/' && dirStr != "file:///" {
		dirStr = dirStr[:len(dirStr)-1]
	}
	ui.fileTree.Select(dirStr)
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
			time.Sleep(canvas.DurationShort)
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
		if uid == "file://" {
			uid = "file:///"
		}
		u, _ := storage.ParseURI(uid)
		ui.setDirectory(u)
	}

	openParent(files, u)
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

	newFolderButton := widget.NewButtonWithIcon("", theme.FolderNewIcon(), func() {
		newFolderEntry := widget.NewEntry()
		dialog.ShowForm("New Folder", "Create Folder", "Cancel", []*widget.FormItem{
			{
				Text:   "Name",
				Widget: newFolderEntry,
			},
		}, func(s bool) {
			if !s || newFolderEntry.Text == "" {
				return
			}

			newFolderPath := filepath.Join(ui.pwd.Path(), newFolderEntry.Text)
			createFolderErr := os.MkdirAll(newFolderPath, 0750)
			if createFolderErr != nil {
				fyne.LogError(
					fmt.Sprintf("Failed to create folder with path %s", newFolderPath),
					createFolderErr,
				)
				dialog.ShowError(errors.New("folder cannot be created"), ui.win)
			}
			ui.items.SetDir(ui.pwd)
		}, ui.win)
	})
	newFolderButton.Importance = widget.LowImportance

	return container.NewBorder(nil, nil, widget.NewToolbar(
		widget.NewToolbarAction(theme.HomeIcon(), func() {
			home, err := os.UserHomeDir()
			if err != nil {
				return
			}
			ui.setDirectory(storage.NewFileURI(home))
		})), newFolderButton,
		container.NewHScroll(l))
}
