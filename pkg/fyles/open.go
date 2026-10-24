package fyles

import (
	"os"
	"os/exec"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"

	"github.com/FyshOS/fyles/pkg/archive"
)

// Open launches the default application for the file at u.
// Content inside an archive is extracted to a temporary location first.
func Open(u fyne.URI) error {
	if archive.Within(u) {
		path, err := archive.Extract(u)
		if err != nil {
			return err
		}
		u = storage.NewFileURI(path)
	}

	file := u.Name()
	if u.Scheme() == "file" {
		file = u.Path() // better specificity if we can
	}

	cmd := exec.Command("xdg-mime", "query", "filetype", file)
	mime, err := cmd.Output()
	if err != nil {
		return err
	}

	cmd = exec.Command("xdg-mime", "query", "default", strings.TrimSpace(string(mime)))
	entry, err := cmd.Output()
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(entry)) == "" {
		return nil // should this show an error?
	}

	cmd = exec.Command("xdg-open", u.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

// OpenWithProgress is Open for use off the UI thread: while content is being
// extracted from an archive for longer than a moment, a progress dialog is
// shown over win.
func OpenWithProgress(u fyne.URI, win fyne.Window) error {
	if !archive.Within(u) {
		return Open(u)
	}

	return withActivity(win, u.Name(), "Extracting...", func() error {
		return Open(u)
	})
}

// PrepareArchive reads the archive that u points into, if any, so that it can
// be listed without delay afterward. It blocks, so call it off the UI
// thread; a progress dialog is shown over win while it takes a while.
func PrepareArchive(u fyne.URI, win fyne.Window) error {
	if archive.Prepared(u) {
		return nil
	}

	return withActivity(win, u.Name(), "Opening archive...", func() error {
		return archive.Prepare(u)
	})
}
