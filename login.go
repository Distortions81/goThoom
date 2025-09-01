package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	loginCancel context.CancelFunc
	loginMu     sync.Mutex
)

func handleDisconnect() {
	loginMu.Lock()
	if loginCancel == nil {
		loginMu.Unlock()
		return
	}
	cancel := loginCancel
	loginCancel = nil
	loginMu.Unlock()

	cancel()
	if recorder != nil {
		stopRecording()
	}
	// Reset session sources so we return to splash state
	clmov = ""
	pcapPath = ""
	pass = ""
	if name != "" {
		for i := range characters {
			if characters[i].Name == name {
				if passHash == "" && (!characters[i].DontRemember || characters[i].passHash != "") {
					characters[i].passHash = ""
					characters[i].DontRemember = true
					characters[i].Key = ""
					saveCharacters()
				}
				break
			}
		}
	}
	consoleMessage("Disconnected from server.")
	loginWin.MarkOpen()
	updateCharacterButtons()
}

const CL_ImagesFile = "CL_Images"
const CL_SoundsFile = "CL_Sounds"

// fetchRandomDemoCharacter retrieves the server's demo characters and returns one at random.
func fetchRandomDemoCharacter(clientVersion int) (string, error) {
	imagesVersion, err := readKeyFileVersion(filepath.Join(dataDirPath, CL_ImagesFile))
	imagesMissing := false
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("CL_Images missing; will fetch from server")
			imagesVersion = 0
			imagesMissing = true
		} else {
			log.Printf("warning: %v", err)
			imagesVersion = encodeFullVersion(clientVersion)
		}
	}

	soundsVersion, err := readKeyFileVersion(filepath.Join(dataDirPath, CL_SoundsFile))
	soundsMissing := false
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("CL_Sounds missing; will fetch from server")
			soundsVersion = 0
			soundsMissing = true
		} else {
			log.Printf("warning: %v", err)
			soundsVersion = encodeFullVersion(clientVersion)
		}
	}

	sendVersion := int(imagesVersion >> 8)
	clientFull := encodeFullVersion(sendVersion)
	soundsOutdated := soundsVersion != clientFull
	if soundsOutdated && !soundsMissing {
		log.Printf("warning: CL_Sounds version %d does not match client version %d", soundsVersion>>8, sendVersion)
	}
	if imagesMissing || soundsMissing || soundsOutdated || sendVersion == 0 {
		sendVersion = clVersion - 1
	}

	tcpConn, err := net.Dial("tcp", host)
	if err != nil {
		return "", fmt.Errorf("tcp connect: %w", err)
	}
	defer tcpConn.Close()

	udpConn, err := net.Dial("udp", host)
	if err != nil {
		tcpConn.Close()
		return "", fmt.Errorf("udp connect: %w", err)
	}
	defer udpConn.Close()

	var idBuf [4]byte
	if _, err := io.ReadFull(tcpConn, idBuf[:]); err != nil {
		return "", fmt.Errorf("read id: %w", err)
	}
	handshake := append([]byte{0xff, 0xff}, idBuf[:]...)
	if _, err := udpConn.Write(handshake); err != nil {
		return "", fmt.Errorf("send handshake: %w", err)
	}
	var confirm [2]byte
	if _, err := io.ReadFull(tcpConn, confirm[:]); err != nil {
		return "", fmt.Errorf("confirm handshake: %w", err)
	}
	if err := sendClientIdentifiers(tcpConn, encodeFullVersion(sendVersion), imagesVersion, soundsVersion); err != nil {
		return "", fmt.Errorf("send identifiers: %w", err)
	}

	msg, err := readTCPMessage(tcpConn)
	if err != nil {
		return "", fmt.Errorf("read challenge: %w", err)
	}
	if len(msg) < 16 {
		return "", fmt.Errorf("short challenge message")
	}
	const kMsgChallenge = 18
	if binary.BigEndian.Uint16(msg[:2]) != kMsgChallenge {
		return "", fmt.Errorf("unexpected msg tag %d", binary.BigEndian.Uint16(msg[:2]))
	}
	// The server echoes its current Clan Lord version in the challenge
	// message. If we are newer than the server, fall back to the server's
	// version so we remain compatible with older servers.
	serverVersion := int(binary.BigEndian.Uint32(msg[4:8]) >> 8)
	if sendVersion > serverVersion {
		sendVersion = serverVersion
	}
	challenge := msg[16 : 16+16]

	answer, err := answerChallenge("demo", challenge)
	if err != nil {
		return "", fmt.Errorf("hash: %w", err)
	}
	const kMsgCharList = 14
	accountBytes := encodeMacRoman("demo")
	packet := make([]byte, 16+len(accountBytes)+1+len(answer))
	binary.BigEndian.PutUint16(packet[0:2], kMsgCharList)
	binary.BigEndian.PutUint16(packet[2:4], 0)
	binary.BigEndian.PutUint32(packet[4:8], encodeFullVersion(sendVersion))
	binary.BigEndian.PutUint32(packet[8:12], imagesVersion)
	binary.BigEndian.PutUint32(packet[12:16], soundsVersion)
	copy(packet[16:], accountBytes)
	packet[16+len(accountBytes)] = 0
	copy(packet[17+len(accountBytes):], answer)
	simpleEncrypt(packet[16:])
	if err := sendTCPMessage(tcpConn, packet); err != nil {
		return "", fmt.Errorf("send character list: %w", err)
	}

	resp, err := readTCPMessage(tcpConn)
	if err != nil {
		return "", fmt.Errorf("read character list: %w", err)
	}
	if len(resp) < 16 {
		return "", fmt.Errorf("short char list resp")
	}
	if binary.BigEndian.Uint16(resp[:2]) != kMsgCharList {
		return "", fmt.Errorf("unexpected tag %d", binary.BigEndian.Uint16(resp[:2]))
	}
	result := int16(binary.BigEndian.Uint16(resp[2:4]))
	simpleEncrypt(resp[16:])
	if result != 0 {
		msg := resp[16:]
		if i := bytes.IndexByte(msg, 0); i >= 0 {
			msg = msg[:i]
		}
		return "", fmt.Errorf("%s", decodeMacRoman(msg))
	}
	if len(resp) < 28 {
		return "", fmt.Errorf("short char list resp")
	}

	data := resp[16:]
	namesData := data[12:]
	var names []string
	for len(namesData) > 0 {
		i := bytes.IndexByte(namesData, 0)
		if i <= 0 {
			break
		}
		n := strings.TrimSpace(decodeMacRoman(namesData[:i]))
		if n != "" {
			names = append(names, n)
		}
		namesData = namesData[i+1:]
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no demo characters returned")
	}
	return names[rand.Intn(len(names))], nil
}

// login connects to the server and performs the login handshake.
// It runs the network loops and blocks until the context is canceled.
func login(ctx context.Context, clientVersion int) error {
	resetDrawState()
	for {
		imagesVersion, err := readKeyFileVersion(filepath.Join(dataDirPath, CL_ImagesFile))
		imagesMissing := false
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("CL_Images missing; will fetch from server")
				imagesVersion = 0
				imagesMissing = true
			} else {
				log.Printf("warning: %v", err)
				imagesVersion = encodeFullVersion(clientVersion)
			}
		}

		soundsVersion, err := readKeyFileVersion(filepath.Join(dataDirPath, CL_SoundsFile))
		soundsMissing := false
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("CL_Sounds missing; will fetch from server")
				soundsVersion = 0
				soundsMissing = true
			} else {
				log.Printf("warning: %v", err)
				soundsVersion = encodeFullVersion(clientVersion)
			}
		}

		sendVersion := int(imagesVersion >> 8)
		clientFull := encodeFullVersion(sendVersion)
		soundsOutdated := soundsVersion != clientFull
		if soundsOutdated && !soundsMissing {
			log.Printf("warning: CL_Sounds version %d does not match client version %d", soundsVersion>>8, sendVersion)
		}

		if imagesMissing || soundsMissing || soundsOutdated || sendVersion == 0 {
			sendVersion = clVersion - 1
		}

		var errDial error
		tcpConn, errDial = net.Dial("tcp", host)
		if errDial != nil {
			return fmt.Errorf("tcp connect: %w", errDial)
		}
		udpConn, err := net.Dial("udp", host)
		if err != nil {
			tcpConn.Close()
			return fmt.Errorf("udp connect: %w", err)
		}

		var idBuf [4]byte
		if _, err := io.ReadFull(tcpConn, idBuf[:]); err != nil {
			tcpConn.Close()
			udpConn.Close()
			return fmt.Errorf("read id: %w", err)
		}

		handshake := append([]byte{0xff, 0xff}, idBuf[:]...)
		if _, err := udpConn.Write(handshake); err != nil {
			tcpConn.Close()
			udpConn.Close()
			return fmt.Errorf("send handshake: %w", err)
		}

		var confirm [2]byte
		if _, err := io.ReadFull(tcpConn, confirm[:]); err != nil {
			tcpConn.Close()
			udpConn.Close()
			return fmt.Errorf("confirm handshake: %w", err)
		}
		if err := sendClientIdentifiers(tcpConn, encodeFullVersion(sendVersion), imagesVersion, soundsVersion); err != nil {
			tcpConn.Close()
			udpConn.Close()
			return fmt.Errorf("send identifiers: %w", err)
		}
		logDebug("connected to %v", host)

		msg, err := readTCPMessage(tcpConn)
		if err != nil {
			tcpConn.Close()
			udpConn.Close()
			return fmt.Errorf("read challenge: %w", err)
		}
		if len(msg) < 16 {
			tcpConn.Close()
			udpConn.Close()
			return fmt.Errorf("short challenge message")
		}
		tag := binary.BigEndian.Uint16(msg[:2])
		const kMsgChallenge = 18
		if tag != kMsgChallenge {
			tcpConn.Close()
			udpConn.Close()
			return fmt.Errorf("unexpected msg tag %d", tag)
		}
		// Obtain the server's client version from the challenge and, if
		// ours is newer, downgrade so we can connect to an older server.
		serverVersion := int(binary.BigEndian.Uint32(msg[4:8]) >> 8)
		if sendVersion > serverVersion {
			sendVersion = serverVersion
		}
		challenge := msg[16 : 16+16]

		if pass == "" && passHash == "" {
			tcpConn.Close()
			udpConn.Close()
			return fmt.Errorf("character password required")
		}
		playerName = utfFold(name)
		applyLocalLabels()
		applyEnabledPlugins()

		var resp []byte
		var result int16
		for {
			var answer []byte
			if pass != "" {
				answer, err = answerChallenge(pass, challenge)
			} else {
				answer, err = answerChallengeHash(passHash, challenge)
			}
			if err != nil {
				tcpConn.Close()
				udpConn.Close()
				return fmt.Errorf("hash: %w", err)
			}

			const kMsgLogOn = 13
			nameBytes := encodeMacRoman(name)
			buf := make([]byte, 16+len(nameBytes)+1+len(answer))
			binary.BigEndian.PutUint16(buf[0:2], kMsgLogOn)
			binary.BigEndian.PutUint16(buf[2:4], 0)
			binary.BigEndian.PutUint32(buf[4:8], encodeFullVersion(sendVersion))
			binary.BigEndian.PutUint32(buf[8:12], imagesVersion)
			binary.BigEndian.PutUint32(buf[12:16], soundsVersion)
			copy(buf[16:], nameBytes)
			buf[16+len(nameBytes)] = 0
			copy(buf[17+len(nameBytes):], answer)
			simpleEncrypt(buf[16:])

			if err := sendTCPMessage(tcpConn, buf); err != nil {
				tcpConn.Close()
				udpConn.Close()
				return fmt.Errorf("send login: %w", err)
			}

			resp, err = readTCPMessage(tcpConn)
			if err != nil {
				tcpConn.Close()
				udpConn.Close()
				return fmt.Errorf("read login response: %w", err)
			}
			resTag := binary.BigEndian.Uint16(resp[:2])
			const kMsgLogOnResp = 13
			if resTag == kMsgLogOnResp {
				result = int16(binary.BigEndian.Uint16(resp[2:4]))
				if name, ok := errorNames[result]; ok && result != 0 {
					logDebug("login result: %d (%v)", result, name)
				} else {
					logDebug("login result: %d", result)
				}
				break
			}
			if resTag == kMsgChallenge {
				challenge = resp[16 : 16+16]
				continue
			}
			tcpConn.Close()
			udpConn.Close()
			return fmt.Errorf("unexpected response tag %d", resTag)
		}

		if result == -30972 || result == -30973 {
			// Server indicates our client/data is out of date. Attempt an
			// in-place auto-update using the provided base URL if available.
			// Some servers omit version fields; autoUpdate handles this by
			// still attempting to fetch assets from the base path. Regardless
			// of outcome, retry the login once assets may have been updated.
			_, _ = autoUpdate(resp, dataDirPath)
			tcpConn.Close()
			udpConn.Close()
			// Retry the outer loop instead of failing or opening a browser.
			continue
		}

		if result != 0 {
			tcpConn.Close()
			udpConn.Close()
			if name, ok := errorNames[result]; ok {
				return fmt.Errorf("login failed: %s (%d)", name, result)
			}
			return fmt.Errorf("login failed: %d", result)
		}

		logDebug("login succeeded, reading messages (Ctrl-C to quit)...")

		inputMu.Lock()
		s := latestInput
		inputMu.Unlock()
		if err := sendPlayerInput(udpConn, s.mouseX, s.mouseY, s.mouseDown, false); err != nil {
			logError("send player input: %v", err)
		}

		go sendInputLoop(ctx, udpConn, tcpConn)
		go udpReadLoop(ctx, udpConn)
		go tcpReadLoop(ctx, tcpConn)

		<-ctx.Done()
		if tcpConn != nil {
			tcpConn.Close()
			tcpConn = nil
		}
		if udpConn != nil {
			udpConn.Close()
		}
		return nil
	}
}
