package main

import (
	"gothoom/eui"
	scriptapi "gt2"
)

func scriptRowWidth(width int) float32 {
	if width == 0 {
		return 96
	}
	return float32(max(24, min(800, width)))
}

func (s *scriptWindowState) rebuildRows(rows []scriptapi.WindowRow) {
	for id := range s.rowIDs {
		if c := s.controls[id]; c != nil {
			c.revision++
			c.option.Disabled = true
			c.item.Disabled = true
			delete(s.controls, id)
		}
		if b := s.buttons[id]; b != nil {
			b.Disabled = true
			delete(s.buttons, id)
		}
	}
	s.rowIDs = map[string]bool{}
	s.rows.Contents = nil
	if len(rows) == 0 {
		s.rowList, s.rowCount = nil, 0
		return
	}
	header := eui.NewRow()
	for _, control := range rows[0].Controls {
		label := eui.NewLabel(control.Label)
		label.Fixed = true
		label.Size = eui.Point{X: scriptRowWidth(control.Width), Y: 20}
		header.AddItem(label)
	}
	s.rows.AddItem(header)
	list := eui.NewColumn()
	if s.rowList != nil {
		list.Scroll = s.rowList.Scroll
		if len(rows) > s.rowCount {
			list.Scroll.Y = 1e9 // Layout clamps to the bottom so appended rows are visible.
		}
	}
	s.rowList, s.rowCount = list, len(rows)
	list.Fixed, list.Scrollable = true, true
	list.Size = eui.Point{X: s.width, Y: float32(min(280, len(rows)*52))}
	for _, option := range rows {
		row := eui.NewRow()
		for _, control := range option.Controls {
			s.addControls(row, []scriptapi.WindowControl{control}, scriptRowWidth(control.Width), true)
			s.rowIDs[control.ID] = true
		}
		for _, button := range option.Buttons {
			s.addButtons(row, []scriptapi.WindowButton{button}, scriptRowWidth(button.Width), true)
			s.rowIDs[button.ID] = true
		}
		list.AddItem(row)
	}
	s.rows.AddItem(list)
}

func (w Window) SetRows(rows []scriptapi.WindowRow) {
	rows = cloneScriptWindowOptions(scriptapi.WindowOptions{Rows: rows}).Rows
	w.update(func(s *scriptWindowState) {
		options := s.options
		options.Rows = rows
		if err := validateScriptWindowOptions(options); err != nil {
			return
		}
		s.options.Rows = rows
		s.rebuildRows(rows)
		s.ui.Refresh()
	})
}
