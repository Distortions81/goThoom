package main

import (
	"testing"

	"gothoom/eui"
)

func TestViewportImageBuffersAreIndependent(t *testing.T) {
	first := &viewportRenderState{window: newGameRenderWindow()}
	second := &viewportRenderState{window: newGameRenderWindow()}
	first.window.Size = eui.Point{X: 400, Y: 300}
	second.window.Size = eui.Point{X: 400, Y: 300}

	updateViewportImageSize(first, false)
	updateViewportImageSize(second, false)
	t.Cleanup(func() {
		if first.imageBacking != nil {
			first.imageBacking.Deallocate()
		}
		if second.imageBacking != nil {
			second.imageBacking.Deallocate()
		}
	})

	if first.image == nil || second.image == nil {
		t.Fatal("viewport images were not allocated")
	}
	if first.image == second.image || first.imageBacking == second.imageBacking || first.imageItem == second.imageItem {
		t.Fatal("viewport image resources alias each other")
	}

	secondBacking := second.imageBacking
	first.window.Size = eui.Point{X: 800, Y: 600}
	updateViewportImageSize(first, false)
	if second.imageBacking != secondBacking {
		t.Fatal("resizing one viewport replaced another viewport's backing image")
	}
}

func TestSecondaryViewportWindowIsTransparentToWorldInput(t *testing.T) {
	oldViewports := appViewports
	oldGameWindow := gameWin
	t.Cleanup(func() {
		appViewports = oldViewports
		gameWin = oldGameWindow
	})

	appViewports = newViewportManager()
	appViewports.enableMulti(viewportLayoutFreeform)
	state := appViewports.renderStateForViewport(2)
	state.window = eui.NewWindow()

	if uiOwnsPointerPress(false, state.window, true) {
		t.Fatal("secondary playfield retained a world press")
	}
	if !isViewportWindow(state.window) {
		t.Fatal("secondary playfield window was not recognized")
	}
}
