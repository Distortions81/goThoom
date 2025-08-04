package main

import "encoding/binary"

// dispatchMessage processes a raw server message. When hasHeader is true the
// packet is expected to include the standard 16 byte header. The dispatcher will
// invoke exactly one of the message handlers per packet.
func dispatchMessage(m []byte, hasHeader bool) {
	if len(m) == 0 {
		return
	}
	if hasHeader {
		if len(m) < 2 {
			return
		}
		tag := binary.BigEndian.Uint16(m[:2])
		if tag == 2 { // kMsgDrawState
			handleDrawState(m)
			return
		}
		if txt := decodeMessage(m); txt != "" {
			addMessage(txt)
		}
		return
	}
	handleInfoText(m)
}
