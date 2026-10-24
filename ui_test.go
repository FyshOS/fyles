package main

import (
	"archive/zip"
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"

	"github.com/FyshOS/fyles/pkg/archive"
	"github.com/FyshOS/fyles/pkg/fyles"
)

// savePNG writes a capture of the window when FYLES_SHOTS names a folder,
// so the rendering can be checked by eye.
func savePNG(t *testing.T, w fyne.Window, name string) {
	dir := os.Getenv("FYLES_SHOTS")
	if dir == "" {
		return
	}
	var buf bytes.Buffer
	png.Encode(&buf, w.Canvas().Capture())
	os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644)
}

func writeTestZip(t *testing.T, path string) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range []string{"src/main.go", "src/lib/util.go", "README"} {
		f, _ := zw.Create(n)
		f.Write([]byte("content of " + n))
	}
	zw.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// tap enters u the way a tap in the panel does. The test driver runs
// fyne.Do inline, so the change has been applied when it returns.
func tap(ui *fylesUI, u fyne.URI) {
	do(func() { ui.enterOrOpen(archive.Resolve(u)) })
}

// do runs f on the UI thread, as the widgets expect.
func do(f func()) {
	fyne.DoAndWait(f)
}

func TestArchiveInPanelAndTree(t *testing.T) {
	test.NewTempApp(t)
	t.Cleanup(archive.Cleanup)
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "plain"), 0o755)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0o644)
	zipPath := filepath.Join(dir, "project.zip")
	writeTestZip(t, zipPath)

	w := test.NewTempWindow(t, nil)
	ui := &fylesUI{win: w, filter: filterHidden()}
	do(func() {
		w.Resize(fyne.NewSize(640, 400))
		tools := ui.makeToolbar()
		ui.items = fyles.NewFylesPanel(ui.itemTapped, w)
		ui.items.Filter = ui.filter
		ui.fileScroll = container.NewScroll(container.NewStack(newDirTapPanel(ui), ui.items))
		ui.fileTree = ui.makeFilesPanel(storage.NewFileURI(dir))
		ui.setDirectory(storage.NewFileURI(dir))
		split := container.NewHSplit(ui.fileTree, ui.fileScroll)
		split.Offset = 0.3
		w.SetContent(container.NewBorder(tools, nil, nil, nil, split))
		savePNG(t, w, "1-folder.png")
	})

	// the tree lists the zipPath as a branch below the folder
	dirID := storage.NewFileURI(dir).String()
	treeID := storage.NewFileURI(zipPath).String()
	var kids []string
	do(func() {
		if kids := ui.fileTree.ChildUIDs(dirID); len(kids) != 2 || kids[0] != dirID+"/plain" || kids[1] != treeID {
			t.Errorf("folder tree children = %v", kids)
		}
		if !ui.fileTree.IsBranch(treeID) {
			t.Error("zipPath should be a tree branch")
		}
		kids = ui.fileTree.ChildUIDs(treeID)
		if len(kids) != 1 || kids[0] != treeID+"/src" {
			t.Errorf("zipPath tree children = %v", kids)
		}
		if !ui.fileTree.IsBranch(kids[0]) {
			t.Error("folder inside zipPath should be a branch")
		}
	})

	// tapping the zipPath in the panel enters it
	tap(ui, storage.NewFileURI(zipPath))
	do(func() {
		if ui.pwd.Path() != zipPath || !archive.Within(ui.pwd) {
			t.Fatalf("did not enter zipPath, pwd = %v", ui.pwd)
		}
		if !ui.newFolder.Disabled() {
			t.Error("new folder should be disabled inside an archive")
		}
		if !ui.fileTree.IsBranchOpen(dirID) {
			t.Error("tree should open the folder holding the selected archive")
		}
		ui.fileTree.OpenBranch(treeID)
		ui.fileTree.OpenBranch(kids[0])
		savePNG(t, w, "2-archive.png")

		// selecting a folder in the tree enters it
		ui.fileTree.Select(treeID + "/src/lib")
		if ui.pwd.Path() != filepath.Join(zipPath, "src/lib") || !archive.Within(ui.pwd) {
			t.Errorf("tree selection went to %v", ui.pwd)
		}
		savePNG(t, w, "3-nested.png")
	})

	// (Parent) walks back out through the zipPath
	for _, want := range []string{filepath.Join(zipPath, "src"), zipPath, dir} {
		var parent fyne.URI
		do(func() {
			var err error
			if parent, err = storage.Parent(ui.pwd); err != nil {
				t.Fatal(err)
			}
		})
		tap(ui, parent)
		do(func() {
			if ui.pwd.Path() != want {
				t.Errorf("parent went to %v, want %s", ui.pwd, want)
			}
		})
	}
	do(func() {
		if archive.Within(ui.pwd) {
			t.Error("should have left the archive")
		}
		if ui.newFolder.Disabled() {
			t.Error("new folder should be enabled again")
		}
	})
}
