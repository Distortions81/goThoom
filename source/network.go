package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

// tcpConn is the active TCP connection to the game server.
var tcpConn net.Conn

// messageBufferSize is large enough to hold the most common payloads such as
// identifiers and player input packets.
const messageBufferSize = 512

const (
	minProtocolMessageSize = 2
	maxProtocolMessageSize = 1396
	udpReadBufferSize      = 2 + maxProtocolMessageSize + 1
	maxPlayerCommandBytes  = 511
)

var errMalformedUDPDatagram = errors.New("malformed UDP datagram")

var messageBufferPool = sync.Pool{
	New: func() any {
		return make([]byte, messageBufferSize)
	},
}

var udpReadBufferPool = sync.Pool{
	New: func() any {
		return make([]byte, udpReadBufferSize)
	},
}

func getMessageBuffer() []byte {
	if buf := messageBufferPool.Get(); buf != nil {
		b := buf.([]byte)
		if cap(b) >= messageBufferSize {
			return b[:messageBufferSize]
		}
	}
	return make([]byte, messageBufferSize)
}

func putMessageBuffer(buf []byte) {
	if cap(buf) < messageBufferSize {
		return
	}
	messageBufferPool.Put(buf[:messageBufferSize])
}

// sendClientIdentifiers transmits the client, image and sound versions to the server.
func sendClientIdentifiers(connection net.Conn, clVersion, imagesVersion, soundsVersion uint32) error {
	const kMsgIdentifiers = 19
	uname := os.Getenv("USER")
	if uname == "" {
		uname = "unknown"
	}
	hname, _ := os.Hostname()
	if hname == "" {
		hname = "unknown"
	}
	boot := "/"

	unameBytes := encodeMacRoman(uname)
	hnameBytes := encodeMacRoman(hname)
	bootBytes := encodeMacRoman(boot)

	payloadLen := 16 + 8 + 6 + len(unameBytes) + 1 + len(hnameBytes) + 1 + len(bootBytes) + 1 + 1
	usePool := payloadLen <= messageBufferSize
	var baseBuf []byte
	if usePool {
		baseBuf = getMessageBuffer()
	} else {
		baseBuf = make([]byte, payloadLen)
	}
	packet := baseBuf[:payloadLen]
	defer func() {
		clear(packet)
		if usePool {
			putMessageBuffer(baseBuf)
		}
	}()

	binary.BigEndian.PutUint16(packet[0:2], kMsgIdentifiers)
	binary.BigEndian.PutUint16(packet[2:4], 0)
	binary.BigEndian.PutUint32(packet[4:8], clVersion)
	binary.BigEndian.PutUint32(packet[8:12], imagesVersion)
	binary.BigEndian.PutUint32(packet[12:16], soundsVersion)
	offset := 16
	for i := 0; i < 14; i++ { // magic file info (8) + ethernet address (6)
		packet[offset+i] = 0
	}
	offset += 14
	copy(packet[offset:], unameBytes)
	offset += len(unameBytes)
	packet[offset] = 0
	offset++
	copy(packet[offset:], hnameBytes)
	offset += len(hnameBytes)
	packet[offset] = 0
	offset++
	copy(packet[offset:], bootBytes)
	offset += len(bootBytes)
	packet[offset] = 0
	offset++
	packet[offset] = byte(0) // language

	simpleEncrypt(packet[16:])
	logDebug("identifiers client=%d images=%d sounds=%d", clVersion, imagesVersion, soundsVersion)
	return sendTCPMessage(connection, packet)
}

// sendTCPMessage writes a length-prefixed message to the TCP connection.
func sendTCPMessage(connection net.Conn, payload []byte) error {
	if err := validateProtocolPayload(payload); err != nil {
		return err
	}
	var size [2]byte
	binary.BigEndian.PutUint16(size[:], uint16(len(payload)))
	if err := writeAll(connection, size[:]); err != nil {
		return err
	}
	if err := writeAll(connection, payload); err != nil {
		return err
	}
	tag := binary.BigEndian.Uint16(payload[:2])
	logDebug("send tcp tag %d len %d", tag, len(payload))
	hexDump("send", payload)
	return nil
}

// sendUDPMessage writes a length-prefixed message to the UDP connection.
func sendUDPMessage(connection net.Conn, payload []byte) error {
	if err := validateProtocolPayload(payload); err != nil {
		return err
	}
	var size [2]byte
	binary.BigEndian.PutUint16(size[:], uint16(len(payload)))
	totalLen := 2 + len(payload)
	usePool := totalLen <= messageBufferSize
	var baseBuf []byte
	if usePool {
		baseBuf = getMessageBuffer()
	} else {
		baseBuf = make([]byte, totalLen)
	}
	frame := baseBuf[:totalLen]
	defer func() {
		clear(frame)
		if usePool {
			putMessageBuffer(baseBuf)
		}
	}()
	frame[0] = size[0]
	frame[1] = size[1]
	copy(frame[2:], payload)
	n, err := connection.Write(frame)
	if err != nil {
		return err
	}
	if n != len(frame) {
		return io.ErrShortWrite
	}
	tag := binary.BigEndian.Uint16(payload[:2])
	logDebug("send udp tag %d len %d", tag, len(payload))
	hexDump("send", payload)
	return nil
}

func validateProtocolPayload(payload []byte) error {
	if len(payload) < minProtocolMessageSize || len(payload) > maxProtocolMessageSize {
		return fmt.Errorf("protocol message size %d outside %d..%d", len(payload), minProtocolMessageSize, maxProtocolMessageSize)
	}
	return nil
}

// writeAll writes the entirety of data to conn, returning an error if the
// write fails or is short.
func writeAll(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

// readUDPMessage reads a single length-prefixed message from the UDP
// connection. Datagram boundaries are protocol boundaries: the declared
// payload length must exactly match the remainder of this one datagram.
func readUDPMessage(connection net.Conn) ([]byte, error) {
	readBuf := udpReadBufferPool.Get().([]byte)
	n, err := connection.Read(readBuf)
	if err != nil {
		udpReadBufferPool.Put(readBuf)
		return nil, err
	}
	defer func() {
		clear(readBuf[:n])
		udpReadBufferPool.Put(readBuf)
	}()
	if n < 2 {
		return nil, fmt.Errorf("%w: datagram has %d-byte header", errMalformedUDPDatagram, n)
	}
	sz := int(binary.BigEndian.Uint16(readBuf[:2]))
	if sz < minProtocolMessageSize || sz > maxProtocolMessageSize {
		return nil, fmt.Errorf("%w: payload size %d outside %d..%d", errMalformedUDPDatagram, sz, minProtocolMessageSize, maxProtocolMessageSize)
	}
	if sz != n-2 {
		return nil, fmt.Errorf("%w: declared payload %d, datagram payload %d", errMalformedUDPDatagram, sz, n-2)
	}
	msg := append([]byte(nil), readBuf[2:n]...)
	tag := binary.BigEndian.Uint16(msg[:2])
	logDebug("recv udp tag %d len %d", tag, len(msg))
	hexDump("recv", msg)
	return msg, nil
}

// sendPlayerInput sends the provided mouse state to the server. When
// reliable is true the packet is written to the TCP connection; otherwise
// it is sent via UDP.
func sendPlayerInput(connection net.Conn, mouseX, mouseY int16, mouseDown bool, reliable bool) error {
	const kMsgPlayerInput = 3
	flags := uint16(0)

	if mouseDown {
		flags = kPIMDownField
	}
	inputAck, inputResend := networkFrameStateSnapshot()

	commandMu.Lock()
	nextCommandLocked()
	packetCommand := commandNum
	cmd := ""
	commandID := uint8(0)
	if pendingCommand != "" {
		if pendingCommandID == 0 {
			pendingCommandID = nextCommandNumberLocked()
		}
		packetCommand = uint32(pendingCommandID)
		commandID = pendingCommandID
	}
	if pendingCommand != "" && !pendingCommandSent {
		cmd = pendingCommand
		// Record last-command frame for who throttling.
		whoLastCommandFrame = inputAck
		pendingCommandSent = true
		if pendingCommandSentAt.IsZero() {
			pendingCommandSentAt = time.Now()
		}
	}
	commandMu.Unlock()
	var cmdBytes []byte
	if cmd != "" {
		cmdBytes = encodeMacRoman(cmd)
		if len(cmdBytes) > maxPlayerCommandBytes {
			logWarn("player command is %d bytes; truncating to classic %d-byte limit", len(cmdBytes), maxPlayerCommandBytes)
			cmdBytes = cmdBytes[:maxPlayerCommandBytes]
		}
	}
	packetLen := 20 + len(cmdBytes) + 1
	usePool := packetLen <= messageBufferSize
	var baseBuf []byte
	if usePool {
		baseBuf = getMessageBuffer()
	} else {
		baseBuf = make([]byte, packetLen)
	}
	packet := baseBuf[:packetLen]
	defer func() {
		clear(packet)
		if usePool {
			putMessageBuffer(baseBuf)
		}
	}()
	binary.BigEndian.PutUint16(packet[0:2], kMsgPlayerInput)
	binary.BigEndian.PutUint16(packet[2:4], uint16(mouseX))
	binary.BigEndian.PutUint16(packet[4:6], uint16(mouseY))
	binary.BigEndian.PutUint16(packet[6:8], flags)
	binary.BigEndian.PutUint32(packet[8:12], uint32(inputAck))
	binary.BigEndian.PutUint32(packet[12:16], uint32(inputResend))
	binary.BigEndian.PutUint32(packet[16:20], packetCommand)
	copy(packet[20:], cmdBytes)
	packet[20+len(cmdBytes)] = 0
	logDebug("player input ack=%d resend=%d cmd=%d mouse=%d,%d flags=%#x", inputAck, inputResend, packetCommand, mouseX, mouseY, flags)
	var err error
	if reliable {
		err = sendTCPMessage(connection, packet)
	} else {
		err = sendUDPMessage(connection, packet)
	}
	if err != nil && cmd != "" {
		commandMu.Lock()
		if pendingCommandID == commandID && pendingCommand == cmd {
			pendingCommandSent = false
			pendingCommandSentAt = time.Time{}
		}
		commandMu.Unlock()
	}
	return err
}

// readTCPMessage reads a single length-prefixed message from the TCP connection.
func readTCPMessage(connection net.Conn) ([]byte, error) {
	var sizeBuf [2]byte
	if _, err := io.ReadFull(connection, sizeBuf[:]); err != nil {
		//logError("read tcp size: %v", err)
		return nil, err
	}
	sz := int(binary.BigEndian.Uint16(sizeBuf[:]))
	if sz < minProtocolMessageSize || sz > maxProtocolMessageSize {
		return nil, fmt.Errorf("TCP message size %d outside %d..%d", sz, minProtocolMessageSize, maxProtocolMessageSize)
	}
	buf := make([]byte, sz)
	if _, err := io.ReadFull(connection, buf); err != nil {
		return nil, err
	}
	tag := binary.BigEndian.Uint16(buf[:2])
	logDebug("recv tcp tag %d len %d", tag, len(buf))
	hexDump("recv", buf)
	return buf, nil
}

// processServerMessage handles a raw server message by inspecting its tag and
// routing it appropriately. Draw state messages (tag 2) are forwarded to
// handleDrawState after noting a frame. All other messages are decoded and any
// resulting text is logged to the in-game console.
func processServerMessage(msg []byte) {
	if len(msg) < 2 {
		return
	}
	tag := binary.BigEndian.Uint16(msg[:2])
	if tag == 2 {
		if handleDrawState(msg, true) {
			noteFrame()
		}
		return
	}
	if txt := decodeMessage(msg); txt != "" {
		consoleMessage(txt)
	} else {
		logDebug("msg tag %d len %d", tag, len(msg))
	}
}
