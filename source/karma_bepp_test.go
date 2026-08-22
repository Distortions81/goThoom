package main

import "testing"

func TestDecodeKarmaBlockedIgnored(t *testing.T) {
	raw := bepp("kr", append(pnTag("Bob"), []byte(" gives you good karma")...))

	// Sanity check: unblocked message should be returned.
	players = make(map[string]*Player)
	if got := decodeBEPP(raw); got == "" {
		t.Fatalf("decodeBEPP returned empty for unblocked message")
	}

	cases := []struct {
		name string
		p    *Player
	}{
		{"blocked", &Player{Name: "Bob", Blocked: true}},
		{"ignored", &Player{Name: "Bob", Ignored: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			players = map[string]*Player{"Bob": tc.p}
			if got := decodeBEPP(raw); got != "" {
				t.Fatalf("decodeBEPP returned %q, want empty", got)
			}
		})
	}
}
