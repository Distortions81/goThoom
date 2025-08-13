package eui

// WindowZone represents a saved window zone.
type WindowZone struct {
	H HZone
	V VZone
}

// WindowZoneState captures whether a window is zoned and its zone.
type WindowZoneState struct {
	Zoned bool
	Zone  WindowZone
}

// SaveWindowZones returns a table of window titles to their zone state.
func SaveWindowZones() map[string]WindowZoneState {
	table := make(map[string]WindowZoneState, len(windows))
	for _, win := range windows {
		st := WindowZoneState{}
		if win.zone != nil {
			st.Zoned = true
			st.Zone = WindowZone{H: win.zone.h, V: win.zone.v}
		}
		table[win.Title] = st
	}
	return table
}

// LoadWindowZones restores window zone states from the provided table.
func LoadWindowZones(table map[string]WindowZoneState) {
	for _, win := range windows {
		if st, ok := table[win.Title]; ok {
			if st.Zoned {
				win.SetZone(st.Zone.H, st.Zone.V)
			} else {
				win.ClearZone()
			}
		}
	}
}
