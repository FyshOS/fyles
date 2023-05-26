package fyles

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	xWidget "fyne.io/x/fyne/widget"
)

type Panel struct {
	widget.BaseWidget

	HideParent bool
	items      []*fileData

	content *xWidget.GridWrap
	cb      func(fyne.URI)
	win     fyne.Window
	current *fileItem
}

func NewFylesPanel(c func(fyne.URI), w fyne.Window) *Panel {
	fileItemMin = fyne.NewSize(fileIconCellWidth, fileIconSize+fileTextSize+theme.InnerPadding())

	p := &Panel{cb: c, win: w}
	p.ExtendBaseWidget(p)

	p.content = xWidget.NewGridWrap(
		func() int {
			return len(p.items)
		},
		func() fyne.CanvasObject {
			icon := &fileItem{parent: p}
			icon.ExtendBaseWidget(icon)
			return icon
		},
		func(id xWidget.GridWrapItemID, obj fyne.CanvasObject) {
			icon := obj.(*fileItem)
			icon.setData(p.items[id])
		})

	return p
}

func (p *Panel) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(p.content)
}

func (p *Panel) SetDir(u fyne.URI) {
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
		for _, item := range list {
			//if !ui.filter.Matches(item) {
			//	continue
			//}
			if item.Name()[0] == '.' {
				continue
			}

			dir, _ := storage.CanList(item)
			items = append(items, &fileData{name: fileName(item), location: item, dir: dir})
		}
	}

	p.items = items
	p.content.Refresh()
}
