package eui

import "log"

// disposeImages releases any cached images for the item and its children.
func (item *itemData) disposeImages() {
	if item.Render != nil {
		if DebugMode {
			log.Printf("disposing render for item %p", item)
		}
		item.Render.Dispose()
		item.Render = nil
	}
	if item.Image != nil {
		if DebugMode {
			log.Printf("disposing source image for item %p", item)
		}
		item.Image.Dispose()
		item.Image = nil
	}
	for _, child := range item.Contents {
		if child != nil {
			child.disposeImages()
		}
	}
	for _, tab := range item.Tabs {
		if tab != nil {
			tab.disposeImages()
		}
	}
}

// disposeImages releases cached images for all items in the window.
func (win *windowData) disposeImages() {
	if DebugMode {
		log.Printf("disposing images for window %p (%s)", win, win.Title)
	}
	for _, it := range win.Contents {
		if it != nil {
			it.disposeImages()
		}
	}
}
