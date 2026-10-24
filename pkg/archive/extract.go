package archive

import (
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
)

var (
	tempMu    sync.Mutex
	tempRoot  string
	extracted = map[string]bool{} // temporary paths whose content is complete
)

// Extract makes the file or folder that u addresses inside an archive
// available on disk, returning the path of a temporary copy. Each version of
// an archive is extracted at most once, so repeated calls are cheap.
// For a URI outside an archive the path is returned as it is.
func Extract(u fyne.URI) (string, error) {
	if !Within(u) {
		return u.Path(), nil
	}

	archive, inner, ok := splitArchivePath(u.Path())
	if !ok {
		return "", errNotInArchive
	}
	idx, err := indexFor(archive)
	if err != nil {
		return "", err
	}
	entry := idx.entries[inner]
	if entry == nil {
		return "", fs.ErrNotExist
	}

	tempMu.Lock()
	defer tempMu.Unlock()
	base, err := tempDirFor(idx)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(base, filepath.FromSlash(inner))
	if isExtracted(base, dest) {
		return dest, nil
	}

	if err := extractInto(idx, entry, base); err != nil {
		os.RemoveAll(dest)
		return "", err
	}
	extracted[dest] = true
	return dest, nil
}

// Cleanup removes everything that was extracted from archives.
// It should be called when the application exits.
func Cleanup() {
	tempMu.Lock()
	defer tempMu.Unlock()

	if tempRoot != "" {
		os.RemoveAll(tempRoot)
		tempRoot = ""
	}
	extracted = map[string]bool{}
}

// tempDirFor returns the folder that holds the extracted content of one
// version of an archive, named so that the archive can be recognised in the
// apps it is opened with.
func tempDirFor(idx *archiveIndex) (string, error) {
	if tempRoot == "" {
		dir, err := os.MkdirTemp("", "fyles-")
		if err != nil {
			return "", err
		}
		tempRoot = dir
	}

	h := fnv.New64a()
	fmt.Fprintf(h, "%s|%d|%d", idx.path, idx.modTime.UnixNano(), idx.size)
	name := filepath.Base(idx.path)
	name = strings.TrimSuffix(name, name[len(name)-len(Suffix(name)):])
	return filepath.Join(tempRoot, fmt.Sprintf("%s-%x", name, h.Sum64())), nil
}

// isExtracted reports whether dest, or a folder above it, was fully extracted.
func isExtracted(base, dest string) bool {
	for p := dest; strings.HasPrefix(p, base); p = filepath.Dir(p) {
		if extracted[p] {
			return true
		}
		if p == base {
			break
		}
	}
	return false
}

// extractInto writes the entry, or the whole tree below it, into base.
func extractInto(idx *archiveIndex, entry *archiveEntry, base string) error {
	if !entry.dir {
		return extractFile(idx, entry, base)
	}

	// folders first, so that empty ones are not missed
	for name, e := range idx.entries {
		if e.dir && within(name, entry.name) {
			if err := os.MkdirAll(filepath.Join(base, filepath.FromSlash(name)), dirPerm(e.mode)); err != nil {
				return err
			}
		}
	}

	ar, err := openArchive(idx.path)
	if err != nil {
		return err
	}
	defer ar.Close()

	for {
		h, err := ar.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		name, ok := cleanEntryName(h.name)
		if !ok || !h.regular || !within(name, entry.name) {
			continue
		}
		e := idx.entries[name]
		if e == nil || e.dir {
			continue
		}

		rc, err := ar.Open()
		if err != nil {
			return err
		}
		err = writeFile(filepath.Join(base, filepath.FromSlash(name)), rc, e.mode)
		rc.Close()
		if err != nil {
			return err
		}
	}
}

func extractFile(idx *archiveIndex, entry *archiveEntry, base string) error {
	rc, err := openEntry(idx.path, entry.name)
	if err != nil {
		return err
	}
	defer rc.Close()

	return writeFile(filepath.Join(base, filepath.FromSlash(entry.name)), rc, entry.mode)
}

func writeFile(dest string, r io.Reader, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}

	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, filePerm(mode))
	if err != nil {
		return err
	}
	if _, err = io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// within reports whether name is dir or lies below it ("" being the root).
func within(name, dir string) bool {
	return dir == "" || name == dir || strings.HasPrefix(name, dir+"/")
}

// filePerm keeps the executable bits of an entry while making sure the copy
// is private to, and removable by, the user.
func filePerm(mode fs.FileMode) fs.FileMode {
	return (mode.Perm() | 0o600) &^ 0o077
}

func dirPerm(mode fs.FileMode) fs.FileMode {
	return (mode.Perm() | 0o700) &^ 0o077
}
