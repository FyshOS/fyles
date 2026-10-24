package fyles

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2/theme"
	"github.com/FyshOS/appie"
	"github.com/FyshOS/fancyfs"
	"github.com/FyshOS/fyles/pkg/archive"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

const (
	fileIconSize      = 64
	fileIconCellWidth = fileIconSize * 1.25
)

type fileData struct {
	name     string
	location fyne.URI
	dir      bool
	parent   bool // the "(Parent)" entry, which cannot be edited from here
}

type fileItem struct {
	widget.BaseWidget
	parent *Panel

	data *fileData
}

func (i *fileItem) Tapped(*fyne.PointEvent) {
	i.tapMe()
}

func (i *fileItem) TappedSecondary(ev *fyne.PointEvent) {
	m := i.buildMenu(i.data.location)
	widget.ShowPopUpMenuAtPosition(m, i.parent.win.Canvas(), ev.AbsolutePosition)
}

func (i *fileItem) CreateRenderer() fyne.WidgetRenderer {
	text := widget.NewLabelWithStyle("FileName", fyne.TextAlignCenter, fyne.TextStyle{})
	text.Truncation = fyne.TextTruncateEllipsis
	text.Wrapping = fyne.TextWrapBreak
	icon := widget.NewFileIcon(nil)
	over := &canvas.Image{}

	return &fileItemRenderer{
		item:         i,
		icon:         icon,
		text:         text,
		over:         over,
		objects:      []fyne.CanvasObject{icon, text, over},
		fileTextSize: widget.NewLabel("M\nM").MinSize().Height, // cache two-line label height,
	}
}

func mimeForURI(u fyne.URI) (string, error) {
	if archive.IsEntry(u) {
		return u.MimeType(), nil // known from the archive index, nothing on disk to inspect
	}

	file := u.Name()
	if u.Scheme() == "file" {
		file = u.Path() // better specificity if we can
	}

	cmd := exec.Command("xdg-mime", "query", "filetype", file)
	mime, err := cmd.Output()
	return strings.TrimSpace(string(mime)), err
}

func (i *fileItem) buildMenu(u fyne.URI) *fyne.Menu {
	openItem := fyne.NewMenuItem("Open", i.tapMe)
	openWithItem := fyne.NewMenuItem("Open With...", nil)

	appItems := []*fyne.MenuItem{}
	mime, err := mimeForURI(u)
	if err != nil {
		fyne.LogError("failed to lookup file mime", err)
	} else if mime != "" {
		apps := i.appsForMime(mime)
		appItems = make([]*fyne.MenuItem, len(apps))

		for id, a := range apps {
			match := a
			item := fyne.NewMenuItem(a.Name(), func() {
				i.openWith(match)
			})
			item.Icon = a.Icon("", 64)

			appItems[id] = item
		}
	}

	openWithItem.ChildMenu = fyne.NewMenu("Open With", appItems...)
	if len(appItems) == 0 {
		openWithItem.Disabled = true
	}

	items := []*fyne.MenuItem{
		openItem, openWithItem,
		fyne.NewMenuItem("Copy path", func() {
			i.parent.win.Clipboard().SetContent(u.Path())
		}),
	}
	if !i.data.parent && !i.data.inArchive() {
		items = append(items, fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Rename...", func() {
				i.showRename(u)
			}),
			fyne.NewMenuItem("Delete...", func() {
				i.showDelete(u, i.data.dir && !archive.IsFile(u))
			}))
	}

	return fyne.NewMenu(u.Name(), items...)
}

func (i *fileItem) showRename(u fyne.URI) {
	entry := widget.NewEntry()
	entry.SetText(u.Name())
	entry.Validator = func(name string) error {
		if name == "" {
			return errors.New("name cannot be empty")
		}
		if strings.ContainsAny(name, `/\`) {
			return errors.New("name cannot contain a path separator")
		}
		return nil
	}

	win := i.parent.win
	form := dialog.NewForm("Rename", "Rename", "Cancel", []*widget.FormItem{
		{Text: "Name", Widget: entry},
	}, func(ok bool) {
		name := entry.Text
		if !ok || name == u.Name() {
			return
		}

		dir, err := storage.Parent(u)
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		dest, err := storage.Child(dir, name)
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		if exists, _ := storage.Exists(dest); exists {
			dialog.ShowError(errors.New("an item named "+name+" already exists"), win)
			return
		}

		if err := storage.Move(u, dest); err != nil {
			fyne.LogError("Failed to rename "+u.Path(), err)
			dialog.ShowError(errors.New("could not rename "+u.Name()), win)
			return
		}
		i.parent.reload(u, dest)
	}, win)
	entry.OnSubmitted = func(string) {
		form.Submit()
	}
	form.Show()
	win.Canvas().Focus(entry)
}

func (i *fileItem) showDelete(u fyne.URI, dir bool) {
	msg := "Are you sure you want to delete \"" + u.Name() + "\"?"
	if dir {
		msg = "Are you sure you want to delete the folder \"" + u.Name() + "\" and all of its contents?"
	}

	win := i.parent.win
	dialog.ShowConfirm("Delete", msg, func(ok bool) {
		if !ok {
			return
		}

		var err error
		if dir {
			err = storage.DeleteAll(u)
		} else {
			err = storage.Delete(u)
		}
		if err != nil {
			fyne.LogError("Failed to delete "+u.Path(), err)
			dialog.ShowError(errors.New("could not delete "+u.Name()), win)
			return
		}
		i.parent.reload(u, nil)
	}, win)
}

func (i *fileItem) setData(d *fileData) {
	i.data = d // the name was already trimmed of its extension by fileName
	i.Refresh()
}

func (i *fileItem) tapMe() {
	for id, item := range i.parent.items {
		if item.location == i.data.location {
			i.parent.content.Select(id)

			return
		}
	}
}

func fileName(path fyne.URI) string {
	name := path.Name()
	if suffix := archive.Suffix(name); suffix != "" {
		return name[:len(name)-len(suffix)]
	}
	ext := filepath.Ext(name[1:])
	return name[:len(name)-len(ext)]
}

type fileItemRenderer struct {
	item         *fileItem
	fileTextSize float32

	icon    *widget.FileIcon
	text    *widget.Label
	over    *canvas.Image
	objects []fyne.CanvasObject
}

func (s *fileItemRenderer) Layout(size fyne.Size) {
	s.icon.Resize(fyne.NewSize(fileIconSize, fileIconSize))
	s.icon.Move(fyne.NewPos((size.Width-fileIconSize)/2, 0))

	folderInsetX := float32(10)
	folderInsetBottom := float32(25)
	folderInsetTop := float32(20)
	s.over.Resize(fyne.NewSize(fileIconSize-folderInsetX*2, fileIconSize-folderInsetX-folderInsetBottom))
	s.over.Move(s.icon.Position().AddXY(folderInsetX, folderInsetTop))

	s.text.Alignment = fyne.TextAlignCenter
	s.text.Resize(fyne.NewSize(size.Width, s.fileTextSize))
	s.text.Move(fyne.NewPos(0, size.Height-s.fileTextSize))
}

func (s fileItemRenderer) MinSize() fyne.Size {
	return fyne.NewSize(fileIconCellWidth, fileIconSize+s.fileTextSize)
}

func (s *fileItemRenderer) Refresh() {
	s.fileTextSize = widget.NewLabel("M\nM").MinSize().Height // cache two-line label height

	s.text.SetText(s.item.data.name)
	icon := s.item.data.location
	if archive.IsFile(icon) {
		icon = archive.Resolve(icon)
	}
	s.icon.SetURI(icon)

	var ff *fancyfs.FancyFolder
	var err error
	if !archive.Within(s.item.data.location) { // probing an archive for metadata would read it all
		ff, err = fancyfs.DetailsForFolder(s.item.data.location)
	}
	if ff != nil && err == nil {
		if ff.BackgroundURI != nil {
			s.over.File = ff.BackgroundURI.Path()
		} else {
			s.over.File = ""
		}
		if ff.BackgroundResource != nil {
			s.over.Resource = theme.NewColoredResource(ff.BackgroundResource, theme.ColorNameBackground)
		} else {
			s.over.Resource = nil
		}
		s.over.FillMode = ff.BackgroundFill
	} else {
		s.over.File = ""
		s.over.Resource = nil
		s.over.Image = nil
	}

	s.over.Refresh()
	canvas.Refresh(s.item)
}

func (s *fileItemRenderer) Objects() []fyne.CanvasObject {
	return s.objects
}

func (s *fileItemRenderer) Destroy() {
}

func (i *fileItem) appsForMime(mime string) []appie.AppData {
	ret := []appie.AppData{}
	if i.parent.apps == nil {
		return ret
	}

	apps := i.parent.apps.AvailableApps()
	for _, a := range apps {
		for _, m := range a.MimeTypes() {
			if m == mime {
				ret = append(ret, a)
				break
			}
		}
	}

	return ret
}

func (i *fileItem) openWith(app appie.AppData) {
	i.parent.content.UnselectAll()

	u := i.data.location
	win := i.parent.win
	go func() { // extracting from an archive can take a moment
		file := u.Name()
		if archive.Within(u) {
			var path string
			err := withActivity(win, u.Name(), "Extracting...", func() (err error) {
				path, err = archive.Extract(u)
				return err
			})
			if err != nil {
				fyne.LogError("Failed to extract "+u.Path(), err)
				fyne.Do(func() { dialog.ShowError(err, win) })
				return
			}
			file = path
		} else if u.Scheme() == "file" {
			file = u.Path() // better specificity if we can
		}

		err := app.RunWithParameters([]string{file}, nil)
		if err != nil {
			fyne.LogError("Failed to launch app", err)
		}
	}()
}

// inArchive reports whether this item is inside an archive, where content
// cannot be changed.
func (d *fileData) inArchive() bool {
	return archive.IsEntry(d.location)
}
