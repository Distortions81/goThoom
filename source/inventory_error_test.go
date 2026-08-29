package main

import (
	"encoding/binary"
	"testing"
)

// Optional state-record corruption must not reject an otherwise valid packed
// draw frame. A resend cannot repair a client-side logical decoder mismatch.
func TestParseDrawStateBadInventoryDoesNotRejectFrame(t *testing.T) {
	resetTestState()
	data := buildDrawData("Bob", kBubbleNormal, "hi")
	// Replace inventory with an incomplete multiple command.
	data[len(data)-1] = byte(kInvCmdMultiple)
	if _, _, err := parseDrawState(data, false); err != nil {
		t.Fatalf("parseDrawState rejected structurally valid frame: %v", err)
	}
}

func TestHandleDrawStateBadInventoryStillAdvancesAck(t *testing.T) {
	prepareLiveStateFragmentTest(t)
	resetCommandStateForTest(t, 5)
	pendingCommand = "/equip 123"
	pendingCommandID = 6
	pendingCommandSent = true

	body := buildDrawData("Bob", kBubbleNormal, "hi")
	body[0] = 6
	body[len(body)-1] = byte(kInvCmdMultiple)
	binary.BigEndian.PutUint32(body[1:5], 1)
	packet := make([]byte, len(body)+2)
	binary.BigEndian.PutUint16(packet[:2], 2)
	copy(packet[2:], body)

	if !handleDrawState(packet, false) {
		t.Fatal("handleDrawState rejected structurally valid frame")
	}
	if ackFrame != 1 || resendFrame != 0 {
		t.Fatalf("frame state = ack %d resend %d, want 1/0", ackFrame, resendFrame)
	}
	if pendingCommand != "" || pendingCommandID != 0 || pendingCommandSent {
		t.Fatalf("command was not acknowledged: %q id=%d sent=%v", pendingCommand, pendingCommandID, pendingCommandSent)
	}
}
