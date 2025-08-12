//go:build !nomac

package main

import "golang.org/x/text/encoding/charmap"

func decodeMacRoman(b []byte) string {
	s, err := charmap.Macintosh.NewDecoder().Bytes(b)
	if err != nil {
		return string(b)
	}
	return string(s)
}

func encodeMacRoman(s string) []byte {
	b, err := charmap.Macintosh.NewEncoder().Bytes([]byte(s))
	if err != nil {
		return []byte(s)
	}
	return b
}
