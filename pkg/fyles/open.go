package fyles

import (
	"os"
	"os/exec"
	"strings"

	"fyne.io/fyne/v2"
)

func Open(u fyne.URI) error {
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
