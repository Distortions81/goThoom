package clmusicref

// RefEvent represents a parsed music event from the C reference parser.
// Keys holds MIDI note numbers for a chord; a rest has len(Keys)==0.
type RefEvent struct {
	StartMS int
	DurMS   int
	Keys    []int
}
