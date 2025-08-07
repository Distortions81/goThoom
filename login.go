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
)

// login connects to the server and performs the login handshake.
// It runs the network loops and blocks until the context is canceled.
func login(ctx context.Context, clientVersion int) {
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

		clientFull := encodeFullVersion(clientVersion)
		imagesOutdated := imagesVersion != clientFull
		soundsOutdated := soundsVersion != clientFull
		if imagesOutdated && !imagesMissing {
			log.Printf("warning: CL_Images version %d does not match client version %d", imagesVersion>>8, clientVersion)
		}
		if soundsOutdated && !soundsMissing {
			log.Printf("warning: CL_Sounds version %d does not match client version %d", soundsVersion>>8, clientVersion)
		}

		sendVersion := clientVersion
		if imagesMissing || soundsMissing || imagesOutdated || soundsOutdated {
			sendVersion = baseVersion - 1
		}

		var errDial error
		tcpConn, errDial = net.Dial("tcp", host)
		if errDial != nil {
			log.Fatalf("tcp connect: %v", errDial)
		}
		udpConn, err := net.Dial("udp", host)
		if err != nil {
			log.Fatalf("udp connect: %v", err)
		}

		var idBuf [4]byte
		if _, err := io.ReadFull(tcpConn, idBuf[:]); err != nil {
			log.Fatalf("read id: %v", err)
		}

		handshake := append([]byte{0xff, 0xff}, idBuf[:]...)
		if _, err := udpConn.Write(handshake); err != nil {
			log.Fatalf("send handshake: %v", err)
		}

		var confirm [2]byte
		if _, err := io.ReadFull(tcpConn, confirm[:]); err != nil {
			log.Fatalf("confirm handshake: %v", err)
		}
		if err := sendClientIdentifiers(tcpConn, encodeFullVersion(sendVersion), imagesVersion, soundsVersion); err != nil {
			log.Fatalf("send identifiers: %v", err)
		}
		fmt.Println("connected to", host)

		msg, err := readTCPMessage(tcpConn)
		if err != nil {
			log.Fatalf("read challenge: %v", err)
		}
		if len(msg) < 16 {
			log.Fatalf("short challenge message")
		}
		tag := binary.BigEndian.Uint16(msg[:2])
		const kMsgChallenge = 18
		if tag != kMsgChallenge {
			log.Fatalf("unexpected msg tag %d", tag)
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
				log.Fatalf("list characters: %v", err)
			}
			if len(names) == 0 {
				log.Fatalf("no characters available for account %v", acct)
			}
			if demo {
				name = names[rand.Intn(len(names))]
				fmt.Println("selected demo character:", name)
				pass = "demo"
			} else {
				selected := false
				if name != "" {
					for _, n := range names {
						if n == name {
							fmt.Println("selected character:", name)
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
						fmt.Println("selected character:", name)
					} else {
						fmt.Println("available characters:")
						for i, n := range names {
							fmt.Printf("%d) %v\n", i+1, n)
						}
						fmt.Print("select character: ")
						var choice int
						for {
							if _, err := fmt.Scanln(&choice); err != nil || choice < 1 || choice > len(names) {
								fmt.Printf("enter a number between 1 and %d: ", len(names))
								continue
							}
							break
						}
						name = names[choice-1]
						fmt.Println("selected character:", name)
					}
				}
			}
		}
		if pass == "" && passHash == "" && !demo {
			fmt.Print("enter character password: ")
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
				log.Fatalf("hash: %v", err)
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
				log.Fatalf("send login: %v", err)
			}

			resp, err = readTCPMessage(tcpConn)
			if err != nil {
				log.Fatalf("read login response: %v", err)
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
			log.Fatalf("unexpected response tag %d", resTag)
		}

		if result == -30972 || result == -30973 {
			fmt.Println("server requested update, downloading...")
			if err := autoUpdate(resp, dataDir); err != nil {
				log.Fatalf("auto update: %v", err)
			}
			fmt.Println("update complete, reconnecting...")
			tcpConn.Close()
			udpConn.Close()
			continue
		}

		if result == 0 {
			fmt.Println("login succeeded, reading messages (Ctrl-C to quit)...")

			if err := sendPlayerInput(udpConn); err != nil {
				logError("send player input: %v", err)
			}

			go sendInputLoop(ctx, udpConn)
			go udpReadLoop(ctx, udpConn)
			go tcpReadLoop(ctx, tcpConn)

			<-ctx.Done()
			tcpConn.Close()
			udpConn.Close()
		}
		break
	}
}
