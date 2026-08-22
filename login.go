package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	loginCancel context.CancelFunc
	loginMu     sync.Mutex
)

type serverTarget struct {
	addr     string
	display  string
	fallback bool
}

var errRetryLogin = errors.New("retry login")

func serverTargets(addr string) []serverTarget {
	primary := serverTarget{addr: addr, display: addr}
	fallbackAddr, ok := fallbackAddress(addr)
	if !ok {
		return []serverTarget{primary}
	}

	fallback := serverTarget{
		addr:     fallbackAddr,
		display:  fmt.Sprintf("%s (fallback)", fallbackAddr),
		fallback: true,
	}

	if preferIPFallback {
		return []serverTarget{fallback}
	}
	return []serverTarget{primary, fallback}
}

const connectAttemptTimeout = 15 * time.Second

func dialServer(network string, target serverTarget) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: connectAttemptTimeout}
	conn, err := dialer.Dial(network, target.addr)
	if err != nil {
		recordFallbackFailure(target, err)
		return nil, err
	}
	return conn, nil
}

var (
	preferIPFallback         bool
	preferIPFallbackDueToDNS bool
)

func recordFallbackFailure(target serverTarget, err error) {
	if err == nil || target.fallback || errors.Is(err, errRetryLogin) {
		return
	}
	if shouldPreferFallback(err) {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) {
			preferIPFallbackDueToDNS = true
		}
		preferIPFallback = true
	}
}

func shouldPreferFallback(err error) bool {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded)
}

func connectStatusMessage(target serverTarget) string {
	base := fmt.Sprintf("Connecting to %s...", target.display)
	if !target.fallback {
		return base
	}
	if preferIPFallbackDueToDNS {
		return fmt.Sprintf("%s DNS lookup failed; using fallback IP.", base)
	}
	return fmt.Sprintf("%s Using fallback IP.", base)
}

func retryConnectStatusMessage(current, next serverTarget, err error) string {
	base := fmt.Sprintf("Unable to reach %s (%v);", current.display, err)
	if next.fallback {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) || preferIPFallbackDueToDNS {
			return fmt.Sprintf("%s DNS lookup failed; trying fallback IP %s...", base, next.display)
		}
		return fmt.Sprintf("%s trying fallback %s...", base, next.display)
	}
	return fmt.Sprintf("%s trying %s...", base, next.display)
}

func fallbackAddress(addr string) (string, bool) {
	hostName, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", false
	}
	if !strings.EqualFold(hostName, defaultServerHostName) {
		return "", false
	}
	return net.JoinHostPort(fallbackServerIP, port), true
}

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
	// Reset frame/loss counters so a new session starts fresh.
	lastAckFrame = 0
	numFrames = 0
	lostFrames = 0
	for i := range frameBuckets {
		frameBuckets[i] = 0
	}
	for i := range lostBuckets {
		lostBuckets[i] = 0
	}
	for i := range bucketTimes {
		bucketTimes[i] = 0
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
func fetchRandomDemoCharacter(clVersion int) (string, error) {
	imagesVersion, err := readKeyFileVersion(filepath.Join(dataDirPath, CL_ImagesFile))
	imagesMissing := false
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("CL_Images missing; will fetch from server")
			imagesVersion = 0
			imagesMissing = true
		} else {
			log.Printf("warning: %v", err)
			imagesVersion = encodeFullVersion(clVersion)
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
			soundsVersion = encodeFullVersion(clVersion)
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

	targets := serverTargets(host)
	var lastErr error
	for i, target := range targets {
		name, err := fetchDemoFromTarget(target, sendVersion, imagesVersion, soundsVersion)
		if err == nil {
			return name, nil
		}
		lastErr = err
		if i < len(targets)-1 {
			next := targets[i+1]
			logWarn("demo login via %s failed (%v); trying %s", target.display, err, next.display)
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no server targets available")
	}
	return "", lastErr
}

func fetchDemoFromTarget(target serverTarget, sendVersion int, imagesVersion, soundsVersion uint32) (string, error) {
	tcpConn, err := dialServer("tcp", target)
	if err != nil {
		return "", fmt.Errorf("tcp connect %s: %w", target.addr, err)
	}
	defer tcpConn.Close()

	udpConn, err := dialServer("udp", target)
	if err != nil {
		return "", fmt.Errorf("udp connect %s: %w", target.addr, err)
	}
	defer udpConn.Close()

	var idBuf [4]byte
	if _, err := io.ReadFull(tcpConn, idBuf[:]); err != nil {
		return "", fmt.Errorf("read id via %s: %w", target.addr, err)
	}
	handshake := append([]byte{0xff, 0xff}, idBuf[:]...)
	if _, err := udpConn.Write(handshake); err != nil {
		return "", fmt.Errorf("send handshake via %s: %w", target.addr, err)
	}
	var confirm [2]byte
	if _, err := io.ReadFull(tcpConn, confirm[:]); err != nil {
		return "", fmt.Errorf("confirm handshake via %s: %w", target.addr, err)
	}
	if err := sendClientIdentifiers(tcpConn, encodeFullVersion(sendVersion), imagesVersion, soundsVersion); err != nil {
		return "", fmt.Errorf("send identifiers via %s: %w", target.addr, err)
	}

	msg, err := readTCPMessage(tcpConn)
	if err != nil {
		return "", fmt.Errorf("read challenge via %s: %w", target.addr, err)
	}
	if len(msg) < 16 {
		return "", fmt.Errorf("short challenge message via %s", target.addr)
	}
	const kMsgChallenge = 18
	if binary.BigEndian.Uint16(msg[:2]) != kMsgChallenge {
		return "", fmt.Errorf("unexpected msg tag %d via %s", binary.BigEndian.Uint16(msg[:2]), target.addr)
	}
	serverVersion := int(binary.BigEndian.Uint32(msg[4:8]) >> 8)
	sendVersionLocal := sendVersion
	if sendVersionLocal > serverVersion {
		sendVersionLocal = serverVersion
	}
	challenge := msg[16 : 16+16]

	answer, err := answerChallenge("demo", challenge)
	if err != nil {
		return "", fmt.Errorf("hash via %s: %w", target.addr, err)
	}
	const kMsgCharList = 14
	accountBytes := encodeMacRoman("demo")
	packet := make([]byte, 16+len(accountBytes)+1+len(answer))
	binary.BigEndian.PutUint16(packet[0:2], kMsgCharList)
	binary.BigEndian.PutUint16(packet[2:4], 0)
	binary.BigEndian.PutUint32(packet[4:8], encodeFullVersion(sendVersionLocal))
	binary.BigEndian.PutUint32(packet[8:12], imagesVersion)
	binary.BigEndian.PutUint32(packet[12:16], soundsVersion)
	copy(packet[16:], accountBytes)
	packet[16+len(accountBytes)] = 0
	copy(packet[17+len(accountBytes):], answer)
	simpleEncrypt(packet[16:])
	if err := sendTCPMessage(tcpConn, packet); err != nil {
		return "", fmt.Errorf("send character list via %s: %w", target.addr, err)
	}

	resp, err := readTCPMessage(tcpConn)
	if err != nil {
		return "", fmt.Errorf("read character list via %s: %w", target.addr, err)
	}
	if len(resp) < 16 {
		return "", fmt.Errorf("short char list resp via %s", target.addr)
	}
	if binary.BigEndian.Uint16(resp[:2]) != kMsgCharList {
		return "", fmt.Errorf("unexpected tag %d via %s", binary.BigEndian.Uint16(resp[:2]), target.addr)
	}
	result := int16(binary.BigEndian.Uint16(resp[2:4]))
	simpleEncrypt(resp[16:])
	if result != 0 {
		msg := resp[16:]
		if i := bytes.IndexByte(msg, 0); i >= 0 {
			msg = msg[:i]
		}
		return "", fmt.Errorf("%s", decodeServerText(msg))
	}
	if len(resp) < 28 {
		return "", fmt.Errorf("short char list resp via %s", target.addr)
	}

	data := resp[16:]
	namesData := data[12:]
	var names []string
	for len(namesData) > 0 {
		i := bytes.IndexByte(namesData, 0)
		if i <= 0 {
			break
		}
		n := strings.TrimSpace(decodeServerText(namesData[:i]))
		if n != "" {
			names = append(names, n)
		}
		namesData = namesData[i+1:]
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no demo characters returned via %s", target.addr)
	}
	return names[rand.Intn(len(names))], nil
}

// login connects to the server and performs the login handshake.
// It runs the network loops and blocks until the context is canceled.
func login(ctx context.Context, clVersion int) error {
	resetDrawState()
	if gs.AutoRecord {
		recordingMovie = true
	}
	go setupSynthOnce.Do(setupSynth)
outer:
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
				imagesVersion = encodeFullVersion(clVersion)
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
				soundsVersion = encodeFullVersion(clVersion)
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

		targets := serverTargets(host)
		var lastErr error
		for i, target := range targets {
			updateConnectDialog(connectStatusMessage(target))
			err := runLoginAttempt(ctx, target, sendVersion, imagesVersion, soundsVersion)
			if err == nil {
				return nil
			}
			if errors.Is(err, errRetryLogin) {
				continue outer
			}
			lastErr = err
			if i < len(targets)-1 {
				next := targets[i+1]
				updateConnectDialog(retryConnectStatusMessage(target, next, err))
				logWarn("login via %s failed (%v); trying %s", target.display, err, next.display)
				continue
			}
			return lastErr
		}
		if lastErr != nil {
			return lastErr
		}
	}
}

func runLoginAttempt(ctx context.Context, target serverTarget, sendVersion int, imagesVersion, soundsVersion uint32) (err error) {
	var tcp net.Conn
	var udp net.Conn
	defer func() {
		recordFallbackFailure(target, err)
		if err != nil {
			if tcp != nil {
				tcp.Close()
			}
			if udp != nil {
				udp.Close()
			}
		}
	}()

	tcp, err = dialServer("tcp", target)
	if err != nil {
		return fmt.Errorf("tcp connect %s: %w", target.addr, err)
	}
	if err := tcp.SetDeadline(time.Now().Add(connectAttemptTimeout)); err != nil {
		tcp.Close()
		tcp = nil
		return fmt.Errorf("set tcp deadline %s: %w", target.addr, err)
	}

	updateConnectDialog("TCP connected; opening UDP channel...")
	udp, err = dialServer("udp", target)
	if err != nil {
		tcp.Close()
		return fmt.Errorf("udp connect %s: %w", target.addr, err)
	}
	if err := udp.SetDeadline(time.Now().Add(connectAttemptTimeout)); err != nil {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("set udp deadline %s: %w", target.addr, err)
	}

	updateConnectDialog("Waiting for server handshake...")
	var idBuf [4]byte
	if _, err := io.ReadFull(tcp, idBuf[:]); err != nil {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("read id via %s: %w", target.addr, err)
	}

	handshake := append([]byte{0xff, 0xff}, idBuf[:]...)
	updateConnectDialog("Sending handshake...")
	if _, err := udp.Write(handshake); err != nil {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("send handshake via %s: %w", target.addr, err)
	}

	var confirm [2]byte
	updateConnectDialog("Confirming handshake...")
	if _, err := io.ReadFull(tcp, confirm[:]); err != nil {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("confirm handshake via %s: %w", target.addr, err)
	}
	updateConnectDialog("Identifying client...")
	sendVersionLocal := sendVersion
	if err := sendClientIdentifiers(tcp, encodeFullVersion(sendVersionLocal), imagesVersion, soundsVersion); err != nil {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("send identifiers via %s: %w", target.addr, err)
	}
	logDebug("connected to %v", target.addr)

	updateConnectDialog("Waiting for server challenge...")
	msg, err := readTCPMessage(tcp)
	if err != nil {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("read challenge via %s: %w", target.addr, err)
	}
	if len(msg) < 16 {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("short challenge message via %s", target.addr)
	}
	const kMsgChallenge = 18
	tag := binary.BigEndian.Uint16(msg[:2])
	if tag != kMsgChallenge {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("unexpected msg tag %d", tag)
	}
	serverVersion := int(binary.BigEndian.Uint32(msg[4:8]) >> 8)
	if sendVersionLocal > serverVersion {
		sendVersionLocal = serverVersion
	}
	challenge := msg[16 : 16+16]

	if pass == "" && passHash == "" {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("character password required")
	}
	playerName = utfFold(name)
	updateGameWindowTitle()
	applyLocalLabels()
	applyEnabledScripts()
	loadShortcuts()

	var resp []byte
	var result int16
	updateConnectDialog("Authenticating...")
	for {
		var answer []byte
		if pass != "" {
			answer, err = answerChallenge(pass, challenge)
		} else {
			answer, err = answerChallengeHash(passHash, challenge)
		}
		if err != nil {
			tcp.Close()
			tcp = nil
			udp.Close()
			udp = nil
			return fmt.Errorf("hash: %w", err)
		}

		const kMsgLogOn = 13
		nameBytes := encodeMacRoman(name)
		buf := make([]byte, 16+len(nameBytes)+1+len(answer))
		binary.BigEndian.PutUint16(buf[0:2], kMsgLogOn)
		binary.BigEndian.PutUint16(buf[2:4], 0)
		binary.BigEndian.PutUint32(buf[4:8], encodeFullVersion(sendVersionLocal))
		binary.BigEndian.PutUint32(buf[8:12], imagesVersion)
		binary.BigEndian.PutUint32(buf[12:16], soundsVersion)
		copy(buf[16:], nameBytes)
		buf[16+len(nameBytes)] = 0
		copy(buf[17+len(nameBytes):], answer)
		simpleEncrypt(buf[16:])

		updateConnectDialog("Sending credentials...")
		if err := sendTCPMessage(tcp, buf); err != nil {
			tcp.Close()
			tcp = nil
			udp.Close()
			udp = nil
			return fmt.Errorf("send login via %s: %w", target.addr, err)
		}

		updateConnectDialog("Waiting for login response...")
		resp, err = readTCPMessage(tcp)
		if err != nil {
			tcp.Close()
			tcp = nil
			udp.Close()
			udp = nil
			return fmt.Errorf("read login response via %s: %w", target.addr, err)
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
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("unexpected response tag %d", resTag)
	}

	if result == -30972 || result == -30973 {
		updateConnectDialog("Server requested update; retrying...")
		_, _ = autoUpdate(resp, dataDirPath)
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return errRetryLogin
	}

	if result != 0 {
		if result == -30987 {
			passHash = ""
			setCharacterPassHash(name, "", false)
		}
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		if name, ok := errorNames[result]; ok {
			return fmt.Errorf("login failed: %s (%d)", name, result)
		}
		return fmt.Errorf("login failed: %d", result)
	}

	logDebug("login succeeded, reading messages (Ctrl-C to quit)...")
	if err := loadLegacyMacrosForCharacter(playerName); err != nil {
		log.Printf("legacy macros: %v", err)
	}
	updateConnectDialog("Login successful!")
	closeConnectDialog()

	shaderWarnShown = false
	lowFPSSince = time.Time{}
	shaderWarnWin = nil

	inputMu.Lock()
	s := latestInput
	inputMu.Unlock()
	if err := sendPlayerInput(udp, s.mouseX, s.mouseY, s.mouseDown, false); err != nil {
		logError("send player input: %v", err)
	}

	loginMu.Lock()
	tcpConn = tcp
	loginMu.Unlock()

	if err := tcp.SetDeadline(time.Time{}); err != nil {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		loginMu.Lock()
		tcpConn = nil
		loginMu.Unlock()
		return fmt.Errorf("clear tcp deadline %s: %w", target.addr, err)
	}
	if err := udp.SetDeadline(time.Time{}); err != nil {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		loginMu.Lock()
		tcpConn = nil
		loginMu.Unlock()
		return fmt.Errorf("clear udp deadline %s: %w", target.addr, err)
	}

	go sendInputLoop(ctx, udp, tcp)
	go udpReadLoop(ctx, udp)
	go tcpReadLoop(ctx, tcp)

	<-ctx.Done()
	if tcp != nil {
		tcp.Close()
		loginMu.Lock()
		tcpConn = nil
		loginMu.Unlock()
		tcp = nil
	}
	if udp != nil {
		udp.Close()
	}
	return nil
}
