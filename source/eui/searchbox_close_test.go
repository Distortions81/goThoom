package eui

import "testing"

func TestCloseSearchClearsState(t *testing.T) {
	win := &WindowData{Searchable: true, searchOpen: true, SearchText: "hi"}
	activeSearch = win
	var cb string
	win.OnSearch = func(s string) { cb = s }

	win.closeSearch()

	if win.SearchText != "" {
		t.Fatalf("expected empty SearchText, got %q", win.SearchText)
	}
	if cb != "" {
		t.Fatalf("expected OnSearch callback with empty string, got %q", cb)
	}
	if activeSearch != nil {
		t.Fatalf("expected activeSearch to be nil")
	}
}
