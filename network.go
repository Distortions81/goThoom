package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

// tcpConn is the active TCP connection to the game server.
var tcpConn net.Conn

// sendClientIdentifiers transmits the client, image and sound versions to the server.
func sendClientIdentifiers(connection net.Conn, clientVersion, imagesVersion, soundsVersion uint32) error {
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

	data := make([]byte, 0, 8+6+len(unameBytes)+1+len(hnameBytes)+1+len(bootBytes)+1+1)
	data = append(data, make([]byte, 8)...) // magic file info placeholder
	data = append(data, make([]byte, 6)...) // ethernet address placeholder
	data = append(data, unameBytes...)
	data = append(data, 0)
	data = append(data, hnameBytes...)
	data = append(data, 0)
	data = append(data, bootBytes...)
	data = append(data, 0)
	data = append(data, byte(0)) // language

	buf := make([]byte, 16+len(data))
	binary.BigEndian.PutUint16(buf[0:2], kMsgIdentifiers)
	binary.BigEndian.PutUint16(buf[2:4], 0)
	binary.BigEndian.PutUint32(buf[4:8], clientVersion)
	binary.BigEndian.PutUint32(buf[8:12], imagesVersion)
	binary.BigEndian.PutUint32(buf[12:16], soundsVersion)
	copy(buf[16:], data)
	simpleEncrypt(buf[16:])
	logDebug("identifiers client=%d images=%d sounds=%d", clientVersion, imagesVersion, soundsVersion)
	return sendTCPMessage(connection, buf)
}

// sendTCPMessage writes a length-prefixed message to the TCP connection.
func sendTCPMessage(connection net.Conn, payload []byte) error {
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
	var size [2]byte
	binary.BigEndian.PutUint16(size[:], uint16(len(payload)))
	buf := append(size[:], payload...)
	if err := writeAll(connection, buf); err != nil {
		return err
	}
	tag := binary.BigEndian.Uint16(payload[:2])
	logDebug("send udp tag %d len %d", tag, len(payload))
	hexDump("send", payload)
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

// udpBuffer holds leftover datagram data between reads.
var udpBuffer []byte

// readUDPMessage reads a single length-prefixed message from the UDP
// connection. Packets may be fragmented across multiple datagrams or multiple
// messages may be present in a single datagram. Data is accumulated in
// udpBuffer until a full message is available.
func readUDPMessage(connection net.Conn) ([]byte, error) {
	for {
		if len(udpBuffer) >= 2 {
			sz := int(binary.BigEndian.Uint16(udpBuffer[:2]))
			if len(udpBuffer) >= 2+sz {
				msg := append([]byte(nil), udpBuffer[2:2+sz]...)
				udpBuffer = udpBuffer[2+sz:]
				tag := binary.BigEndian.Uint16(msg[:2])
				logDebug("recv udp tag %d len %d", tag, len(msg))
				hexDump("recv", msg)
				return msg, nil
			}
		}

		buf := make([]byte, 65535)
		n, err := connection.Read(buf)
		if err != nil {
			//logError("read udp: %v", err)
			return nil, err
		}
		if n == 0 {
			return nil, fmt.Errorf("short udp packet")
		}
		udpBuffer = append(udpBuffer, buf[:n]...)
	}
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

	nextCommand()
	// Before reading the pending command, give background queues
	// a chance to schedule maintenance commands.
	if pendingCommand == "" {
		if !maybeEnqueueInfo() {
			_ = maybeEnqueueWho()
		}
	}
	cmd := pendingCommand
	cmdBytes := encodeMacRoman(cmd)
	packet := make([]byte, 20+len(cmdBytes)+1)
	binary.BigEndian.PutUint16(packet[0:2], kMsgPlayerInput)
	binary.BigEndian.PutUint16(packet[2:4], uint16(mouseX))
	binary.BigEndian.PutUint16(packet[4:6], uint16(mouseY))
	binary.BigEndian.PutUint16(packet[6:8], flags)
	binary.BigEndian.PutUint32(packet[8:12], uint32(ackFrame))
	binary.BigEndian.PutUint32(packet[12:16], uint32(resendFrame))
	packetCommand := commandNum
	binary.BigEndian.PutUint32(packet[16:20], packetCommand)
	copy(packet[20:], cmdBytes)
	packet[20+len(cmdBytes)] = 0
	if cmd != "" {
		// Record last-command frame for who throttling.
		whoLastCommandFrame = ackFrame
		pendingCommand = ""
		nextCommand()
	}
	commandNum++
	logDebug("player input ack=%d resend=%d cmd=%d mouse=%d,%d flags=%#x", ackFrame, resendFrame, packetCommand, mouseX, mouseY, flags)
	latencyMu.Lock()
	lastInputSent = time.Now()
	latencyMu.Unlock()
	if reliable {
		return sendTCPMessage(connection, packet)
	}
	return sendUDPMessage(connection, packet)
}

// readTCPMessage reads a single length-prefixed message from the TCP connection.
func readTCPMessage(connection net.Conn) ([]byte, error) {
	var sizeBuf [2]byte
	if _, err := io.ReadFull(connection, sizeBuf[:]); err != nil {
		//logError("read tcp size: %v", err)
		return nil, err
	}
	sz := binary.BigEndian.Uint16(sizeBuf[:])
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
		noteFrame()
		// Advance plugin tick sleepers on each server frame
		pluginAdvanceTick()
		handleDrawState(msg, true)
		return
	}
	if txt := decodeMessage(msg); txt != "" {
		consoleMessage(txt)
	} else {
		logDebug("msg tag %d len %d", tag, len(msg))
	}
}
