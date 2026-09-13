package main

import (
	"fmt"
	"sync/atomic"

	"gothoom/eui"
)

type musicSourceControl struct {
	id   SessionID
	item *eui.ItemData
}

var musicSourceControls []musicSourceControl
var musicSourceUIUpdateQueued atomic.Bool

func init() {
	queueMusicSourceUIUpdate = func() {
		if !musicSourceUIUpdateQueued.CompareAndSwap(false, true) {
			return
		}
		dispatchMainThread(func() {
			musicSourceUIUpdateQueued.Store(false)
			refreshMusicSourceControls()
		})
	}
}

func musicSourceLabel(id SessionID) string {
	label := fmt.Sprintf("Session %d", id)
	if session, ok := appSessions.session(id); ok {
		if name := session.characterName(); name != "" {
			return label + " — " + name
		}
	}
	return label
}

func refreshMusicSourceControls() {
	selected := musicSourceSessionID()
	for _, control := range musicSourceControls {
		if control.item == nil {
			continue
		}
		control.item.Text = musicSourceLabel(control.id)
		control.item.Checked = control.id == selected
		_, active := appSessions.session(control.id)
		control.item.Disabled = !active
		control.item.Dirty = true
	}
}

func addMusicSourceControls(parent *eui.ItemData, width float32, group string) {
	for slot := 0; slot < maxSessions; slot++ {
		id, _ := sessionIDForSlot(slot)
		radio, events := eui.NewRadio()
		radio.Text = musicSourceLabel(id)
		radio.RadioGroup = group
		radio.Size = eui.Point{X: width, Y: 24}
		radio.Checked = id == musicSourceSessionID()
		if _, active := appSessions.session(id); !active {
			radio.Disabled = true
		}
		sourceID := id
		events.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventRadioSelected {
				selectMusicSource(sourceID)
			}
		}
		musicSourceControls = append(musicSourceControls, musicSourceControl{id: id, item: radio})
		parent.AddItem(radio)
	}
}
