package main

import (
	"github.com/Distortions81/EUI/eui"
	"github.com/hajimehoshi/ebiten/v2"
)

var loginWin *eui.WindowData

func initUI() {

	if !noSplash && 1 == 2 {
		loginWin = eui.NewWindow(&eui.WindowData{
			Title:     "Login",
			Open:      true,
			Closable:  false,
			Resizable: false,
			AutoSize:  true,
			Movable:   true,
			PinTo:     eui.PIN_MID_CENTER,
		})

		loginFlow := &eui.ItemData{
			ItemType: eui.ITEM_FLOW,
			FlowType: eui.FLOW_VERTICAL,
		}

		hostInput, _ := eui.NewInput(&eui.ItemData{Label: "Host", TextPtr: &host, Size: eui.Point{X: 200, Y: 24}, Text: host})
		loginFlow.AddItem(hostInput)

		acctInput, _ := eui.NewInput(&eui.ItemData{Label: "Account", TextPtr: &account, Size: eui.Point{X: 200, Y: 24}})
		loginFlow.AddItem(acctInput)

		acctPassInput, _ := eui.NewInput(&eui.ItemData{Label: "Account Pass", TextPtr: &accountPass, Size: eui.Point{X: 200, Y: 24}})
		loginFlow.AddItem(acctPassInput)

		nameInput, _ := eui.NewInput(&eui.ItemData{Label: "Name", TextPtr: &name, Size: eui.Point{X: 200, Y: 24}})
		loginFlow.AddItem(nameInput)

		passInput, _ := eui.NewInput(&eui.ItemData{Label: "Character Password", TextPtr: &pass, Size: eui.Point{X: 200, Y: 24}})
		loginFlow.AddItem(passInput)

		connBtn, connEvents := eui.NewButton(&eui.ItemData{Text: "Connect", Size: eui.Point{X: 200, Y: 48}, Padding: 10})
		connEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				addMessage("Beep beep")
			}
		}

		loginFlow.AddItem(connBtn)

		loginWin.AddItem(loginFlow)
		loginWin.AddWindow(false)
	}

	settingsWin = eui.NewWindow(&eui.WindowData{
		Title:     "Settings",
		Open:      false,
		Closable:  false,
		Resizable: false,
		AutoSize:  true,
		Movable:   true,
	})

	mainFlow := &eui.ItemData{
		ItemType: eui.ITEM_FLOW,
		FlowType: eui.FLOW_VERTICAL,
	}

	filt, filtEvents := eui.NewCheckbox(&eui.ItemData{Text: "Image Filtering", Size: eui.Point{X: 150, Y: 24}, Checked: linear})
	filtEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			linear = ev.Checked
			if linear {
				drawFilter = ebiten.FilterLinear
			} else {
				drawFilter = ebiten.FilterNearest
			}
		}
	}
	mainFlow.AddItem(filt)

	motion, motionEvents := eui.NewCheckbox(&eui.ItemData{Text: "Smooth Motion", Size: eui.Point{X: 150, Y: 24}, Checked: interp})
	motionEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			interp = ev.Checked
		}
	}
	mainFlow.AddItem(motion)

	anim, animEvents := eui.NewCheckbox(&eui.ItemData{Text: "Animation Smoothing", Size: eui.Point{X: 150, Y: 24}, Checked: onion})
	animEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			onion = ev.Checked
		}
	}
	mainFlow.AddItem(anim)

	fastAnim, fastAnimEvents := eui.NewCheckbox(&eui.ItemData{Text: "Fast Animation", Size: eui.Point{X: 150, Y: 24}, Checked: fastAnimation})
	fastAnimEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			fastAnimation = ev.Checked
		}
	}
	mainFlow.AddItem(fastAnim)

	pictBlend, pictBlendEvents := eui.NewCheckbox(&eui.ItemData{Text: "Picture Blending", Size: eui.Point{X: 150, Y: 24}, Checked: blendPicts})
	pictBlendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			blendPicts = ev.Checked
		}
	}
	mainFlow.AddItem(pictBlend)

	nightCB, nightEvents := eui.NewCheckbox(&eui.ItemData{Text: "Night Mode", Size: eui.Point{X: 150, Y: 24}, Checked: nightMode})
	nightEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			nightMode = ev.Checked
		}
	}
	mainFlow.AddItem(nightCB)

	toggle, toggleEvents := eui.NewCheckbox(&eui.ItemData{Text: "Click-to-Toggle Walk", Size: eui.Point{X: 150, Y: 24}, Checked: clickToToggle})
	toggleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			clickToToggle = ev.Checked
			if !clickToToggle {
				walkToggled = false
			}
		}
	}
	mainFlow.AddItem(toggle)

	bubbleCB, bubbleEvents := eui.NewCheckbox(&eui.ItemData{Text: "Bubble Test Mode", Size: eui.Point{X: 150, Y: 24}, Checked: showBubbles})
	bubbleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			showBubbles = ev.Checked
		}
	}
	mainFlow.AddItem(bubbleCB)

	debugCB, debugEvents := eui.NewCheckbox(&eui.ItemData{Text: "Debug Mode", Size: eui.Point{X: 150, Y: 24}, Checked: debug})
	debugEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			debug = ev.Checked
			setDebugLogging(debug)
		}
	}
	mainFlow.AddItem(debugCB)

	planesCB, planesEvents := eui.NewCheckbox(&eui.ItemData{Text: "Planes Debug", Size: eui.Point{X: 150, Y: 24}, Checked: showPlanes})
	planesEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			showPlanes = ev.Checked
		}
	}
	mainFlow.AddItem(planesCB)

	silentCB, silentEvents := eui.NewCheckbox(&eui.ItemData{Text: "Silence Errors", Size: eui.Point{X: 150, Y: 24}, Checked: silent})
	silentEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			silent = ev.Checked
		}
	}
	mainFlow.AddItem(silentCB)

	denoiseCB, denoiseEvents := eui.NewCheckbox(&eui.ItemData{Text: "Denoiser", Size: eui.Point{X: 150, Y: 24}, Checked: denoise})
	denoiseEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			denoise = ev.Checked
			if clImages != nil {
				clImages.Denoise = denoise
			}
		}
	}
	mainFlow.AddItem(denoiseCB)

	settingsWin.AddItem(mainFlow)
	settingsWin.AddWindow(false)

	settingsWin.Open = false

	inventoryWin = eui.NewWindow(&eui.WindowData{
		Title:     "Inventory",
		Open:      false,
		Closable:  false,
		Resizable: false,
		AutoSize:  true,
		Movable:   true,
	})
	inventoryList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	title, _ := eui.NewText(&eui.ItemData{Text: "Inventory", Size: eui.Point{X: 256, Y: 128}})
	inventoryWin.AddItem(title)
	inventoryWin.AddItem(inventoryList)
	inventoryWin.Open = false
	inventoryWin.AddWindow(false)

	playersWin = eui.NewWindow(&eui.WindowData{
		Title:     "Players",
		Open:      false,
		Closable:  false,
		Resizable: false,
		AutoSize:  false,
		Movable:   true,
		Size:      eui.Point{X: 128, Y: 384},
	})
	playersList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	playersWin.AddItem(playersList)
	playersWin.Open = false
	playersWin.AddWindow(false)
	playersWin.Resizable = true
	playersWin.AutoSize = true

	overlay := &eui.ItemData{
		ItemType: eui.ITEM_FLOW,
		FlowType: eui.FLOW_HORIZONTAL,
		PinTo:    eui.PIN_BOTTOM_RIGHT,
	}
	playersBtn, playersEvents := eui.NewButton(&eui.ItemData{Text: "P", Size: eui.Point{X: 36, Y: 36}, FontSize: 27})
	playersEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			playersWin.Open = !playersWin.Open
			if playersWin.Open {
				updatePlayersWindow()
			}
		}
	}
	overlay.AddItem(playersBtn)

	invBtn, invEvents := eui.NewButton(&eui.ItemData{Text: "I", Size: eui.Point{X: 36, Y: 36}, FontSize: 27})
	invEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			inventoryWin.Open = !inventoryWin.Open
			if inventoryWin.Open {
				updateInventoryWindow()
			}
		}
	}
	overlay.AddItem(invBtn)

	btn, btnEvents := eui.NewButton(&eui.ItemData{Text: "...", Size: eui.Point{X: 36, Y: 36}, FontSize: 27})
	btnEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			settingsWin.Open = !settingsWin.Open
		}
	}
	overlay.AddItem(btn)
	eui.AddOverlayFlow(overlay)
}
