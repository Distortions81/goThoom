package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
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
	addMessage("Disconnected from server.")
	makeLoginWindow()
}

// login connects to the server and performs the login handshake.
// It runs the network loops and blocks until the context is canceled.
func login(ctx context.Context, clientVersion int) error {
	for {
		imagesVersion, err := readKeyFileVersion(filepath.Join(dataDir, "CL_Images"))
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

		soundsVersion, err := readKeyFileVersion(filepath.Join(dataDir, "CL_Sounds"))
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
			sendVersion = baseVersion - 1
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
		challenge := msg[16 : 16+16]

		if account != "" || demo {
			acct := account
			acctPass := accountPass
			if demo {
				acct = "demo"
				acctPass = "demo"
			}
			names, err := requestCharList(tcpConn, acct, acctPass, challenge, encodeFullVersion(sendVersion), imagesVersion, soundsVersion)
			if err != nil {
				tcpConn.Close()
				udpConn.Close()
				return fmt.Errorf("list characters: %w", err)
			}
			if len(names) == 0 {
				tcpConn.Close()
				udpConn.Close()
				return fmt.Errorf("no characters available for account %v", acct)
			}
			if demo {
				name = names[rand.Intn(len(names))]
				logDebug("selected demo character: %v", name)
				pass = "demo"
			} else {
				selected := false
				if name != "" {
					for _, n := range names {
						if n == name {
							logDebug("selected character: %v", name)
							selected = true
							break
						}
					}
					if !selected {
						logError("character %v not found in account %v", name, account)
					}
				}
				if !selected {
					if len(names) == 1 {
						name = names[0]
						logDebug("selected character: %v", name)
					} else {
						logDebug("available characters:")
						for i, n := range names {
							logDebug("%d) %v", i+1, n)
						}
						logDebug("select character: ")
						var choice int
						for {
							if _, err := fmt.Scanln(&choice); err != nil || choice < 1 || choice > len(names) {
								logDebug("enter a number between 1 and %d: ", len(names))
								continue
							}
							break
						}
						name = names[choice-1]
						logDebug("selected character: %v", name)
					}
				}
			}
		}
		if pass == "" && passHash == "" && !demo {
			logDebug("enter character password: ")
			fmt.Scanln(&pass)
		}
		playerName = name

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
			logDebug("server requested update, downloading...")
			if err := autoUpdate(resp, dataDir); err != nil {
				tcpConn.Close()
				udpConn.Close()
				return fmt.Errorf("auto update: %w", err)
			}
			logDebug("update complete, reconnecting...")
			tcpConn.Close()
			udpConn.Close()
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
		if err := sendPlayerInput(udpConn, s.mouseX, s.mouseY, s.mouseDown); err != nil {
			logError("send player input: %v", err)
		}

		go sendInputLoop(ctx, udpConn)
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
