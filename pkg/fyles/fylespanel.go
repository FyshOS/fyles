package fyles

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

type Panel struct {
	widget.BaseWidget

	HideParent bool
	Filter     storage.FileFilter
	items      []*fileData

	content  *widget.GridWrap
	cb       func(fyne.URI)
	selected widget.GridWrapItemID
	win      fyne.Window
}

func NewFylesPanel(c func(fyne.URI), w fyne.Window) *Panel {
	p := &Panel{cb: c, win: w}
	p.ExtendBaseWidget(p)

	p.content = widget.NewGridWrap(
		func() int {
			return len(p.items)
		},
		func() fyne.CanvasObject {
			icon := &fileItem{parent: p}
			icon.ExtendBaseWidget(icon)
			return icon
		},
		func(id widget.GridWrapItemID, obj fyne.CanvasObject) {
			icon := obj.(*fileItem)
			icon.setData(p.items[id])
		})
	p.content.OnSelected = func(id widget.GridWrapItemID) {
		p.selected = id
		p.cb(p.items[id].location)
	}

	return p
}

func (p *Panel) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(p.content)
}

// SetDir asks the fyles panel to display the specified directory
func (p *Panel) SetDir(u fyne.URI) {
	p.content.Unselect(p.selected)

	var items []*fileData
	if !p.HideParent {
		parent, err := storage.Parent(u)
		if err == nil {
			items = append(items, &fileData{name: "(Parent)", location: parent, dir: true})
		}
	}
	list, err := storage.List(u)
	if err != nil {
		fyne.LogError("Could not read dir", err)
	} else {
		p.addListing(list, items)
	}
}

// SetListing asks the fyles panel to display a list of URIs.
// This supports manually creating a collection of items not in a standard directory.
func (p *Panel) SetListing(u []fyne.URI) {
	p.content.Unselect(p.selected)

	var items []*fileData
	p.addListing(u, items)
}

func (p *Panel) addListing(list []fyne.URI, items []*fileData) {
	for _, item := range list {
		if !p.Filter.Matches(item) {
			continue
		}
		if item.Name()[0] == '.' {
			continue
		}

		dir, _ := storage.CanList(item)
		items = append(items, &fileData{name: fileName(item), location: item, dir: dir})
	}

	p.items = items
	p.content.Refresh()
}
