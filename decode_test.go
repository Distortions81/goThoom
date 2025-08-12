package main

import "testing"

func resetConsole() {
	messageMu.Lock()
	messages = nil
	messageMu.Unlock()
}

func makeBEPP(prefix, text string) []byte {
	b := []byte{0xC2, prefix[0], prefix[1]}
	b = append(b, []byte(text)...)
	return b
}

func TestDecodeBEPPPrefixes(t *testing.T) {
	cases := []struct {
		prefix string
		text   string
		want   string
		log    string
	}{
		{"ba", "bard", "bard: bard", ""},
		{"be", "payload", "", ""},
		{"cn", "clan", "clan", ""},
		{"cf", "cfg", "", "config: cfg"},
		{"dd", "hidden", "", ""},
		{"de", "demo", "demo: demo", ""},
		{"dp", "depart", "depart: depart", ""},
		{"dl", "file", "", "download: file"},
		{"er", "oops", "error: oops", ""},
		{"gm", "msg", "gm: msg", ""},
		{"hf", "fallen text", "fallen: fallen text", ""},
		{"nf", "recover", "no longer fallen: recover", ""},
		{"hp", "help me", "help: help me", ""},
		{"in", "info", "info: info", ""},
		{"iv", "inventory item", "inventory: inventory item", ""},
		{"ka", "karma", "karma: karma", ""},
		{"kr", "krec", "karma received: krec", ""},
		{"lf", "bye", "logoff: bye", ""},
		{"lg", "hi", "logon: hi", ""},
		{"lo", "here", "location: here", ""},
		{"mn", "monster", "monster", ""},
		{"ml", "multi", "multi", ""},
		{"mu", "song", "", "music: song"},
		{"nw", "news", "news: news", ""},
		{"pn", "player", "player", ""},
		{"sh", "share text", "share: share text", ""},
		{"su", "unshare text", "unshare: unshare text", ""},
		{"tl", "log only", "", "log only"},
		{"th", "thought", "think: thought", ""},
		{"tt", "mono", "mono", ""},
		{"wh", "who text", "who: who text", ""},
		{"yk", "you killed", "you killed", ""},
	}

	for _, tc := range cases {
		resetConsole()
		data := makeBEPP(tc.prefix, tc.text)
		got := decodeBEPP(data)
		if got != tc.want {
			t.Errorf("prefix %s got %q want %q", tc.prefix, got, tc.want)
		}
		msgs := getConsoleMessages()
		last := ""
		if len(msgs) > 0 {
			last = msgs[len(msgs)-1]
		}
		if last != tc.log {
			t.Errorf("prefix %s log %q want %q", tc.prefix, last, tc.log)
		}
	}
}
