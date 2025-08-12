package eui

import "log"

// deallocImages releases any cached images for the item and its children.
func (item *itemData) deallocImages() {
	if item.Render != nil {
		if DebugMode {
			log.Printf("disposing render for item %p", item)
		}
		item.Render.Deallocate()
		item.Render = nil
	}
	if item.Image != nil {
		if DebugMode {
			log.Printf("disposing source image for item %p", item)
		}
		item.Image.Deallocate()
		item.Image = nil
	}
	if item.LabelImage != nil {
		if DebugMode {
			log.Printf("disposing label image for item %p", item)
		}
		item.LabelImage.Deallocate()
		item.LabelImage = nil
	}
	if item.nameImage != nil {
		if DebugMode {
			log.Printf("disposing name image for item %p", item)
		}
		item.nameImage.Deallocate()
		item.nameImage = nil
	}
	for _, child := range item.Contents {
		if child != nil {
			child.deallocImages()
		}
	}
	for _, tab := range item.Tabs {
		if tab != nil {
			tab.deallocImages()
		}
	}
}

// deallocImages releases cached images for all items in the window.
func (win *windowData) deallocImages() {
	if DebugMode {
		log.Printf("disposing images for window %p (%s)", win, win.Title)
	}
	if win.Render != nil {
		win.Render.Deallocate()
		win.Render = nil
	}
	for _, it := range win.Contents {
		if it != nil {
			it.deallocImages()
		}
	}
}
