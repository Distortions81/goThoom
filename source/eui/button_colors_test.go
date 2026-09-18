package eui

import "testing"

func TestButtonColorsOverrideThemeAndKeepDisabledAppearance(t *testing.T) {
	button, _ := NewButton()
	ordinary, _ := NewButton()
	base := *ordinary.themeStyle()
	button.SetButtonColors(ColorDarkRed, ColorRed)
	style := button.themeStyle()
	if style.Color != ColorDarkRed || style.HoverColor != ColorRed || style.ClickColor != ColorRed {
		t.Fatal("action colors did not reach the rendered button style")
	}
	if ordinary.themeStyle().Color != base.Color || style.TextColor != base.TextColor {
		t.Fatal("action colors changed the shared theme or its caption colors")
	}
	if disabledStyle(style).Color != base.DisabledColor {
		t.Fatal("custom button lost its disabled appearance")
	}
}

func TestPopupActionColorsReachButtonStyle(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	popup := ShowPopup("Confirm", "", []PopupButton{{Text: "Cancel"}, {Text: "Play", Color: &ColorDarkRed, HoverColor: &ColorRed}})
	t.Cleanup(popup.Close)
	button := popup.Contents[0].Contents[0].Contents[1]
	if style := button.themeStyle(); style.Color != ColorDarkRed || style.HoverColor != ColorRed {
		t.Fatal("popup ignored its explicit action colors")
	}
}
