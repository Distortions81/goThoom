//go:build integration
// +build integration

package main

import (
	"reflect"
	"testing"
)

// helper to wrap a name in -pn tags
func pn(name string) []byte {
	b := []byte{0xC2, 'p', 'n'}
	b = append(b, []byte(name)...)
	b = append(b, 0xC2, 'p', 'n')
	return b
}

func TestParseNamesSkipsDelimiters(t *testing.T) {
	tests := []struct {
		data []byte
		want []string
	}{
		{
			data: func() []byte {
				d := pn("Alice")
				d = append(d, []byte(", and ")...)
				d = append(d, pn("Bob")...)
				return d
			}(),
			want: []string{"Alice", "Bob"},
		},
		{
			data: func() []byte {
				d := pn("Chad")
				d = append(d, []byte(" and ")...)
				d = append(d, pn("Dana")...)
				return d
			}(),
			want: []string{"Chad", "Dana"},
		},
		{
			data: func() []byte {
				d := pn("Eve")
				d = append(d, []byte(", ")...)
				d = append(d, pn("Finn")...)
				return d
			}(),
			want: []string{"Eve", "Finn"},
		},
	}
	for _, tt := range tests {
		got := parseNames(tt.data)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parseNames() = %v, want %v", got, tt.want)
		}
	}
}

func TestParseNamesMacRoman(t *testing.T) {
	nameBytes := []byte{'M', 0x8e, 'm', 'e'}
	data := []byte{0xC2, 'p', 'n'}
	data = append(data, nameBytes...)
	data = append(data, 0xC2, 'p', 'n')
	got := parseNames(data)
	want := []string{"Meme"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseNames() = %v, want %v", got, want)
	}
}
