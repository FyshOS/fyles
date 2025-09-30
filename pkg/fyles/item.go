package fyles

import (
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2/theme"
	"github.com/FyshOS/appie"
	"github.com/FyshOS/fancyfs"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
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

	return fyne.NewMenu(u.Name(),
		openItem, openWithItem,
		fyne.NewMenuItem("Copy path", func() {
			i.parent.win.Clipboard().SetContent(u.Path())
		}),
	)
}

func (i *fileItem) setData(d *fileData) {
	i.data = d

	ext := filepath.Ext(i.data.name[1:])
	i.data.name = i.data.name[:len(i.data.name)-len(ext)]

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
	s.icon.SetURI(s.item.data.location)

	ff, err := fancyfs.DetailsForFolder(s.item.data.location)
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

	file := i.data.location.Name()
	if i.data.location.Scheme() == "file" {
		file = i.data.location.Path() // better specificity if we can
	}

	err := app.RunWithParameters([]string{file}, nil)
	if err != nil {
		fyne.LogError("Failed to launch app", err)
	}
}
