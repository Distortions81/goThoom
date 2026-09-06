package main

import "gothoom/eui"

var scriptEventsWin *eui.WindowData

func makeScriptEventsWindow() {
	if scriptEventsWin != nil {
		refreshscriptDebug()
		return
	}
	const width float32 = 640
	scriptEventsWin = eui.NewWindow()
	scriptEventsWin.Title = "Script Events"
	scriptEventsWin.Closable = true
	scriptEventsWin.AutoSize = true
	scriptEventsWin.Movable = true
	scriptEventsWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)
	scriptSection := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	debugCB, debugEvents := eui.NewCheckbox()
	debugCB.Text = "Record script events"
	debugCB.Size = eui.Point{X: width, Y: 24}
	debugCB.Checked = gs.scriptEventDebug
	debugEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.scriptEventDebug = ev.Checked
			settingsDirty = true
			scriptDebugList.Invisible = !ev.Checked
			if ev.Checked {
				refreshscriptDebug()
			} else {
				scriptEventsWin.Refresh()
			}
		}
	}
	scriptSection.AddItem(debugCB)

	scriptDebugList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	scriptDebugList.Size = eui.Point{X: width, Y: 300}
	scriptDebugList.Invisible = !gs.scriptEventDebug
	scriptSection.AddItem(scriptDebugList)

	scriptEventsWin.AddItem(scriptSection)
	scriptEventsWin.AddWindow(false)
	refreshscriptDebug()
}
