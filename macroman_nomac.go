//go:build nomac

package main

func decodeMacRoman(b []byte) string { return string(b) }

func encodeMacRoman(s string) []byte { return []byte(s) }
