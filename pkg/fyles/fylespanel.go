package fyles

import (
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"github.com/FyshOS/appie"
	"github.com/FyshOS/fancyfs"
	"github.com/FyshOS/fyles/pkg/archive"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

type Panel struct {
	widget.BaseWidget

	HideParent bool
	Filter     storage.FileFilter
	items      []*fileData
	dir        fyne.URI // the directory shown by SetDir, nil for a custom listing

	content  *widget.GridWrap
	bg       *canvas.Image
	cb       func(fyne.URI)
	selected widget.GridWrapItemID
	win      fyne.Window
	apps     appie.Provider
}

func NewFylesPanel(c func(fyne.URI), w fyne.Window) *Panel {
	p := &Panel{cb: c, win: w, apps: appie.SystemProvider()}
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
		},
	)
	p.content.OnSelected = func(id widget.GridWrapItemID) {
		p.selected = id
		p.cb(p.items[id].location)
	}

	p.bg = &canvas.Image{}
	p.bg.Translucency = 0.7
	p.bg.FillMode = canvas.ImageFillCover

	return p
}

func (p *Panel) ClearSelection() {
	p.content.Unselect(p.selected)
}

func (p *Panel) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(p.bg, p.content))
}

// SetDir asks the fyles panel to display the specified directory,
// which may also be an archive file or a folder inside one.
func (p *Panel) SetDir(u fyne.URI) {
	u = archive.Resolve(u)
	p.dir = u
	p.content.Unselect(p.selected)

	var items []*fileData
	if !p.HideParent {
		parent, err := storage.Parent(u)
		trim := parent.String()
		if trim[len(trim)-1] == '/' {
			trim = trim[:len(trim)-1]
		}
		parent, _ = storage.ParseURI(trim)
		if err == nil {
			items = append(items, &fileData{name: "(Parent)", location: parent, dir: true, parent: true})
		}
	}
	list, err := storage.List(u)
	if err != nil {
		fyne.LogError("Could not read dir", err)
	} else {
		p.addListing(list, items)
	}

	var ff *fancyfs.FancyFolder
	if !archive.Within(u) { // archives carry no folder metadata, and probing would read them
		ff, err = fancyfs.DetailsForFolder(u)
	}
	if ff == nil || err != nil {
		if err != nil && err != fancyfs.ErrNoMetadata {
			fyne.LogError("Could not read dir metadata", err)
		}

		// reset
		p.bg.File = ""
		p.bg.Resource = nil
		p.bg.Image = nil
		p.bg.FillMode = canvas.ImageFillContain // cover mode needs an image to measure
		p.bg.Refresh()

		return
	}

	if ff.BackgroundURI != nil {
		p.bg.File = ff.BackgroundURI.Path()
	} else {
		p.bg.File = ""
	}
	p.bg.Resource = ff.BackgroundResource
	p.bg.FillMode = ff.BackgroundFill
	p.bg.Refresh()
}

// SetListing asks the fyles panel to display a list of URIs.
// This supports manually creating a collection of items not in a standard directory.
func (p *Panel) SetListing(u []fyne.URI) {
	p.dir = nil
	p.content.Unselect(p.selected)

	var items []*fileData
	p.addListing(u, items)
}

// reload updates the panel after the item at old was renamed to updated,
// or removed when updated is nil.
func (p *Panel) reload(old, updated fyne.URI) {
	if p.dir != nil {
		p.SetDir(p.dir)
		return
	}

	// a custom listing cannot be re-read, so patch the items we were given
	list := make([]fyne.URI, 0, len(p.items))
	for _, item := range p.items {
		if item.location.String() == old.String() {
			if updated == nil {
				continue
			}
			item.location = updated
		}
		list = append(list, item.location)
	}
	p.SetListing(list)
}

func (p *Panel) addListing(list []fyne.URI, items []*fileData) {
	for _, item := range list {
		if p.Filter != nil && !p.Filter.Matches(item) {
			continue
		}

		dir, _ := storage.CanList(item)
		if archive.IsFile(item) {
			dir = true // browsed like a folder
		}
		items = append(items, &fileData{name: fileName(item), location: item, dir: dir})
	}

	p.items = items
	p.content.Refresh()
}
