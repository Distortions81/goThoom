package eui

import "testing"

func TestScrollbarHitRegions(t *testing.T) {
	previousScale := UIScale()
	t.Cleanup(func() { SetUIScale(previousScale) })
	for _, scale := range []float32{1, 1.5, 2} {
		SetUIScale(scale)
		win := NewWindow()
		win.Padding, win.BorderPad, win.TitleHeight = 0, 0, 0
		win.Position, win.Size = point{30, 40}, point{300, 200}
		win.Contents = []*itemData{{ItemType: ITEM_FLOW, Fixed: true, Size: point{600, 600}}}
		pos, size := win.getPosition(), win.GetSize()
		for _, test := range []struct {
			point point
			want  dragType
		}{
			{point{pos.X + size.X - 1, pos.Y + 10}, PART_SCROLL_V},
			{point{pos.X + 10, pos.Y + size.Y - 1}, PART_SCROLL_H},
			{point{pos.X + size.X/2, pos.Y + size.Y/2}, PART_NONE},
			{point{pos.X + size.X - 1, pos.Y + size.Y - 20}, PART_NONE},
			{point{pos.X - 1, pos.Y + 10}, PART_NONE},
		} {
			if got := win.getScrollbarPart(test.point); got != test.want {
				t.Fatalf("scale %.1f at %+v: got %v, want %v", scale, test.point, got, test.want)
			}
		}
	}
}

func TestScrollbarThumbLength(t *testing.T) {
	tests := []struct {
		name           string
		track, content float32
		scale          float32
		want           float32
	}{
		{name: "proportional", track: 200, content: 400, scale: 1, want: 100},
		{name: "minimum", track: 200, content: 4000, scale: 1, want: minScrollbarThumbSize},
		{name: "scaled minimum", track: 200, content: 4000, scale: 2, want: minScrollbarThumbSize * 2},
		{name: "minimum limited by track", track: 30, content: 3000, scale: 1, want: 30},
		{name: "content fits", track: 200, content: 100, scale: 1, want: 200},
		{name: "empty track", track: 0, content: 100, scale: 1, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scrollbarThumbLength(tt.track, tt.content, tt.scale); got != tt.want {
				t.Fatalf("scrollbarThumbLength(%v, %v, %v) = %v, want %v", tt.track, tt.content, tt.scale, got, tt.want)
			}
		})
	}
}
