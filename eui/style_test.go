package eui

import (
	"reflect"
	"testing"
)

func TestLoadStyleDoesNotModifyDefaultStyle(t *testing.T) {
	orig := *defaultStyle
	origCurrentStyle := currentStyle
	origCurrentStyleName := currentStyleName
	defer func() {
		currentStyle = origCurrentStyle
		currentStyleName = origCurrentStyleName
	}()

	if err := LoadStyle("CleanLines"); err != nil {
		t.Fatalf("LoadStyle CleanLines: %v", err)
	}
	if !reflect.DeepEqual(*defaultStyle, orig) {
		t.Errorf("defaultStyle changed after first LoadStyle")
	}

	if err := LoadStyle("MinimalPro"); err != nil {
		t.Fatalf("LoadStyle MinimalPro: %v", err)
	}
	if !reflect.DeepEqual(*defaultStyle, orig) {
		t.Errorf("defaultStyle changed after second LoadStyle")
	}
}
