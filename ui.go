package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/storage/repository"
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
			fyne.Do(func() {
				ui.setDirectory(u)
			})
		}()
		return
	}

	file := u.Name()
	if u.Scheme() == "file" {
		file = u.Path() // better specificity if we can
	}

	cmd := exec.Command("xdg-mime", "query", "filetype", file)
	mime, err := cmd.Output()
	if err != nil {
		dialog.ShowError(err, ui.win)
		return
	}

	cmd = exec.Command("xdg-mime", "query", "default", strings.TrimSpace(string(mime)))
	entry, err := cmd.Output()
	if err != nil {
		dialog.ShowError(err, ui.win)
		return
	}
	if strings.TrimSpace(string(entry)) == "" {
		return // should this show an error?
	}

	ui.items.ClearSelection()
	cmd = exec.Command("xdg-open", u.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		dialog.ShowError(err, ui.win)
	}
}

func (ui *fylesUI) makeFilesPanel(u fyne.URI) *xWidget.FileTree {
	vol := filepath.VolumeName(u.Path())
	if vol == "" {
		vol = "/"
	}
	root := storage.NewFileURI(vol)
	repository.Register("tree", &favRepo{})

	rootID := "tree:///"
	favID := rootID + "Favourites"
	base := []string{favID, root.String()}
	homeDir := ""
	homeRoot := ""
	if current, err := user.Current(); err == nil {
		homeDir = current.HomeDir
		homeRoot = rootID + "Home"
		base = []string{base[0], homeRoot, base[1]}
	}
	faves := []string{
		favID + "/Documents",
		favID + "/Downloads",
		favID + "/Music",
		favID + "/Pictures",
		favID + "/Videos",
	}

	files := xWidget.NewFileTree(root)
	mapIcon(files, favID+"/Documents", theme.DocumentIcon())
	mapIcon(files, favID+"/Downloads", theme.DownloadIcon())
	mapIcon(files, favID+"/Music", theme.MediaMusicIcon())
	mapIcon(files, favID+"/Pictures", theme.MediaPhotoIcon())
	mapIcon(files, favID+"/Videos", theme.MediaVideoIcon())

	files.Filter = filterDir(ui.filter)
	files.Sorter = func(u1, u2 fyne.URI) bool {
		return u1.String() < u2.String() // Sort alphabetically
	}
	files.Root = ""
	origChildren := files.ChildUIDs
	files.ChildUIDs = func(id widget.TreeNodeID) []widget.TreeNodeID {
		if id == "" {
			return base
		} else if id == favID {
			return faves
		}

		if strings.HasPrefix(id, homeRoot) {
			path := strings.Replace(id, homeRoot, "file://"+homeDir, 1)
			items := origChildren(path)
			for i, item := range items {
				items[i] = strings.Replace(item, "file://"+homeDir, homeRoot, 1)
			}
			return items
		}

		return origChildren(id)
	}
	origBranch := files.IsBranch
	files.IsBranch = func(id widget.TreeNodeID) bool {
		if id == "" || id == favID {
			return true
		}

		if strings.HasPrefix(id, homeRoot) {
			path := strings.Replace(id, homeRoot, "file://"+homeDir, 1)
			return origBranch(path)
		}

		return origBranch(id)
	}

	files.OnSelected = func(uid widget.TreeNodeID) {
		if len(uid) > len(favID) && uid[:len(favID)] == favID {
			uid = homeRoot + uid[len(favID):]
		}
		if strings.HasPrefix(uid, homeRoot) {
			path := strings.Replace(uid, homeRoot, "file://"+homeDir, 1)
			uid = path
		}

		if uid == "file://" {
			uid = "file:///"
		}
		u, _ := storage.ParseURI(uid)
		ui.setDirectory(u)
	}

	open := u
	if strings.HasPrefix(u.Path(), homeDir) {
		open, _ = storage.ParseURI("tree:///Home" + strings.TrimPrefix(u.Path(), homeDir))
	}
	openParent(files, open)
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
		if len(id) >= 1 && id[len(id)-1] == filepath.Separator {
			if len(id) >= 2 && id[len(id)-2] != filepath.Separator {
				id = id[:len(id)-1]
			}
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
			h := storage.NewFileURI(home)
			ui.setDirectory(h)
			ui.fileTree.Select(h.String())
		})), newFolderButton,
		container.NewHScroll(l))
}

type customURI struct {
	fyne.URI
	icon fyne.Resource
}

func (c *customURI) Icon() fyne.Resource {
	return c.icon
}

func mapIcon(files *xWidget.FileTree, uid string, icon fyne.Resource) {
	u, _ := storage.ParseURI(uid)

	mapping := &customURI{u, icon}
	files.MapURI(uid, mapping)
}

type favRepo struct {
}

func (f *favRepo) Exists(fyne.URI) (bool, error) {
	return true, nil
}

func (f *favRepo) Reader(fyne.URI) (fyne.URIReadCloser, error) {
	return nil, errors.New("just favourites")
}

func (f *favRepo) CanRead(fyne.URI) (bool, error) {
	return false, nil
}

func (f *favRepo) Destroy(string) {
}

func (f *favRepo) Parent(u fyne.URI) (fyne.URI, error) {
	path := u.Path()
	parent := filepath.Dir(path)
	if path == parent {
		return nil, repository.ErrURIRoot
	}
	return storage.ParseURI("tree://" + parent)
}

func (f *favRepo) Child(u fyne.URI, child string) (fyne.URI, error) {
	path := u.Path()
	return storage.ParseURI("tree://" + filepath.Join(path, child))
}
