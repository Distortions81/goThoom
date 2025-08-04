package main

import (
	"github.com/Distortions81/EUI/eui"
	"github.com/hajimehoshi/ebiten/v2"
)

var loginWin *eui.WindowData

func initUI() {

	if !noSplash {
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

		//hostInput, _ := eui.NewInput(&eui.ItemData{Label: "Host", TextPtr: &host, Size: eui.Point{X: 200, Y: 24}})
		//loginFlow.AddItem(hostInput)

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

	settingsWin.AddItem(mainFlow)
	settingsWin.AddWindow(false)

	settingsWin.Open = false

	overlay := &eui.ItemData{
		ItemType: eui.ITEM_FLOW,
		FlowType: eui.FLOW_HORIZONTAL,
		PinTo:    eui.PIN_BOTTOM_RIGHT,
	}
	btn, btnEvents := eui.NewButton(&eui.ItemData{Text: "...", Size: eui.Point{X: 12, Y: 12}, FontSize: 9})
	btnEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			settingsWin.Open = !settingsWin.Open
		}
	}
	overlay.AddItem(btn)
	eui.AddOverlayFlow(overlay)
}
