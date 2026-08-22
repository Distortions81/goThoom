package eui

import "testing"

func TestURLRangesUseRuneIndexes(t *testing.T) {
	item := &itemData{Text: "café https://example.com/path?x=1)"}
	spans := item.urlRanges()
	if len(spans) != 1 {
		t.Fatalf("urlRanges() = %d spans, want 1", len(spans))
	}
	if got, want := item.urlAtChar(spans[0].Start), "https://example.com/path?x=1"; got != want {
		t.Fatalf("urlAtChar() = %q, want %q", got, want)
	}
	if got := item.urlAtChar(spans[0].End); got != "" {
		t.Fatalf("urlAtChar(end) = %q, want empty", got)
	}
}
