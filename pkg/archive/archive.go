// Package archive lets the contents of archive files, such as zip and tar
// files, be browsed through Fyne's storage API as if they were folders.
package archive

//go:generate fyne bundle -o bundled.go --pkg archive archive.svg

import (
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/storage/repository"
	"fyne.io/fyne/v2/theme"
)

// archiveScheme is the URI scheme under which the contents of archive files are
// browsed. The path of such a URI is the path of the archive file followed by
// the path of an entry inside it, so "archive:///tmp/a.zip/dir/file.txt" is
// the file "dir/file.txt" inside "/tmp/a.zip".
const archiveScheme = "archive"

// archiveSuffixes lists the file name endings that are browsed as archives.
var archiveSuffixes = []string{
	".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst",
	".tgz", ".tbz2", ".txz", ".tzst", ".tar", ".zip",
}

func init() {
	repository.Register(archiveScheme, &archiveRepo{})
}

// Icon is the icon shown for archive files, which open like folders.
func Icon() fyne.Resource {
	return theme.NewThemedResource(resourceArchiveSvg)
}

// Suffix returns the archive extension of a file name, such as ".tar.gz",
// or "" when the name is not of a supported archive type.
func Suffix(name string) string {
	lower := strings.ToLower(name)
	for _, s := range archiveSuffixes {
		if strings.HasSuffix(lower, s) && len(lower) > len(s) {
			return s
		}
	}
	return ""
}

// IsFile reports whether u is a file on disk of a supported archive type.
func IsFile(u fyne.URI) bool {
	if u == nil || u.Scheme() != "file" || Suffix(u.Name()) == "" {
		return false
	}

	info, err := os.Stat(u.Path())
	return err == nil && info.Mode().IsRegular()
}

// Within reports whether u addresses an archive or something inside one,
// that is whether it uses the archive scheme.
func Within(u fyne.URI) bool {
	return u != nil && u.Scheme() == archiveScheme
}

// IsEntry reports whether u addresses something inside an archive, rather
// than an archive file itself or a location outside one.
func IsEntry(u fyne.URI) bool {
	if !Within(u) {
		return false
	}
	_, inner, ok := splitArchivePath(u.Path())
	return ok && inner != ""
}

// Resolve maps a file URI that is, or lies inside, an archive file onto
// the archive scheme so that it can be listed like a folder.
// Any other URI is returned unchanged.
func Resolve(u fyne.URI) fyne.URI {
	if u == nil || u.Scheme() != "file" {
		return u
	}

	archive, inner, ok := splitArchivePath(u.Path())
	if !ok {
		return u
	}
	return newArchiveURI(archive, inner)
}

// Prepare reads the table of contents of the archive that u points into, so
// that listing it afterward is immediate. Tar based archives have to be
// decompressed in full for this, which can take a while, so call it off the
// UI thread. Locations outside an archive need no preparation.
func Prepare(u fyne.URI) error {
	if !Within(u) {
		return nil
	}

	archive, _, ok := splitArchivePath(u.Path())
	if !ok {
		return errNotInArchive
	}
	_, err := indexFor(archive)
	return err
}

// Prepared reports whether listing u will be immediate: it is outside an
// archive, or the archive it points into has been read and is unchanged.
func Prepared(u fyne.URI) bool {
	if !Within(u) {
		return true
	}

	archive, _, ok := splitArchivePath(u.Path())
	if !ok {
		return true // nothing to read
	}
	info, err := os.Stat(archive)
	if err != nil {
		return true
	}

	indexMu.Lock()
	defer indexMu.Unlock()
	return cachedIndex(archive, info) != nil
}

// splitArchivePath finds the archive file that p is or lies inside, returning
// its path and the slash separated path of the entry within it ("" for the
// archive itself).
func splitArchivePath(p string) (archive, inner string, ok bool) {
	p = filepath.Clean(p)
	for dir := p; ; dir = filepath.Dir(dir) {
		if Suffix(filepath.Base(dir)) != "" {
			if info, err := os.Stat(dir); err == nil && info.Mode().IsRegular() {
				inner = strings.TrimPrefix(strings.TrimPrefix(p, dir), string(filepath.Separator))
				return dir, filepath.ToSlash(inner), true
			}
		}

		if filepath.Dir(dir) == dir {
			return "", "", false
		}
	}
}

// archiveRootURI is the URI of an archive file when it is presented as a
// folder, carrying the icon that marks it as an archive.
type archiveRootURI struct {
	fyne.URI
}

func (a *archiveRootURI) Icon() fyne.Resource {
	return Icon()
}

// entryURI is the URI of an item inside an archive. It answers the mime type
// from what was learned while indexing, so file icons never need the content
// to be decompressed again.
type entryURI struct {
	fyne.URI
	entry *archiveEntry
}

func (e *entryURI) MimeType() string {
	if e.entry.dir {
		return "inode/directory"
	}
	if m := mime.TypeByExtension(e.Extension()); m != "" {
		m, _, _ = strings.Cut(m, ";")
		return m
	}
	if e.entry.text {
		return "text/plain"
	}
	return "application/octet-stream"
}

func parseArchiveURI(archive, inner string) (fyne.URI, error) {
	p := filepath.ToSlash(archive)
	if inner != "" {
		p += "/" + inner
	}
	return storage.ParseURI((&url.URL{Scheme: archiveScheme, Path: p}).String())
}

func newArchiveURI(archive, inner string) fyne.URI {
	u, err := parseArchiveURI(archive, inner)
	if err != nil {
		fyne.LogError("Could not build archive URI for "+archive, err)
		return storage.NewFileURI(archive)
	}
	if inner == "" {
		return &archiveRootURI{u}
	}
	return u
}

// archiveEntry describes one item inside an archive.
type archiveEntry struct {
	name string // slash separated path inside the archive
	dir  bool
	size int64
	mode fs.FileMode
	text bool // content looked like text, used when the name has no known extension
}

// archiveIndex is the table of contents of one archive file.
type archiveIndex struct {
	path    string
	modTime time.Time
	size    int64

	entries  map[string]*archiveEntry   // keyed by entry name, "" is the root
	children map[string][]*archiveEntry // keyed by parent name, sorted by name
}

var (
	indexMu sync.Mutex
	indexes = map[string]*archiveIndex{}
)

// indexFor returns the table of contents of the archive at path, reading it
// on first use or when the file has changed since it was last read.
func indexFor(path string) (*archiveIndex, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	indexMu.Lock()
	defer indexMu.Unlock()
	if idx := cachedIndex(path, info); idx != nil {
		return idx, nil
	}

	idx, err := buildIndex(path, info)
	if err != nil {
		return nil, err
	}
	indexes[path] = idx
	return idx, nil
}

// cachedIndex returns the index read for the archive at path if it still
// matches the file on disk. indexMu must be held.
func cachedIndex(path string, info os.FileInfo) *archiveIndex {
	idx := indexes[path]
	if idx != nil && idx.modTime.Equal(info.ModTime()) && idx.size == info.Size() {
		return idx
	}
	return nil
}

func buildIndex(file string, info os.FileInfo) (*archiveIndex, error) {
	idx := &archiveIndex{
		path:     file,
		modTime:  info.ModTime(),
		size:     info.Size(),
		entries:  map[string]*archiveEntry{"": {dir: true}},
		children: map[string][]*archiveEntry{},
	}

	ar, err := openArchive(file)
	if err != nil {
		return nil, err
	}
	defer ar.Close()

	for {
		h, err := ar.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		name, ok := cleanEntryName(h.name)
		if !ok || name == "" {
			continue
		}
		if h.dir {
			idx.addDir(name)
			continue
		}
		if !h.regular {
			continue // links and devices are not browsable
		}

		e := &archiveEntry{name: name, size: h.size, mode: h.mode}
		if mime.TypeByExtension(path.Ext(name)) == "" {
			e.text = looksTextual(ar)
		}
		idx.add(e)
	}

	for _, list := range idx.children {
		sort.Slice(list, func(i, j int) bool { return list[i].name < list[j].name })
	}
	return idx, nil
}

// cleanEntryName normalises the name of an archive entry to a relative, slash
// separated path, reporting false for names that would escape the archive.
func cleanEntryName(name string) (string, bool) {
	name = strings.ReplaceAll(name, `\`, "/")
	name = path.Clean("/" + name) // resolves "." and ".." against a root
	if strings.ContainsRune(name, 0) {
		return "", false
	}
	return strings.TrimPrefix(name, "/"), true
}

func (idx *archiveIndex) addDir(name string) {
	if _, ok := idx.entries[name]; ok {
		return
	}
	idx.add(&archiveEntry{name: name, dir: true, mode: fs.ModeDir | 0o755})
}

func (idx *archiveIndex) add(e *archiveEntry) {
	parent := path.Dir(e.name)
	if parent == "." {
		parent = ""
	}
	idx.addDir(parent)

	if old, ok := idx.entries[e.name]; ok {
		// later entries replace earlier ones, as when extracting
		list := idx.children[parent]
		for i, c := range list {
			if c == old {
				list[i] = e
			}
		}
	} else {
		idx.children[parent] = append(idx.children[parent], e)
	}
	idx.entries[e.name] = e
}

// looksTextual samples the start of the current archive entry.
func looksTextual(ar archiveReader) bool {
	rc, err := ar.Open()
	if err != nil {
		return false
	}
	defer rc.Close()

	buf := make([]byte, 512)
	n, _ := io.ReadFull(rc, buf)
	return isText(buf[:n])
}

// archiveRepo serves the archive scheme by reading the files it points into.
type archiveRepo struct{}

var errNotInArchive = errors.New("not an archive location")

func (r *archiveRepo) lookup(u fyne.URI) (idx *archiveIndex, entry *archiveEntry, err error) {
	archive, inner, ok := splitArchivePath(u.Path())
	if !ok {
		return nil, nil, errNotInArchive
	}

	idx, err = indexFor(archive)
	if err != nil {
		return nil, nil, err
	}
	entry = idx.entries[inner]
	if entry == nil {
		return idx, nil, fs.ErrNotExist
	}
	return idx, entry, nil
}

func (r *archiveRepo) Exists(u fyne.URI) (bool, error) {
	archive, inner, ok := splitArchivePath(u.Path())
	if !ok {
		return false, nil
	}
	if inner == "" {
		return true, nil
	}

	idx, err := indexFor(archive)
	if err != nil {
		return false, err
	}
	_, found := idx.entries[inner]
	return found, nil
}

func (r *archiveRepo) Reader(u fyne.URI) (fyne.URIReadCloser, error) {
	idx, entry, err := r.lookup(u)
	if err != nil {
		return nil, err
	}
	if entry.dir {
		return nil, errors.New(u.Name() + " is a folder")
	}

	rc, err := openEntry(idx.path, entry.name)
	if err != nil {
		return nil, err
	}
	return &entryReadCloser{ReadCloser: rc, uri: u}, nil
}

func (r *archiveRepo) CanRead(u fyne.URI) (bool, error) {
	_, entry, err := r.lookup(u)
	if errors.Is(err, fs.ErrNotExist) || err == errNotInArchive {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !entry.dir, nil
}

func (r *archiveRepo) Destroy(string) {
}

func (r *archiveRepo) CanList(u fyne.URI) (bool, error) {
	archive, inner, ok := splitArchivePath(u.Path())
	if !ok {
		return false, nil
	}
	if inner == "" {
		return true, nil // the archive file itself, indexed only when listed
	}

	idx, err := indexFor(archive)
	if err != nil {
		return false, err
	}
	entry := idx.entries[inner]
	return entry != nil && entry.dir, nil
}

func (r *archiveRepo) List(u fyne.URI) ([]fyne.URI, error) {
	idx, entry, err := r.lookup(u)
	if err != nil {
		return nil, err
	}
	if !entry.dir {
		return nil, errors.New(u.Name() + " is not a folder")
	}

	children := idx.children[entry.name]
	list := make([]fyne.URI, 0, len(children))
	for _, child := range children {
		cu, err := parseArchiveURI(idx.path, child.name)
		if err != nil {
			return nil, err
		}
		list = append(list, &entryURI{URI: cu, entry: child})
	}
	return list, nil
}

func (r *archiveRepo) CreateListable(fyne.URI) error {
	return repository.ErrOperationNotSupported
}

func (r *archiveRepo) Parent(u fyne.URI) (fyne.URI, error) {
	archive, inner, ok := splitArchivePath(u.Path())
	if !ok {
		return nil, errNotInArchive
	}
	if inner == "" {
		return storage.NewFileURI(filepath.Dir(archive)), nil // leave the archive
	}
	return repository.GenericParent(u)
}

func (r *archiveRepo) Child(u fyne.URI, component string) (fyne.URI, error) {
	return repository.GenericChild(u, component)
}

type entryReadCloser struct {
	io.ReadCloser
	uri fyne.URI
}

func (e *entryReadCloser) URI() fyne.URI {
	return e.uri
}
