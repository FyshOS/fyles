package main

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	fileIconSize      = 72
	fileTextSize      = 20
	fileIconCellWidth = fileIconSize * 1.25
)

var fileItemMin fyne.Size

type fileItem struct {
	widget.BaseWidget
	parent    *fyles
	isCurrent bool

	name     string
	location fyne.URI
	dir      bool
}

func (i *fileItem) Tapped(_ *fyne.PointEvent) {
	i.parent.itemTapped(i)
}

func (i *fileItem) CreateRenderer() fyne.WidgetRenderer {
	background := canvas.NewRectangle(theme.SelectionColor())
	background.Hide()
	text := widget.NewLabelWithStyle(i.name, fyne.TextAlignCenter, fyne.TextStyle{})
	text.Wrapping = fyne.TextTruncate
	icon := widget.NewFileIcon(i.location)

	return &fileItemRenderer{
		item:       i,
		background: background,
		icon:       icon,
		text:       text,
		objects:    []fyne.CanvasObject{background, icon, text},
	}
}

func fileName(path fyne.URI) string {
	name := path.Name()
	ext := filepath.Ext(name[1:])
	return name[:len(name)-len(ext)]
}

func newFileItem(location fyne.URI, dir bool, ui *fyles) *fileItem {
	item := &fileItem{
		parent:   ui,
		location: location,
		dir:      dir,
	}

	if dir {
		item.name = location.Name()
	} else {
		item.name = fileName(location)
	}

	item.ExtendBaseWidget(item)
	return item
}

type fileItemRenderer struct {
	item *fileItem

	background *canvas.Rectangle
	icon       *widget.FileIcon
	text       *widget.Label
	objects    []fyne.CanvasObject
}

func (s fileItemRenderer) Layout(size fyne.Size) {
	s.background.Resize(size)

	iconAlign := (size.Width - fileIconSize) / 2
	s.icon.Resize(fyne.NewSize(fileIconSize, fileIconSize))
	s.icon.Move(fyne.NewPos(iconAlign, 0))

	textHeight := s.text.MinSize().Height
	s.text.Resize(fyne.NewSize(size.Width, textHeight))
	s.text.Move(fyne.NewPos(0, size.Height-textHeight))
}

func (s fileItemRenderer) MinSize() fyne.Size {
	return fileItemMin
}

func (s fileItemRenderer) Refresh() {
	if s.item.isCurrent {
		s.background.FillColor = theme.SelectionColor()
		s.background.Show()
	} else {
		s.background.Hide()
	}
	s.background.Refresh()
	canvas.Refresh(s.item)
}

func (s fileItemRenderer) Objects() []fyne.CanvasObject {
	return s.objects
}

func (s fileItemRenderer) Destroy() {
}
