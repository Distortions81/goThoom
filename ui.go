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
		Size:      eui.Point{X: 256, Y: 256},
		Position:  eui.Point{X: 8, Y: 8},
		Open:      false,
		Closable:  false,
		Resizable: true,
		AutoSize:  true,
		Movable:   true,
	})

	mainFlow := &eui.ItemData{
		ItemType: eui.ITEM_FLOW,
		FlowType: eui.FLOW_VERTICAL,
	}
	var width float32 = 250

	scaleSlider, scaleEvents := eui.NewSlider(&eui.ItemData{Label: "Scaling", MinValue: 2, MaxValue: 5, Value: float32(scale), Size: eui.Point{X: width, Y: 24}, IntOnly: true})
	scaleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			scale = int(ev.Value)
			initFont()
			inputBg = nil
			ebiten.SetWindowSize(gameAreaSizeX*scale, gameAreaSizeY*scale)
		}
	}
	mainFlow.AddItem(scaleSlider)

	toggle, toggleEvents := eui.NewCheckbox(&eui.ItemData{Text: "Click-to-Toggle Walk", Size: eui.Point{X: width, Y: 24}, Checked: clickToToggle})
	toggleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			clickToToggle = ev.Checked
			if !clickToToggle {
				walkToggled = false
			}
		}
	}
	mainFlow.AddItem(toggle)

	filt, filtEvents := eui.NewCheckbox(&eui.ItemData{Text: "Image Filtering", Size: eui.Point{X: width, Y: 24}, Checked: linear})
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

	motion, motionEvents := eui.NewCheckbox(&eui.ItemData{Text: "Smooth Motion", Size: eui.Point{X: width, Y: 24}, Checked: interp})
	motionEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			interp = ev.Checked
		}
	}
	mainFlow.AddItem(motion)

	anim, animEvents := eui.NewCheckbox(&eui.ItemData{Text: "Character Frame Blending", Size: eui.Point{X: width, Y: 24}, Checked: onion})
	animEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			onion = ev.Checked
		}
	}
	mainFlow.AddItem(anim)

	pictBlend, pictBlendEvents := eui.NewCheckbox(&eui.ItemData{Text: "Object Frame Blending", Size: eui.Point{X: width, Y: 24}, Checked: blendPicts})
	pictBlendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			blendPicts = ev.Checked
		}
	}
	mainFlow.AddItem(pictBlend)

	blendSlider, blendEvents := eui.NewSlider(&eui.ItemData{Label: "Blend Rate", MinValue: 0.3, MaxValue: 1.0, Value: float32(blendRate), Size: eui.Point{X: width - 10, Y: 24}})
	blendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			blendRate = float64(ev.Value)
		}
	}
	mainFlow.AddItem(blendSlider)

	nightCB, nightEvents := eui.NewCheckbox(&eui.ItemData{Text: "Night Effects", Size: eui.Point{X: width, Y: 24}, Checked: nightMode})
	nightEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			nightMode = ev.Checked
		}
	}
	mainFlow.AddItem(nightCB)

	bubbleCB, bubbleEvents := eui.NewCheckbox(&eui.ItemData{Text: "Show Message Bubbles", Size: eui.Point{X: width, Y: 24}, Checked: showBubbles})
	bubbleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			showBubbles = ev.Checked
		}
	}
	mainFlow.AddItem(bubbleCB)

	textSlider, textEvents := eui.NewSlider(&eui.ItemData{Label: "Text Size", MinValue: 6, MaxValue: 18, Value: float32(mainFontSize), Size: eui.Point{X: width - 10, Y: 24}, IntOnly: true})
	textEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			mainFontSize = float64(ev.Value)
			initFont()
			inputBg = nil
		}
	}
	mainFlow.AddItem(textSlider)

	bubbleTextSlider, bubbleTextEvents := eui.NewSlider(&eui.ItemData{Label: "Bubble Text Size", MinValue: 6, MaxValue: 18, Value: float32(bubbleFontSize), Size: eui.Point{X: width - 10, Y: 24}, IntOnly: true})
	bubbleTextEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			bubbleFontSize = float64(ev.Value)
			initFont()
		}
	}
	mainFlow.AddItem(bubbleTextSlider)

	planesCB, planesEvents := eui.NewCheckbox(&eui.ItemData{Text: "Show image plane numbers", Size: eui.Point{X: width, Y: 24}, Checked: showPlanes})
	planesEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			showPlanes = ev.Checked
		}
	}
	mainFlow.AddItem(planesCB)

	hideMoveCB, hideMoveEvents := eui.NewCheckbox(&eui.ItemData{Text: "Hide Moving", Size: eui.Point{X: width, Y: 24}, Checked: hideMoving})
	hideMoveEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			hideMoving = ev.Checked
		}
	}
	mainFlow.AddItem(hideMoveCB)
	hideMobCB, hideMobEvents := eui.NewCheckbox(&eui.ItemData{Text: "Hide Mobiles", Size: eui.Point{X: width, Y: 24}, Checked: hideMobiles})
	hideMobEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			hideMobiles = ev.Checked
		}
	}
	mainFlow.AddItem(hideMobCB)

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
	playersBtn, playersEvents := eui.NewButton(&eui.ItemData{Text: "Players", Size: eui.Point{X: 128, Y: 24}, FontSize: 18})
	playersEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			playersWin.Open = !playersWin.Open
			if playersWin.Open {
				updatePlayersWindow()
			}
		}
	}
	overlay.AddItem(playersBtn)

	invBtn, invEvents := eui.NewButton(&eui.ItemData{Text: "Inventory", Size: eui.Point{X: 128, Y: 24}, FontSize: 18})
	invEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			inventoryWin.Open = !inventoryWin.Open
			if inventoryWin.Open {
				updateInventoryWindow()
			}
		}
	}
	overlay.AddItem(invBtn)

	btn, btnEvents := eui.NewButton(&eui.ItemData{Text: "Settings", Size: eui.Point{X: 128, Y: 24}, FontSize: 18})
	btnEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			settingsWin.Open = !settingsWin.Open
		}
	}
	overlay.AddItem(btn)
	eui.AddOverlayFlow(overlay)
}
