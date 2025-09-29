package main

import "testing"

func TestFixAreaJSONRemovesTrailingDots(t *testing.T) {
	src := []byte(`{"name":"Start".}`)
	cleaned, changed := fixAreaJSON(src)
	if !changed {
		t.Fatalf("expected change")
	}
	if string(cleaned) != `{"name":"Start"}` {
		t.Fatalf("unexpected output: %s", cleaned)
	}
}

func TestFixAreaJSONPreservesValidData(t *testing.T) {
	src := []byte(`{"name":"Start","neighbors":["South"]}`)
	cleaned, changed := fixAreaJSON(src)
	if changed {
		t.Fatalf("did not expect change")
	}
	if string(cleaned) != string(src) {
		t.Fatalf("data altered")
	}
}

func TestFixAreaJSONIgnoresDecimalValues(t *testing.T) {
	src := []byte(`{"value":1.25}`)
	cleaned, changed := fixAreaJSON(src)
	if changed {
		t.Fatalf("decimal value modified")
	}
	if string(cleaned) != string(src) {
		t.Fatalf("decimal altered")
	}
}
