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

	scriptapi "gt2"
)

// tcpConn is the primary UI compatibility view of primarySession.transport.
// Network ownership and secondary-session connection checks use the transport
// object directly; this alias is removed when the remaining primary-only UI
// controls bind through the session manager.
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
	msg, _, err := readUDPMessageAt(connection)
	return msg, err
}

func readUDPMessageAt(connection net.Conn) ([]byte, time.Time, error) {
	readBuf := udpReadBufferPool.Get().([]byte)
	n, err := connection.Read(readBuf)
	if err != nil {
		udpReadBufferPool.Put(readBuf)
		return nil, time.Time{}, err
	}
	receivedAt := time.Now()
	defer func() {
		clear(readBuf[:n])
		udpReadBufferPool.Put(readBuf)
	}()
	if n < 2 {
		return nil, receivedAt, fmt.Errorf("%w: datagram has %d-byte header", errMalformedUDPDatagram, n)
	}
	sz := int(binary.BigEndian.Uint16(readBuf[:2]))
	if sz < minProtocolMessageSize || sz > maxProtocolMessageSize {
		return nil, receivedAt, fmt.Errorf("%w: payload size %d outside %d..%d", errMalformedUDPDatagram, sz, minProtocolMessageSize, maxProtocolMessageSize)
	}
	if sz != n-2 {
		return nil, receivedAt, fmt.Errorf("%w: declared payload %d, datagram payload %d", errMalformedUDPDatagram, sz, n-2)
	}
	msg := append([]byte(nil), readBuf[2:n]...)
	tag := binary.BigEndian.Uint16(msg[:2])
	logDebug("recv udp tag %d len %d", tag, len(msg))
	hexDump("recv", msg)
	return msg, receivedAt, nil
}

// sendPlayerInput sends the provided mouse state to the server. When
// reliable is true the packet is written to the TCP connection; otherwise
// it is sent via UDP.
func sendPlayerInput(connection net.Conn, mouseX, mouseY int16, mouseDown bool, reliable bool) error {
	return sendSessionPlayerInput(primarySession, connection, mouseX, mouseY, mouseDown, reliable)
}

func sendSessionPlayerInput(session *Session, connection net.Conn, mouseX, mouseY int16, mouseDown bool, reliable bool) error {
	if session == nil {
		return errors.New("player input session is nil")
	}
	const kMsgPlayerInput = 3
	flags := uint16(0)

	if mouseDown {
		flags = kPIMDownField
	}
	inputAck, inputResend := session.frames.snapshot()
	commands := session.commands

	commands.mu.Lock()
	commands.nextLocked()
	packetCommand := commands.number
	cmd := ""
	commandID := uint8(0)
	recordCommandTiming := false
	var sentTicket *scriptCommandState
	if commands.pending != "" {
		if commands.pendingID == 0 {
			commands.pendingID = commands.nextNumberLocked()
		}
		packetCommand = uint32(commands.pendingID)
		commandID = commands.pendingID
	}
	if commands.pending != "" && !commands.pendingSent {
		cmd = commands.pending
		sentTicket = commands.pendingTicket
		if sentTicket != nil {
			sentTicket.inFlight = true
		}
		// Record last-command frame for who throttling.
		commands.lastCommandFrame = inputAck
		commands.pendingSent = true
		recordCommandTiming = commands.pendingSentAt.IsZero()
	}
	commands.mu.Unlock()
	var cmdBytes []byte
	if cmd != "" {
		wireText := encodeEmojiShortcodes(cmd)
		cmdBytes = encodeMacRoman(wireText)
		if len(cmdBytes) > maxPlayerCommandBytes {
			logWarn("player command is %d bytes; truncating to classic %d-byte limit", len(cmdBytes), maxPlayerCommandBytes)
			cmdBytes = encodeMacRomanEscapedPrefix(wireText, maxPlayerCommandBytes)
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
	if recordCommandTiming {
		sentAt := time.Now()
		sentPhase, sentInterval, sentPredictively := session.pnaCommandTiming(sentAt)
		commands.mu.Lock()
		if commands.pendingID == commandID && commands.pending == cmd && commands.pendingSent && commands.pendingSentAt.IsZero() {
			commands.pendingSentAt = sentAt
			commands.pendingSentFrame = inputAck
			commands.pendingSentPhase = sentPhase
			commands.pendingSentInterval = sentInterval
			commands.pendingSentPredictively = sentPredictively
		}
		commands.mu.Unlock()
	}
	var err error
	if reliable {
		err = sendTCPMessage(connection, packet)
	} else {
		err = sendUDPMessage(connection, packet)
	}
	if err != nil && cmd != "" {
		commands.mu.Lock()
		if sentTicket != nil {
			sentTicket.inFlight = false
		}
		if commands.pendingID == commandID && commands.pending == cmd && commands.pendingTicket == sentTicket {
			commands.pendingSent = false
			commands.resetPendingTimingLocked()
			if sentTicket != nil && sentTicket.cancelRequested {
				commands.cancelTicketLocked(sentTicket)
			}
		}
		commands.mu.Unlock()
	}
	if err == nil && sentTicket != nil {
		commands.mu.Lock()
		sentTicket.inFlight = false
		sentTicket.status = scriptapi.CommandStatus{State: scriptapi.CommandSent}
		commands.mu.Unlock()
	}
	return err
}

// readTCPMessage reads a single length-prefixed message from the TCP connection.
func readTCPMessage(connection net.Conn) ([]byte, error) {
	msg, _, err := readTCPMessageAt(connection)
	return msg, err
}

func readTCPMessageAt(connection net.Conn) ([]byte, time.Time, error) {
	var sizeBuf [2]byte
	if _, err := io.ReadFull(connection, sizeBuf[:]); err != nil {
		//logError("read tcp size: %v", err)
		return nil, time.Time{}, err
	}
	sz := int(binary.BigEndian.Uint16(sizeBuf[:]))
	if sz < minProtocolMessageSize || sz > maxProtocolMessageSize {
		return nil, time.Time{}, fmt.Errorf("TCP message size %d outside %d..%d", sz, minProtocolMessageSize, maxProtocolMessageSize)
	}
	buf := make([]byte, sz)
	if _, err := io.ReadFull(connection, buf); err != nil {
		return nil, time.Time{}, err
	}
	receivedAt := time.Now()
	tag := binary.BigEndian.Uint16(buf[:2])
	logDebug("recv tcp tag %d len %d", tag, len(buf))
	hexDump("recv", buf)
	return buf, receivedAt, nil
}

// processServerMessage handles messages without a captured socket-arrival
// timestamp, including playback and tests.
func processServerMessage(msg []byte) {
	processSessionServerMessageAt(primarySession, msg, time.Now())
}

// processServerMessageAt keeps live draw-state phase measurements anchored to
// socket arrival rather than to the end of client-side decoding and cache work.
func processServerMessageAt(msg []byte, receivedAt time.Time) {
	processSessionServerMessageAt(primarySession, msg, receivedAt)
}

func processSessionServerMessageAt(session *Session, msg []byte, receivedAt time.Time) {
	if session == nil {
		return
	}
	if len(msg) < 2 {
		return
	}
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	tag := binary.BigEndian.Uint16(msg[:2])
	if tag == 2 {
		if handleSessionDrawStateAt(session, msg, true, receivedAt) {
			session.noteFrameAt(session.frames.acknowledged(), receivedAt)
		}
		return
	}
	if txt := decodeSessionMessage(session, msg); txt != "" {
		if session == primarySession {
			session.publishEvent(sessionEvent{Kind: sessionEventConsole, Text: txt, MessageType: messageTextTypeSystem})
			consoleMessage(txt)
		} else {
			session.publishConsole(txt, messageTextTypeSystem)
		}
	} else {
		logDebug("msg tag %d len %d", tag, len(msg))
	}
}
