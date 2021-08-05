package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"
)

type fyles struct {
	win        fyne.Window
	pwd        fyne.URI
	fileScroll *container.Scroll
	items      *fyne.Container

	current *fileItem
	filter  storage.FileFilter
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
