package main

import (
	"errors"
	"fmt"
	"os"
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
	"github.com/FyshOS/fyles/pkg/archive"
	"github.com/FyshOS/fyles/pkg/fyles"

	xWidget "fyne.io/x/fyne/widget"
)

// treeID maps a URI to the identifier that addresses it in the directory tree.
// Locations inside the home directory appear under a root of their own.
func (ui *fylesUI) treeID(u fyne.URI) widget.TreeNodeID {
	id := u.String()
	if u.Scheme() != "file" && !archive.Within(u) {
		return id // not a local path, such as a favourite under the tree scheme
	}

	path := u.Path()
	if ui.homeRoot != "" && (path == ui.homeDir || strings.HasPrefix(path, ui.homeDir+"/")) {
		return ui.homeRoot + strings.TrimPrefix(path, ui.homeDir)
	}

	if archive.Within(u) {
		id = storage.NewFileURI(path).String()
	}
	if id[len(id)-1] == '/' && id != "file:///" {
		id = id[:len(id)-1]
	}
	return id
}

// fileURI is the inverse of treeID, resolving a tree node (favourite or home)
// to the file URI of the directory it represents. Content of archives is
// addressed by path too, see archive.Resolve for mapping it back.
func (ui *fylesUI) fileURI(u fyne.URI) fyne.URI {
	if u.Scheme() != "tree" {
		return u
	}

	path := u.Path()
	if ui.homeRoot != "" {
		if rest, ok := strings.CutPrefix(path, "/Favourites"); ok {
			path = rest
		} else if rest, ok := strings.CutPrefix(path, "/Home"); ok {
			path = rest
		}
		path = ui.homeDir + path
	}
	return storage.NewFileURI(path)
}

// setDirectory changes the current directory to u, which may be either a file
// URI or a node of the tree (keeping that node selected rather than its twin
// under the home directory).
func (ui *fylesUI) setDirectory(u fyne.URI) {
	node := ui.treeID(u)
	u = archive.Resolve(ui.fileURI(u))

	ui.pwd = u
	if nodeURI, err := storage.ParseURI(node); err == nil {
		openParent(ui.fileTree, nodeURI)
	}
	ui.fileTree.Select(node)
	ui.items.SetDir(u)
	ui.fileScroll.ScrollToTop()
	ui.filePath.SetText(u.Path())
	ui.win.SetTitle(winTitle + " : " + u.Name())
	if archive.Within(u) {
		ui.newFolder.Disable() // archives are read-only
	} else {
		ui.newFolder.Enable()
	}
}

func (ui *fylesUI) itemTapped(u fyne.URI) {
	if u == nil {
		return
	}

	// an archive is read on first use, so keep that off the UI thread
	go ui.enterOrOpen(archive.Resolve(u))
}

// enterOrOpen changes to u when it can be listed and launches it otherwise.
// It may block on the file system, so it runs off the UI thread and hands
// the results back with fyne.Do.
func (ui *fylesUI) enterOrOpen(u fyne.URI) {
	listable, err := storage.CanList(u)
	if err == nil && listable {
		// show it is selected then change
		time.Sleep(canvas.DurationShort)
		ui.enter(u)
		return
	}

	err = fyles.OpenWithProgress(u, ui.win) // extracts from an archive when needed
	// show it is selected then change
	time.Sleep(canvas.DurationShort)
	fyne.Do(func() {
		ui.items.ClearSelection()
		if err != nil {
			dialog.ShowError(err, ui.win)
		}
	})
}

// enter changes to u, which may be a tree node, after reading the archive it
// points into. Reading can take a while, so an activity dialog is shown in
// the meantime and this must run off the UI thread.
func (ui *fylesUI) enter(u fyne.URI) {
	err := fyles.PrepareArchive(archive.Resolve(ui.fileURI(u)), ui.win)

	fyne.Do(func() {
		if err != nil {
			ui.items.ClearSelection()
			dialog.ShowError(err, ui.win)
			return
		}
		ui.setDirectory(u)
	})
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
	ui.homeDir, ui.homeRoot = homeDir, homeRoot
	files := xWidget.NewFileTree(root)
	var faves []string
	addFave := func(name string, icon fyne.Resource) {
		if homeDir == "" {
			return
		}
		if info, err := os.Stat(filepath.Join(homeDir, name)); err != nil || !info.IsDir() {
			return // only show favourites that exist
		}

		id := favID + "/" + name
		faves = append(faves, id)
		mapIcon(files, id, icon)
	}
	addFave("Documents", theme.DocumentIcon())
	addFave("Downloads", theme.DownloadIcon())
	addFave("Music", theme.MediaMusicIcon())
	addFave("Pictures", theme.MediaPhotoIcon())
	addFave("Videos", theme.MediaVideoIcon())

	files.Filter = filterDir(ui.filter)
	files.Sorter = func(u1, u2 fyne.URI) bool {
		return u1.String() < u2.String() // Sort alphabetically
	}
	files.Root = ""
	listID := func(id widget.TreeNodeID) string {
		if homeRoot != "" && strings.HasPrefix(id, homeRoot) {
			id = strings.Replace(id, homeRoot, "file://"+homeDir, 1)
		}
		u, err := storage.ParseURI(id)
		if err != nil {
			return id
		}
		return archive.Resolve(u).String()
	}
	nodeID := func(listed string) widget.TreeNodeID {
		id := listed
		if rest, ok := strings.CutPrefix(listed, "archive://"); ok {
			id = "file://" + rest
		}
		if homeRoot != "" {
			id = strings.Replace(id, "file://"+homeDir, homeRoot, 1)
		}
		return id
	}
	origChildren := files.ChildUIDs
	files.ChildUIDs = func(id widget.TreeNodeID) []widget.TreeNodeID {
		if id == "" {
			return base
		} else if id == favID {
			return faves
		}

		listed := origChildren(listID(id))
		items := make([]widget.TreeNodeID, len(listed))
		for i, item := range listed {
			items[i] = nodeID(item)
		}
		return items
	}
	origBranch := files.IsBranch
	files.IsBranch = func(id widget.TreeNodeID) bool {
		if id == "" || id == favID {
			return true
		}

		return origBranch(listID(id))
	}
	origUpdate := files.UpdateNode
	files.UpdateNode = func(id widget.TreeNodeID, branch bool, node fyne.CanvasObject) {
		origUpdate(id, branch, node)
		if !branch {
			return
		}

		u, err := storage.ParseURI(id)
		if err == nil && archive.IsFile(ui.fileURI(u)) {
			node.(*fyne.Container).Objects[1].(*widget.Icon).SetResource(archive.Icon())
		}
	}

	files.OnSelected = func(uid widget.TreeNodeID) {
		u, err := storage.ParseURI(uid)
		if err != nil {
			fyne.LogError("Invalid tree node "+uid, err)
			return
		}

		if !archive.Prepared(archive.Resolve(ui.fileURI(u))) {
			go ui.enter(u) // reads the archive first, showing progress
			return
		}
		ui.setDirectory(u)
	}
	files.OnBranchOpened = func(uid widget.TreeNodeID) {
		u, err := storage.ParseURI(uid)
		if err != nil {
			return
		}
		target := archive.Resolve(ui.fileURI(u))
		if archive.Prepared(target) {
			return
		}

		// Listing the branch would read the whole archive on the UI thread,
		// so keep it closed until that has been done with progress shown.
		files.CloseBranch(uid)
		go func() {
			err := fyles.PrepareArchive(target, ui.win)
			fyne.Do(func() {
				if err != nil {
					dialog.ShowError(err, ui.win)
					return
				}
				files.OpenBranch(uid)
			})
		}()
	}

	open, _ := storage.ParseURI(ui.treeID(u))
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
			createFolderErr := os.MkdirAll(newFolderPath, 0o750)
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
	ui.newFolder = newFolderButton

	return container.NewBorder(nil, nil, widget.NewToolbar(
		widget.NewToolbarAction(theme.HomeIcon(), func() {
			home, err := os.UserHomeDir()
			if err != nil {
				return
			}
			ui.setDirectory(storage.NewFileURI(home))
		}),
	), newFolderButton,
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

type favRepo struct{}

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
