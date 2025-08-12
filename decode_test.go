package main

import (
	"bytes"
	"testing"
)

func TestStripBEPPTags(t *testing.T) {
	in := []byte{'h', 'i', ' ', 0xC2, 'b', 'e', ' ', 'w', 'o', 'r', 'l', 'd'}
	got := stripBEPPTags(in)
	want := []byte("hi world")
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestStripBEPPTagsHighBit(t *testing.T) {
	in := []byte{'h', 0x8E, 0xC2, 't', '_', 't', 'g', 0x8F}
	got := stripBEPPTags(in)
	want := []byte{'h', 0x8E, 0x8F}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}
