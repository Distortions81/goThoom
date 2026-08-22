package main

import "testing"

// Test that parseDrawState reports an error when inventory data is malformed.
func TestParseDrawStateBadInventory(t *testing.T) {
	resetTestState()
	data := buildDrawData("Bob", kBubbleNormal, "hi")
	// Replace inventory with an incomplete multiple command.
	data[len(data)-1] = byte(kInvCmdMultiple)
	if _, _, err := parseDrawState(data, false); err == nil {
		t.Fatalf("parseDrawState succeeded for malformed inventory")
	}
}
