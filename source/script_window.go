package main

import (
	"fmt"
	"strings"
	"sync"

	"gothoom/eui"
	scriptapi "gt2"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Window is an opaque handle; scripts never receive mutable EUI objects.
type Window struct{ state *scriptWindowState }

type scriptWindowState struct {
	mu        sync.Mutex
	owner     string
	candidate *scriptCandidate
	handle    scriptRegistrationHandle
	removed   bool
	ui        *eui.WindowData
	label     *eui.ItemData
	buttons   map[string]*eui.ItemData
	controls  map[string]*scriptWindowControl
	text      string
	width     float32
	scale     float32
}

func validateScriptWindowOptions(options scriptapi.WindowOptions) error {
	if len(options.Buttons) > 8 {
		return fmt.Errorf("script window supports at most eight buttons")
	}
	ids := map[string]bool{}
	for _, b := range options.Buttons {
		if strings.TrimSpace(b.ID) == "" || ids[b.ID] {
			return fmt.Errorf("script window button IDs must be nonempty and unique")
		}
		if strings.TrimSpace(b.Label) == "" || b.OnClick == nil {
			return fmt.Errorf("script window button %q needs a label and OnClick", b.ID)
		}
		ids[b.ID] = true
	}
	return validateScriptWindowControls(options, ids)
}

// create and all update closures run on the client thread after staging.
func (w Window) create(options scriptapi.WindowOptions) {
	s := w.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.removed {
		return
	}
	s.handle = registerScriptResource(s.owner, func() { w.Remove() })
	if !s.handle.valid() {
		s.removed = true
		return
	}
	s.width = float32(options.Width)
	if s.width == 0 {
		s.width = 340
	}
	s.width = float32(max(220, min(800, int(s.width))))
	s.ui = eui.NewWindow()
	s.ui.Title = strings.TrimSpace(options.Title)
	if s.ui.Title == "" {
		s.ui.Title = scriptDisplayName(s.owner)
	}
	s.ui.Movable, s.ui.Closable, s.ui.AutoSize = true, true, true
	s.ui.Resizable = false
	s.ui.SetZone(eui.HZoneCenter, eui.VZoneCenter)
	queue := s.handle.queue
	if options.OnClose != nil {
		s.ui.OnClose = func() { queueScriptCallbackOn(queue, s.owner, "Window close", options.OnClose) }
	}
	s.label, _ = eui.NewText()
	s.label.FontSize = 14
	s.label.Face = nil
	s.setText(options.Text)
	column := eui.NewColumn(s.label)
	s.addControls(column, options.Controls)
	s.buttons = make(map[string]*eui.ItemData, len(options.Buttons))
	var row *eui.ItemData
	for i, option := range options.Buttons {
		if i%2 == 0 {
			row = eui.NewRow()
			column.AddItem(row)
		}
		button, events := eui.NewButton()
		button.Text = option.Label
		button.Size = eui.Point{X: (s.width - 16) / 2, Y: 34}
		button.Disabled = option.Disabled
		button.SetTooltip(option.Tooltip)
		s.buttons[option.ID] = button
		events.Handle = func(event eui.UIEvent) {
			if event.Type != eui.EventClick {
				return
			}
			// Recheck enabled/removal at execution time, as a queued click can outlive
			// a status update. No script code runs while the window mutex is held.
			queueScriptCallbackOn(queue, s.owner, "Window button "+option.ID, func() {
				s.mu.Lock()
				enabled := !s.removed && !button.Disabled
				s.mu.Unlock()
				if enabled {
					option.OnClick()
				}
			})
		}
		row.AddItem(button)
	}
	s.ui.AddItem(column)
	s.ui.OnResize = func() {
		if s.scale != eui.UIScale() {
			s.setText(s.text)
			s.ui.Refresh()
		}
	}
	s.ui.AddWindow(false)
	s.ui.MarkOpen()
}

func (s *scriptWindowState) setText(value string) {
	s.text = value
	s.scale = eui.UIScale()
	face := &text.GoTextFace{Source: eui.FontSource(), Size: float64(s.label.FontSize*s.scale + 2)}
	_, lines := eui.WrapText(value, face, float64((s.width-8)*s.scale))
	s.label.Text = strings.Join(lines, "\n")
	s.label.Fixed = false
	s.label.Size = eui.Point{}
	height := s.label.GetSize().Y / eui.UIScale()
	s.label.Size = eui.Point{X: s.width, Y: max(64, height+4)}
	s.label.Fixed = true
}

func (w Window) update(action func(*scriptWindowState)) {
	if w.state == nil {
		return
	}
	s := w.state
	s.candidate.dispatch(s.owner, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if !s.removed && s.ui != nil {
			action(s)
		}
	})
}

func (w Window) SetText(value string) {
	w.update(func(s *scriptWindowState) {
		if s.text == value && s.scale == eui.UIScale() {
			return
		}
		s.setText(value)
		s.ui.Refresh()
	})
}

func (w Window) SetButtonEnabled(id string, enabled bool) {
	w.update(func(s *scriptWindowState) {
		if b := s.buttons[id]; b != nil && b.Disabled == enabled {
			b.Disabled = !enabled
			s.ui.Refresh()
		}
	})
}

func (w Window) Show() { w.update(func(s *scriptWindowState) { s.ui.MarkOpen(); s.ui.Refresh() }) }
func (w Window) Hide() {
	w.update(func(s *scriptWindowState) {
		if s.ui.IsOpen() {
			s.ui.Close()
		}
	})
}

func (w Window) Remove() {
	if w.state == nil {
		return
	}
	s := w.state
	s.mu.Lock()
	if s.removed {
		s.mu.Unlock()
		return
	}
	s.removed = true
	handle, ui := s.handle, s.ui
	s.handle = scriptRegistrationHandle{}
	s.mu.Unlock()
	// Control actions survive script shutdown. The captured pointer ensures an
	// old window's cleanup cannot remove a replacement opened by a reload.
	dispatchScriptControl(func() {
		if ui != nil {
			ui.OnClose = nil
			ui.RemoveWindow()
		}
		handle.release()
	})
}

func (w Window) Active() bool {
	if w.state == nil {
		return false
	}
	s := w.state
	s.mu.Lock()
	active, handle := !s.removed, s.handle
	s.mu.Unlock()
	return active && handle.valid() && scriptEventQueueIsCurrent(s.owner, handle.queue)
}
