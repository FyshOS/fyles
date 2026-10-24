package fyles

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"

	"github.com/FyshOS/fyles/pkg/archive"
)

func writeTestZip(t *testing.T, path string, names ...string) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, n := range names {
		f, err := w.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte("content of " + n))
	}
	w.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func names(items []*fileData) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.name
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPanelListsArchiveAsFolder(t *testing.T) {
	test.NewTempApp(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0o644)
	os.WriteFile(filepath.Join(dir, "fyles-1.2.zip"), []byte("not really"), 0o644)
	zipPath := filepath.Join(dir, "sample.tar.gz.zip")
	writeTestZip(t, zipPath, "docs/read me.txt", "docs/notes", "top.go")

	p := NewFylesPanel(func(fyne.URI) {}, nil)
	p.SetDir(storage.NewFileURI(dir))

	var found *fileData
	for _, item := range p.items {
		if item.location.Path() == zipPath {
			found = item
		}
	}
	if found == nil {
		t.Fatal("archive not listed")
	}
	if found.name != "sample.tar.gz" || !found.dir || !archive.IsFile(found.location) {
		t.Errorf("archive listed as %+v", found)
	}
	if archive.Within(found.location) {
		t.Error("listed archives keep their file URI, so they can be renamed or deleted")
	}
	if found.inArchive() {
		t.Error("the archive file itself can still be renamed or deleted")
	}

	// binding an item to its widget must not trim the name any further
	item := &fileItem{parent: p}
	item.ExtendBaseWidget(item)
	for _, d := range p.items {
		item.setData(d)
		item.setData(d)
	}
	if got := names(p.items); !equal(got, []string{"(Parent)", "fyles-1.2", "notes", "sample.tar.gz"}) {
		t.Errorf("names after binding = %v", got)
	}

	p.SetDir(found.location)
	if got := names(p.items); !equal(got, []string{"(Parent)", "docs", "top"}) {
		t.Errorf("archive listing = %v", got)
	}
	if !p.items[1].inArchive() {
		t.Error("entries inside an archive are read-only")
	}
	if parent := p.items[0].location; parent.Path() != dir {
		t.Errorf("(Parent) of the archive = %v", parent)
	}
}
