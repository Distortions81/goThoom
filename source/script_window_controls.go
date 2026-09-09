package main

import (
	"fmt"
	"strings"

	"gothoom/eui"
	scriptapi "gt2"
)

type scriptWindowControl struct {
	option   scriptapi.WindowControl
	item     *eui.ItemData
	revision uint64
}

func cloneScriptWindowOptions(options scriptapi.WindowOptions) scriptapi.WindowOptions {
	options.Buttons = append([]scriptapi.WindowButton(nil), options.Buttons...)
	options.Controls = append([]scriptapi.WindowControl(nil), options.Controls...)
	for i := range options.Controls {
		options.Controls[i].Options = append([]string(nil), options.Controls[i].Options...)
	}
	return options
}
func validateScriptWindowControls(options scriptapi.WindowOptions, ids map[string]bool) error {
	if len(options.Controls) > 32 {
		return fmt.Errorf("script window supports at most 32 controls")
	}
	for _, c := range options.Controls {
		if strings.TrimSpace(c.ID) == "" || ids[c.ID] {
			return fmt.Errorf("script window control and button IDs must be nonempty and unique")
		}
		ids[c.ID] = true
		if strings.TrimSpace(c.Label) == "" {
			return fmt.Errorf("script window control %q needs a label", c.ID)
		}
		switch c.Kind {
		case scriptapi.ControlText, scriptapi.ControlCheckbox:
		case scriptapi.ControlDropdown, scriptapi.ControlList:
			if len(c.Options) > 256 || c.Selected < -1 || c.Selected >= len(c.Options) && !(len(c.Options) == 0 && c.Selected == 0) {
				return fmt.Errorf("script window control %q has invalid options or selection", c.ID)
			}
		default:
			return fmt.Errorf("script window control %q has unknown kind %q", c.ID, c.Kind)
		}
	}
	return nil
}
func (s *scriptWindowState) addControls(column *eui.ItemData, options []scriptapi.WindowControl) {
	s.controls = make(map[string]*scriptWindowControl, len(options))
	for _, option := range options {
		c := &scriptWindowControl{option: option}
		s.controls[option.ID] = c
		if len(c.option.Options) == 0 {
			c.option.Selected = -1
		}
		var events *eui.EventHandler
		switch option.Kind {
		case scriptapi.ControlText:
			c.item, events = eui.NewInput()
			c.item.Text = option.Text
		case scriptapi.ControlCheckbox:
			c.item, events = eui.NewCheckbox()
			c.item.Text = option.Label
			c.item.Checked = option.Checked
		case scriptapi.ControlDropdown:
			c.item, events = eui.NewDropdown()
			c.item.Options = append([]string(nil), option.Options...)
			c.item.Selected = c.option.Selected
		case scriptapi.ControlList:
			c.item = eui.NewColumn()
			c.item.Fixed, c.item.Scrollable = true, true
			c.item.Size = eui.Point{X: s.width, Y: 140}
			s.rebuildList(c)
		}
		c.item.Disabled = option.Disabled
		c.item.SetTooltip(option.Tooltip)
		if option.Kind != scriptapi.ControlList {
			c.item.Size.X = s.width
		}
		if option.Kind != scriptapi.ControlCheckbox {
			column.AddItem(eui.NewLabel(option.Label))
		}
		column.AddItem(c.item)
		if events != nil {
			events.Handle = func(event eui.UIEvent) { s.controlEdited(c, event, c.revision) }
		}
	}
}
func (s *scriptWindowState) rebuildList(c *scriptWindowControl) {
	c.item.Contents = nil
	c.item.Scroll = eui.Point{}
	revision := c.revision
	for i, label := range c.option.Options {
		row, events := eui.NewRadio()
		row.Text, row.Checked, row.Disabled = label, i == c.option.Selected, c.option.Disabled
		row.Size = eui.Point{X: s.width - 20, Y: 28}
		row.RadioGroup = fmt.Sprintf("script-list-%p", c)
		events.Handle = func(event eui.UIEvent) {
			if event.Type == eui.EventRadioSelected {
				event.Index = i
				s.controlEdited(c, event, revision)
			}
		}
		c.item.AddItem(row)
	}
}
func (s *scriptWindowState) controlEdited(c *scriptWindowControl, event eui.UIEvent, revision uint64) {
	s.mu.Lock()
	if s.removed || c.option.Disabled || c.revision != revision {
		s.mu.Unlock()
		return
	}
	switch c.option.Kind {
	case scriptapi.ControlText:
		if event.Type != eui.EventInputChanged {
			s.mu.Unlock()
			return
		}
		c.option.Text, c.item.Text = event.Text, event.Text
	case scriptapi.ControlCheckbox:
		if event.Type != eui.EventCheckboxChanged {
			s.mu.Unlock()
			return
		}
		c.option.Checked, c.item.Checked = event.Checked, event.Checked
	case scriptapi.ControlDropdown, scriptapi.ControlList:
		if event.Type != eui.EventDropdownSelected && event.Type != eui.EventRadioSelected || event.Index < 0 || event.Index >= len(c.option.Options) {
			s.mu.Unlock()
			return
		}
		c.option.Selected = event.Index
		s.syncControlSelection(c)
	}
	snapshot := scriptapi.WindowControlEvent{ID: c.option.ID, Text: c.option.Text, Checked: c.option.Checked, Selected: c.option.Selected}
	if c.option.Kind == scriptapi.ControlList || c.option.Kind == scriptapi.ControlDropdown {
		if snapshot.Selected >= 0 {
			snapshot.Text = c.option.Options[snapshot.Selected]
		}
	}
	fn := c.option.OnChange
	queue := s.handle.queue
	s.mu.Unlock()
	if fn != nil {
		queueScriptCallbackOn(queue, s.owner, "Window control "+snapshot.ID, func() {
			s.mu.Lock()
			enabled := !s.removed && !c.option.Disabled && c.revision == revision
			s.mu.Unlock()
			if enabled {
				fn(snapshot)
			}
		})
	}
}
func (s *scriptWindowState) syncControlSelection(c *scriptWindowControl) {
	c.item.Selected = c.option.Selected
	if c.option.Kind == scriptapi.ControlList {
		for i, row := range c.item.Contents {
			row.Checked = i == c.option.Selected
		}
	}
	s.ui.Refresh()
}
func (w Window) SetControlText(id, value string) {
	w.update(func(s *scriptWindowState) {
		if c := s.controls[id]; c != nil && c.option.Kind == scriptapi.ControlText {
			c.option.Text, c.item.Text = value, value
			c.revision++
			s.ui.Refresh()
		}
	})
}
func (w Window) SetControlChecked(id string, checked bool) {
	w.update(func(s *scriptWindowState) {
		if c := s.controls[id]; c != nil && c.option.Kind == scriptapi.ControlCheckbox {
			c.option.Checked, c.item.Checked = checked, checked
			c.revision++
			s.ui.Refresh()
		}
	})
}
func (w Window) SetControlSelected(id string, selected int) {
	w.update(func(s *scriptWindowState) {
		if c := s.controls[id]; c != nil && (c.option.Kind == scriptapi.ControlDropdown || c.option.Kind == scriptapi.ControlList) && selected >= -1 && selected < len(c.option.Options) {
			c.option.Selected = selected
			s.syncControlSelection(c)
		}
	})
}
func (w Window) SetControlOptions(id string, options []string) {
	if len(options) > 256 {
		return
	}
	options = append([]string(nil), options...)
	w.update(func(s *scriptWindowState) {
		if c := s.controls[id]; c != nil && (c.option.Kind == scriptapi.ControlDropdown || c.option.Kind == scriptapi.ControlList) {
			c.option.Options, c.option.Selected = options, -1
			c.revision++
			if c.option.Kind == scriptapi.ControlList {
				s.rebuildList(c)
			} else {
				c.item.Options, c.item.Selected, c.item.Open = append([]string(nil), options...), -1, false
			}
			s.ui.Refresh()
		}
	})
}
func (w Window) SetControlEnabled(id string, enabled bool) {
	w.update(func(s *scriptWindowState) {
		if c := s.controls[id]; c != nil {
			c.option.Disabled, c.item.Disabled = !enabled, !enabled
			for _, row := range c.item.Contents {
				row.Disabled = !enabled
			}
			s.ui.Refresh()
		}
	})
}
