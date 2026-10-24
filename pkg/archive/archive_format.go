package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// entryHeader is the format independent description of an archive entry.
type entryHeader struct {
	name    string
	dir     bool
	regular bool
	size    int64
	mode    fs.FileMode
}

// archiveReader iterates over the entries of an archive in storage order.
type archiveReader interface {
	// Next advances to the next entry, returning io.EOF after the last one.
	Next() (entryHeader, error)
	// Open returns the content of the current entry, which must be read
	// before Next is called again.
	Open() (io.ReadCloser, error)
	io.Closer
}

// openArchive opens the archive at path with the reader for its format.
func openArchive(path string) (archiveReader, error) {
	if strings.HasSuffix(strings.ToLower(path), ".zip") {
		return openZip(path)
	}
	return openTar(path)
}

// openEntry returns the content of the named file inside the archive at path.
func openEntry(path, name string) (io.ReadCloser, error) {
	ar, err := openArchive(path)
	if err != nil {
		return nil, err
	}

	for {
		h, err := ar.Next()
		if err == io.EOF {
			err = fs.ErrNotExist
		}
		if err != nil {
			ar.Close()
			return nil, err
		}

		clean, ok := cleanEntryName(h.name)
		if !ok || clean != name || !h.regular {
			continue
		}

		rc, err := ar.Open()
		if err != nil {
			ar.Close()
			return nil, err
		}
		return &chainedCloser{Reader: rc, closers: []io.Closer{rc, ar}}, nil
	}
}

// chainedCloser reads from Reader and closes each closer, in order, when done.
type chainedCloser struct {
	io.Reader
	closers []io.Closer
}

func (c *chainedCloser) Close() error {
	var err error
	for _, closer := range c.closers {
		if e := closer.Close(); e != nil && err == nil {
			err = e
		}
	}
	return err
}

type zipReader struct {
	rc   *zip.ReadCloser
	next int
}

func openZip(path string) (archiveReader, error) {
	rc, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	return &zipReader{rc: rc}, nil
}

func (z *zipReader) Next() (entryHeader, error) {
	if z.next >= len(z.rc.File) {
		return entryHeader{}, io.EOF
	}

	f := z.rc.File[z.next]
	z.next++
	info := f.FileInfo()
	return entryHeader{
		name:    f.Name,
		dir:     info.IsDir(),
		regular: info.Mode().IsRegular(),
		size:    info.Size(),
		mode:    info.Mode(),
	}, nil
}

func (z *zipReader) Open() (io.ReadCloser, error) {
	if z.next == 0 {
		return nil, errors.New("no current entry")
	}
	return z.rc.File[z.next-1].Open()
}

func (z *zipReader) Close() error {
	return z.rc.Close()
}

type tarReader struct {
	tr      *tar.Reader
	closers []io.Closer
}

func openTar(path string) (archiveReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	r, closers, err := decompress(path, f)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &tarReader{tr: tar.NewReader(r), closers: append(closers, f)}, nil
}

// decompress wraps the raw stream of a tar archive with the decoder its name
// calls for, returning the closers needed to release the decoder.
func decompress(path string, f io.Reader) (io.Reader, []io.Closer, error) {
	switch Suffix(path) {
	case ".tar.gz", ".tgz":
		r, err := gzip.NewReader(f)
		if err != nil {
			return nil, nil, err
		}
		return r, []io.Closer{r}, nil
	case ".tar.bz2", ".tbz2":
		return bzip2.NewReader(f), nil, nil
	case ".tar.xz", ".txz":
		r, err := xz.NewReader(f)
		if err != nil {
			return nil, nil, err
		}
		return r, nil, nil
	case ".tar.zst", ".tzst":
		r, err := zstd.NewReader(f)
		if err != nil {
			return nil, nil, err
		}
		return r, []io.Closer{r.IOReadCloser()}, nil
	default:
		return f, nil, nil
	}
}

func (t *tarReader) Next() (entryHeader, error) {
	for {
		h, err := t.tr.Next()
		if err != nil {
			return entryHeader{}, err
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue // metadata such as the pax_global_header written by git
		}

		info := h.FileInfo()
		return entryHeader{
			name:    h.Name,
			dir:     info.IsDir(),
			regular: info.Mode().IsRegular(),
			size:    h.Size,
			mode:    info.Mode(),
		}, nil
	}
}

func (t *tarReader) Open() (io.ReadCloser, error) {
	return io.NopCloser(t.tr), nil
}

func (t *tarReader) Close() error {
	var err error
	for _, c := range t.closers {
		if e := c.Close(); e != nil && err == nil {
			err = e
		}
	}
	return err
}

// isText reports whether a sample from the start of a file reads as text.
func isText(b []byte) bool {
	if bytes.IndexByte(b, 0) >= 0 {
		return false
	}

	// the sample may end part way through a multi-byte character
	for cut := 0; cut < utf8.UTFMax && cut <= len(b); cut++ {
		if utf8.Valid(b[:len(b)-cut]) {
			return true
		}
	}
	return false
}
