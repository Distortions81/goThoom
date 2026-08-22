//go:build test

package eui

// SetActiveSearchForTest sets the active search window for tests.
func SetActiveSearchForTest(win *WindowData) {
	activeSearch = win
	if win != nil {
		win.searchOpen = true
	}
}
