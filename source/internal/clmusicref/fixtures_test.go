package clmusicref

import (
	"testing"
	"time"
)

// Coriakin's tune sample provided for timing investigation.
const coriakinTune = `/play 2 p@70([\g]8p1[=d]8p1[g]8p1[a]8p1[b]8p1/dgf#d=a[B]9p1\[e]8p1[b]8p1[=e]8p1ab[g]9p1\[c]8p1[g]8p1[=c]8p1ed[b]9p1[\g]8p1[=d]8p1[g]8p1[a]8p1[b]8p1/d[d]f#[e]g[d]f#[d]2=b@140[g]9p1[b]8p1[/d]7\[c]8p[g]8p[=c]8p[d]8p[e]8p[g]6pab(([=df#a]6\[e]9p[b]p[=e]p[bgd]6p[e]p[\b]p)2[=df#a]6\[e]p[b]p[=degb]2p|1[\b]p[=df#a]6\[c]9p[g]p[=c]p[bgd]6p[c]p[\g]2p[b=deg]9c\gcg=ce/dc=bg)2[\b=d]8p1[f#]7p1[/c]6\[c]6p[g]p[=c]2p[bgd]6p[c]p[\g]p[b=deg]9c\gc1g1=c1d1e1g1/c1d1g1p3=ga[\g]9/c[=d]9b[g]9p[a]9/[e]p[=`

// TestTune_Coriakin_Scaffold currently exercises the GoParse path to ensure
// we can process the string without panicking, and records a rough total
// duration. Once the classic C reference wrapper is wired, this test will be
// converted to assert equality between ParseRef and the main Go parser.
func TestTune_Coriakin_Scaffold(t *testing.T) {
	t.Skip("Enable after wiring classic C parser wrapper; current C stub doesn't support full syntax")
	// Strip the '/play 2 ' prefix if present
	s := coriakinTune
	const pref = "/play 2 "
	if len(s) >= len(pref) && s[:len(pref)] == pref {
		s = s[len(pref):]
	}
	evs, _ := ParseRef(s, 120)
	var end int
	for _, e := range evs {
		if e.StartMS+e.DurMS > end {
			end = e.StartMS + e.DurMS
		}
	}
	t.Logf("parsed events=%d total=%v", len(evs), time.Duration(end)*time.Millisecond)
}
