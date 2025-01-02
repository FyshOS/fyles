package fyles

import (
	"path/filepath"

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

	return &fileItemRenderer{
		item:         i,
		icon:         icon,
		text:         text,
		objects:      []fyne.CanvasObject{icon, text},
		fileTextSize: widget.NewLabel("M\nM").MinSize().Height, // cache two-line label height,
	}
}

func (i *fileItem) buildMenu(u fyne.URI) *fyne.Menu {
	openItem := fyne.NewMenuItem("Open", i.tapMe)
	return fyne.NewMenu(u.Name(),
		openItem,
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
	objects []fyne.CanvasObject
}

func (s *fileItemRenderer) Layout(size fyne.Size) {
	s.icon.Resize(fyne.NewSize(fileIconSize, fileIconSize))
	s.icon.Move(fyne.NewPos((size.Width-fileIconSize)/2, 0))

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
	canvas.Refresh(s.item)
}

func (s *fileItemRenderer) Objects() []fyne.CanvasObject {
	return s.objects
}

func (s *fileItemRenderer) Destroy() {
}
