package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/fyne-io/fyles/pkg/fyles"
)

type fylesUI struct {
	win        fyne.Window
	pwd        fyne.URI
	fileScroll *container.Scroll
	items      *panel.Panel
	filePath   *widget.Label

	filter storage.FileFilter
}

type filter struct{}

func (f *filter) Matches(u fyne.URI) bool {
	return u.Name()[0] != '.'
}

func filterHidden() storage.FileFilter {
	return &filter{}
}

type dirFilter struct {
	storage.FileFilter
}

func (f *dirFilter) Matches(u fyne.URI) bool {
	if !f.FileFilter.Matches(u) {
		return false
	}

	listable, _ := storage.CanList(u)
	return listable
}

func filterDir(files storage.FileFilter) storage.FileFilter {
	return &dirFilter{FileFilter: files}
}
