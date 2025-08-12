package main

import (
	"strings"
	"testing"
)

func TestParseThinkTextTargets(t *testing.T) {
	cases := []struct {
		raw    string
		target thinkTarget
	}{
		{"Torx\xC2t_th: hello", thinkNone},
		{"Torx\xC2t_tt to you: hello", thinkToYou},
		{"Torx\xC2t_tc to your clan: hello", thinkToClan},
		{"Torx\xC2t_tg to a group: hello", thinkToGroup},
	}
	for _, tc := range cases {
		raw := []byte(tc.raw)
		text := strings.TrimSpace(decodeMacRoman(stripBEPPTags(raw)))
		name, target, msg := parseThinkText(raw, text)
		if name != "Torx" {
			t.Errorf("name = %q, want Torx", name)
		}
		if msg != "hello" {
			t.Errorf("msg = %q, want hello", msg)
		}
		if target != tc.target {
			t.Errorf("target = %v, want %v", target, tc.target)
		}
	}
}
