package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

type dirTapPanel struct {
	widget.BaseWidget

	parent *fyles
}

func (d *dirTapPanel) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (d *dirTapPanel) TappedSecondary(ev *fyne.PointEvent) {
	m := d.buildMenu(d.parent.pwd)
	widget.ShowPopUpMenuAtPosition(m, d.parent.win.Canvas(), ev.AbsolutePosition)
}

func (d *dirTapPanel) buildMenu(u fyne.URI) *fyne.Menu {
	return fyne.NewMenu(u.Name(),
		fyne.NewMenuItem("Copy folder path", func() {
			d.parent.win.Clipboard().SetContent(u.Path())
		}),
	)
}

func newDirTapPanel(ui *fyles) *dirTapPanel {
	d := &dirTapPanel{parent: ui}
	d.ExtendBaseWidget(d)
	return d
}
