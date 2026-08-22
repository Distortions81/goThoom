package clmusicref

import "testing"

func TestScaffold_RefVsGoParse_Basics(t *testing.T) {
	cases := []string{
		"c p c",
		"c_c",
		"C @+60 c",
		"@60 c @ c",
		"c1 p1 C1",
	}
	for _, tt := range cases {
		ref, err := ParseRef(tt, 120)
		if err != nil {
			t.Fatalf("ParseRef error for %q: %v", tt, err)
		}
		goev := parseRefInternal(tt, 120)
		if len(ref) != len(goev) {
			t.Fatalf("len mismatch for %q: ref=%d go=%d", tt, len(ref), len(goev))
		}
		for i := range ref {
			if ref[i].StartMS != goev[i].StartMS || ref[i].DurMS != goev[i].DurMS {
				t.Fatalf("event[%d] mismatch for %q: ref=%+v go=%+v", i, tt, ref[i], goev[i])
			}
		}
	}
}
