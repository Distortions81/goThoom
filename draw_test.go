package main

import "testing"

func TestHandleDrawStateInfoStrings(t *testing.T) {
	messages = nil
	state = drawState{}
	// sample text snippets from test.clMov
	msg1 := "You sense healing energy from Harper."
	msg2 := "a fur, worth 37c. Your share is 3c."

	stateData := append([]byte(msg1), 0)
	stateData = append(stateData, []byte(msg2)...)
	stateData = append(stateData, 0) // terminator before bubble count
	stateData = append(stateData, 0) // bubble count 0

	data := make([]byte, 0, 19+len(stateData))
	data = append(data, 0)                  // ackCmd
	data = append(data, make([]byte, 8)...) // ackFrame + resendFrame
	data = append(data, 0)                  // descriptor count
	data = append(data, make([]byte, 7)...) // hp, hpMax, sp, spMax, bal, balMax, lighting
	data = append(data, 0)                  // picture count
	data = append(data, 0)                  // mobile count
	data = append(data, stateData...)

	m := append([]byte{0, 0}, data...)
	handleDrawState(m)

	got := getMessages()
	if len(got) != 2 || got[0] != msg1 || got[1] != msg2 {
		t.Fatalf("messages = %#v", got)
	}
}

func TestHandleDrawStateEncryptedInfoStrings(t *testing.T) {
	messages = nil
	state = drawState{}
	msg1 := "You sense healing energy from Harper."
	msg2 := "a fur, worth 37c. Your share is 3c."

	stateData := append([]byte(msg1), 0)
	stateData = append(stateData, []byte(msg2)...)
	stateData = append(stateData, 0)
	stateData = append(stateData, 0)

	data := make([]byte, 0, 19+len(stateData))
	data = append(data, 0)
	data = append(data, make([]byte, 8)...)
	data = append(data, 0)
	data = append(data, make([]byte, 7)...)
	data = append(data, 0)
	data = append(data, 0)
	data = append(data, stateData...)

	m := append([]byte{0, 0}, data...)
	simpleEncrypt(m[2:])
	handleDrawState(m)

	got := getMessages()
	if len(got) != 2 || got[0] != msg1 || got[1] != msg2 {
		t.Fatalf("messages = %#v", got)
	}
}

func TestHandleDrawStateUsesDescriptorName(t *testing.T) {
	messages = nil
	state = drawState{}
	playerName = "SomeoneElse"
	playerIndex = 0xff

	desc := []byte{0, 0, 0, 0}
	desc = append(desc, []byte("Tsune")...)
	desc = append(desc, 0) // name terminator
	desc = append(desc, 0) // color count

	msg := "Bashak has fallen to a Shadowcat Huntress."
	bubble := []byte{0, byte(kBubbleYell)}
	bubble = append(bubble, []byte(msg)...)
	bubble = append(bubble, 0)

	stateData := []byte{0}           // end of info strings
	stateData = append(stateData, 1) // bubble count
	stateData = append(stateData, bubble...)

	data := make([]byte, 0, 19+len(stateData)+len(desc))
	data = append(data, 0)                  // ackCmd
	data = append(data, make([]byte, 8)...) // ackFrame + resendFrame
	data = append(data, 1)                  // descriptor count
	data = append(data, desc...)
	data = append(data, make([]byte, 7)...) // hp, sp, etc.
	data = append(data, 0)                  // picture count
	data = append(data, 0)                  // mobile count
	data = append(data, stateData...)

	m := append([]byte{0, 0}, data...)
	handleDrawState(m)

	expected := "Tsune yells, " + msg
	got := getMessages()
	if len(got) != 1 || got[0] != expected {
		t.Fatalf("messages = %#v", got)
	}
}

func TestHandleDrawStateSounds(t *testing.T) {
	messages = nil
	state = drawState{}
	var played []uint16
	origPlaySound := playSound
	playSound = func(id uint16) { played = append(played, id) }
	defer func() { playSound = origPlaySound }()

	stateData := []byte{0}           // end of info strings
	stateData = append(stateData, 0) // bubble count
	stateData = append(stateData, 2) // sound count
	stateData = append(stateData, 0x00, 0x01)
	stateData = append(stateData, 0x02, 0x03)

	data := make([]byte, 0, 19+len(stateData))
	data = append(data, 0)                  // ackCmd
	data = append(data, make([]byte, 8)...) // ackFrame + resendFrame
	data = append(data, 0)                  // descriptor count
	data = append(data, make([]byte, 7)...) // hp, sp, etc.
	data = append(data, 0)                  // picture count
	data = append(data, 0)                  // mobile count
	data = append(data, stateData...)

	m := append([]byte{0, 0}, data...)
	handleDrawState(m)

	if len(played) != 2 || played[0] != 1 || played[1] != 515 {
		t.Fatalf("played = %#v", played)
	}
}
