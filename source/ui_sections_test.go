package main

import (
	"testing"

	"gothoom/eui"
)

func TestNewConfigurationSectionProvidesHeadingAndSpacing(t *testing.T) {
	const width float32 = 270
	section := eui.NewSection("Appearance", width)

	if section.ItemType != eui.ITEM_FLOW || section.FlowType != eui.FLOW_VERTICAL {
		t.Fatalf("section is not a vertical flow: type=%v flow=%v", section.ItemType, section.FlowType)
	}
	if section.Size.X < width {
		t.Fatalf("section width = %v, want at least %v", section.Size.X, width)
	}
	if len(section.Contents) != 2 {
		t.Fatalf("section has %d built-in items, want spacer and heading", len(section.Contents))
	}
	if section.Contents[0].Size.Y <= 0 {
		t.Fatal("section spacer must have a positive height")
	}
	if section.Contents[1].Text != "Appearance" {
		t.Fatalf("section heading = %q, want Appearance", section.Contents[1].Text)
	}
}
