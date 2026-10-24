package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
)

type testEntry struct {
	name string
	body string // "" with a trailing slash in name means a folder
}

var testEntries = []testEntry{
	{"docs/", ""},
	{"docs/read me.txt", "hello"},
	{"docs/notes", "plain text with no extension\n"},
	{"bin/tool", "\x00\x01\x02binary"},
	{"img/deep/picture.png", "\x89PNG"},
	{"empty/", ""},
	{"../escape.txt", "must not leave the archive"},
	{"top.go", "package main"},
}

func writeZip(t *testing.T, path string) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range testEntries {
		f, err := w.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(e.body))
	}
	w.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTarGz(t *testing.T, path string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	w := tar.NewWriter(gz)
	for _, e := range testEntries {
		h := &tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.body)), Typeflag: tar.TypeReg}
		if e.name[len(e.name)-1] == '/' {
			h.Typeflag = tar.TypeDir
			h.Mode = 0o755
		}
		if e.name == "bin/tool" {
			h.Mode = 0o755
		}
		if err := w.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(e.body))
	}
	w.Close()
	gz.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func names(list []fyne.URI) []string {
	out := make([]string, len(list))
	for i, u := range list {
		out[i] = u.Name()
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

func TestArchiveBrowsing(t *testing.T) {
	dir := t.TempDir()
	archives := map[string]func(*testing.T, string){
		"sample.zip":    writeZip,
		"sample.tar.gz": writeTarGz,
	}
	for name, write := range archives {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(dir, name)
			write(t, file)
			onDisk := storage.NewFileURI(file)

			if !IsFile(onDisk) {
				t.Fatal("archive not recognised")
			}
			root := Resolve(onDisk)
			if !Within(root) || root.Path() != file {
				t.Fatalf("unexpected root %v", root)
			}
			if _, ok := root.(fyne.URIWithIcon); !ok {
				t.Error("archive root should carry the archive icon")
			}

			if listable, err := storage.CanList(root); err != nil || !listable {
				t.Fatalf("root not listable: %v", err)
			}
			list, err := storage.List(root)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"bin", "docs", "empty", "escape.txt", "img", "top.go"}
			if got := names(list); !equal(got, want) {
				t.Errorf("root listing = %v, want %v", got, want)
			}

			docs := Resolve(storage.NewFileURI(filepath.Join(file, "docs")))
			list, err = storage.List(docs)
			if err != nil {
				t.Fatal(err)
			}
			if got := names(list); !equal(got, []string{"notes", "read me.txt"}) {
				t.Errorf("docs listing = %v", got)
			}
			for _, u := range list {
				if listable, _ := storage.CanList(u); listable {
					t.Errorf("%v should not be listable", u)
				}
				if ok, _ := storage.Exists(u); !ok {
					t.Errorf("%v should exist", u)
				}
				mime := u.MimeType()
				if u.Name() == "notes" && mime != "text/plain" {
					t.Errorf("notes mime = %s", mime)
				}
				if u.Name() == "read me.txt" && mime != "text/plain" {
					t.Errorf("read me.txt mime = %s", mime)
				}
			}

			bin, _ := storage.Child(root, "bin")
			list, err = storage.List(bin)
			if err != nil || len(list) != 1 {
				t.Fatalf("bin listing = %v, %v", list, err)
			}
			tool := list[0]
			if tool.MimeType() == "text/plain" {
				t.Error("binary content reported as text")
			}
			missing, _ := storage.Child(root, "nope")
			if ok, _ := storage.Exists(missing); ok {
				t.Error("missing entry reported as existing")
			}
			escape, _ := storage.Child(root, "escape.txt")
			if ok, _ := storage.Exists(escape); !ok {
				t.Error("entry escaping the archive should be kept at its root")
			}

			r, err := storage.Reader(tool)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(r)
			r.Close()
			if string(body) != "\x00\x01\x02binary" {
				t.Errorf("read %q", body)
			}

			parent, err := storage.Parent(root)
			if err != nil || parent.Path() != dir {
				t.Errorf("parent of root = %v, %v", parent, err)
			}
			parent, err = storage.Parent(docs)
			if err != nil || parent.String() != root.String() {
				t.Errorf("parent of docs = %v, %v", parent, err)
			}
		})
	}
}

func TestExtract(t *testing.T) {
	t.Cleanup(Cleanup)
	dir := t.TempDir()
	file := filepath.Join(dir, "sample.tar.gz")
	writeTarGz(t, file)
	root := Resolve(storage.NewFileURI(file))

	single, _ := storage.Child(root, "docs/read me.txt")
	path, err := Extract(single)
	if err != nil {
		t.Fatal(err)
	}
	if body, _ := os.ReadFile(path); string(body) != "hello" {
		t.Errorf("extracted %q", body)
	}
	if filepath.Base(path) != "read me.txt" {
		t.Errorf("extracted name %s", path)
	}

	// the whole folder is extracted even though one file is already present
	docs, _ := storage.Child(root, "docs")
	path, err = Extract(docs)
	if err != nil {
		t.Fatal(err)
	}
	if body, _ := os.ReadFile(filepath.Join(path, "notes")); len(body) == 0 {
		t.Error("folder extraction skipped notes")
	}

	all, err := Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(all, "empty")); err != nil || !info.IsDir() {
		t.Error("empty folder missing")
	}
	if info, err := os.Stat(filepath.Join(all, "bin", "tool")); err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Error("executable bit lost")
	}
	if _, err := os.Stat(filepath.Join(all, "escape.txt")); err != nil {
		t.Error("escaping entry should be contained in the archive folder")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(all), "escape.txt")); err == nil {
		t.Error("entry escaped the extraction folder")
	}
	if _, err := os.Stat(filepath.Join(dir, "escape.txt")); err == nil {
		t.Error("entry escaped the temporary folder")
	}

	plain := storage.NewFileURI(file)
	if p, _ := Extract(plain); p != file {
		t.Errorf("plain files should be used in place, got %s", p)
	}

	Cleanup()
	if _, err := os.Stat(all); err == nil {
		t.Error("temporary files not removed")
	}
}

func TestFolderNamedLikeArchive(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "folder.zip")
	os.Mkdir(fake, 0o755)
	u := storage.NewFileURI(fake)
	if IsFile(u) || Within(Resolve(u)) {
		t.Error("a folder is not an archive")
	}
}

func TestRepoArchive(t *testing.T) {
	if testing.Short() {
		t.Skip("reads a large archive")
	}
	file, _ := filepath.Abs(filepath.Join("..", "..", "Fyles.tar.xz"))
	if _, err := os.Stat(file); err != nil {
		t.Skip("sample tar.xz not present")
	}

	root := Resolve(storage.NewFileURI(file))
	list, err := storage.List(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s contains %v", file, names(list))
	if len(list) == 0 {
		t.Error("expected content")
	}
}
