package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	demoLookupInProgress bool
	demoLoginActive      bool
	loginMu              sync.Mutex
)

type serverTarget struct {
	addr     string
	display  string
	fallback bool
}

var (
	errRetryLogin    = errors.New("retry login")
	errDemoSlotsUsed = errors.New("Sorry, all demo slots seem to be used.")
)

type loginResultError struct {
	result int16
}

const loginResultCharacterAlreadyOnline int16 = -30981

func (e *loginResultError) Error() string {
	if name, ok := errorNames[e.result]; ok {
		return fmt.Sprintf("login failed: %s (%d)", name, e.result)
	}
	return fmt.Sprintf("login failed: %d", e.result)
}

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

func updateSessionConnectStatus(session *Session, status string) {
	if session != nil {
		session.login.setStatus(status, nil)
		queueSessionWorkspaceUIUpdate()
	}
	if session == primarySession {
		dispatchMainThread(func() { updateConnectDialog(status) })
	}
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
	handleSessionDisconnect(primarySession)
}

func handleSessionDisconnect(session *Session) {
	if session == nil {
		return
	}
	session.transport.disconnect()
}

func completeSessionDisconnect(session *Session) {
	if session != nil {
		session.login.setStatus("Disconnected", nil)
		queueSessionWorkspaceUIUpdate()
	}
	if session != primarySession {
		session.resetConnectionModels()
		session.login.clearCredentials()
		session.setCharacterName("")
		return
	}
	loginMu.Lock()
	tcpConn = nil
	wasDemo := demoLoginActive
	demoLoginActive = false
	loginMu.Unlock()

	endSessionScripts(scriptSessionGeneration.Load())
	stopAllMusic()
	if recorder != nil {
		stopRecording()
	}
	resetFrameStatistics()
	resetNightState()
	// Reset session sources so we return to splash state
	clmov = ""
	pcapPath = ""
	pass = ""
	passHash = ""
	primarySession.login.clearCredentials()
	if wasDemo {
		name = freeDemoSelection
	}
	discardStagedPassword()
	consoleMessage("Disconnected from server.")
	if loginWin != nil {
		loginWin.MarkOpen()
	}
	updateCharacterButtons()
}

const CL_ImagesFile = "CL_Images"
const CL_SoundsFile = "CL_Sounds"

// fetchDemoCharacters retrieves the server's available demo characters for
// the player to choose from.
func fetchDemoCharacters(clVersion int) ([]string, error) {
	return fetchDemoCharactersAtHost(clVersion, host)
}

func fetchSessionDemoCharacters(session *Session, clVersion int) ([]string, error) {
	if session == nil {
		return nil, errors.New("login session is nil")
	}
	return fetchDemoCharactersAtHost(clVersion, session.login.requestSnapshot().host)
}

func fetchDemoCharactersAtHost(clVersion int, serverAddress string) ([]string, error) {
	serverAddress = strings.TrimSpace(serverAddress)
	if serverAddress == "" {
		return nil, errors.New("server address is empty")
	}
	for {
		names, err := fetchDemoCharactersOnceAtHost(clVersion, serverAddress)
		if errors.Is(err, errRetryLogin) {
			continue
		}
		return names, err
	}
}

func setDemoLoginCandidate(candidate string) {
	setSessionDemoLoginCandidate(primarySession, candidate)
}

func setSessionDemoLoginCandidate(session *Session, candidate string) sessionLoginRequest {
	if session == nil {
		return sessionLoginRequest{}
	}
	request := session.login.setDemoCandidate(candidate)
	if session == primarySession {
		name = request.character
		passHash = ""
		pass = "demo"
	}
	return request
}

func nextDemoCandidateIndex(err error, current, count int) (int, bool) {
	var resultErr *loginResultError
	if !errors.As(err, &resultErr) || resultErr.result != loginResultCharacterAlreadyOnline {
		return current, false
	}
	next := current + 1
	return next, next < count
}

func parseDemoCharacterNames(data []byte) []string {
	if len(data) < 12 {
		return nil
	}
	namesData := data[12:]
	var names []string
	seenNames := make(map[string]struct{})
	for len(namesData) > 0 {
		i := bytes.IndexByte(namesData, 0)
		if i <= 0 {
			break
		}
		n := strings.TrimSpace(decodeServerText(namesData[:i]))
		if n != "" {
			key := strings.ToLower(n)
			if _, seen := seenNames[key]; !seen {
				seenNames[key] = struct{}{}
				names = append(names, n)
			}
		}
		namesData = namesData[i+1:]
	}
	return names
}

func fetchDemoCharactersOnce(clVersion int) ([]string, error) {
	return fetchDemoCharactersOnceAtHost(clVersion, host)
}

func fetchDemoCharactersOnceAtHost(clVersion int, serverAddress string) ([]string, error) {
	imagesVersion, err := readKeyFileVersion(assetFilePath(CL_ImagesFile))
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

	soundsVersion, err := readKeyFileVersion(assetFilePath(CL_SoundsFile))
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

	targets := serverTargets(serverAddress)
	var lastErr error
	for i, target := range targets {
		names, err := fetchDemoFromTarget(target, sendVersion, imagesVersion, soundsVersion)
		if err == nil {
			return names, nil
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
	return nil, lastErr
}

func fetchDemoFromTarget(target serverTarget, sendVersion int, imagesVersion, soundsVersion uint32) ([]string, error) {
	tcpConn, err := dialServer("tcp", target)
	if err != nil {
		return nil, fmt.Errorf("tcp connect %s: %w", target.addr, err)
	}
	defer tcpConn.Close()

	udpConn, err := dialServer("udp", target)
	if err != nil {
		return nil, fmt.Errorf("udp connect %s: %w", target.addr, err)
	}
	defer udpConn.Close()

	var idBuf [4]byte
	if _, err := io.ReadFull(tcpConn, idBuf[:]); err != nil {
		return nil, fmt.Errorf("read id via %s: %w", target.addr, err)
	}
	handshake := append([]byte{0xff, 0xff}, idBuf[:]...)
	if _, err := udpConn.Write(handshake); err != nil {
		return nil, fmt.Errorf("send handshake via %s: %w", target.addr, err)
	}
	var confirm [2]byte
	if _, err := io.ReadFull(tcpConn, confirm[:]); err != nil {
		return nil, fmt.Errorf("confirm handshake via %s: %w", target.addr, err)
	}
	if err := sendClientIdentifiers(tcpConn, encodeFullVersion(sendVersion), imagesVersion, soundsVersion); err != nil {
		return nil, fmt.Errorf("send identifiers via %s: %w", target.addr, err)
	}

	msg, err := readTCPMessage(tcpConn)
	if err != nil {
		return nil, fmt.Errorf("read challenge via %s: %w", target.addr, err)
	}
	if len(msg) < 32 {
		return nil, fmt.Errorf("short challenge message via %s", target.addr)
	}
	const kMsgChallenge = 18
	if binary.BigEndian.Uint16(msg[:2]) != kMsgChallenge {
		return nil, fmt.Errorf("unexpected msg tag %d via %s", binary.BigEndian.Uint16(msg[:2]), target.addr)
	}
	serverVersion := int(binary.BigEndian.Uint32(msg[4:8]) >> 8)
	sendVersionLocal := sendVersion
	if sendVersionLocal > serverVersion {
		sendVersionLocal = serverVersion
	}
	challenge := msg[16 : 16+16]

	const kMsgCharList = 14
	accountBytes := encodeMacRoman("demo")
	var resp []byte
	for {
		answer, err := answerChallenge("demo", challenge)
		if err != nil {
			return nil, fmt.Errorf("hash via %s: %w", target.addr, err)
		}
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
			return nil, fmt.Errorf("send character list via %s: %w", target.addr, err)
		}

		resp, err = readTCPMessage(tcpConn)
		if err != nil {
			return nil, fmt.Errorf("read character list via %s: %w", target.addr, err)
		}
		if len(resp) < 16 {
			return nil, fmt.Errorf("short char list resp via %s", target.addr)
		}
		tag := binary.BigEndian.Uint16(resp[:2])
		result := int16(binary.BigEndian.Uint16(resp[2:4]))
		if result == -30972 || result == -30973 {
			if _, err := autoUpdate(resp, assetsDirPath()); err != nil {
				return nil, fmt.Errorf("update demo data: %w", err)
			}
			return nil, errRetryLogin
		}
		if tag == kMsgChallenge {
			if len(resp) < 32 {
				return nil, fmt.Errorf("short repeated challenge via %s", target.addr)
			}
			challenge = resp[16:32]
			continue
		}
		if tag != kMsgCharList {
			return nil, fmt.Errorf("unexpected tag %d via %s", tag, target.addr)
		}
		break
	}
	result := int16(binary.BigEndian.Uint16(resp[2:4]))
	simpleEncrypt(resp[16:])
	if result != 0 {
		msg := resp[16:]
		if i := bytes.IndexByte(msg, 0); i >= 0 {
			msg = msg[:i]
		}
		return nil, fmt.Errorf("%s", decodeServerText(msg))
	}
	if len(resp) < 28 {
		return nil, fmt.Errorf("short char list resp via %s", target.addr)
	}

	names := parseDemoCharacterNames(resp[16:])
	if len(names) == 0 {
		return nil, fmt.Errorf("no demo characters returned via %s", target.addr)
	}
	return names, nil
}

// login connects to the server and performs the login handshake.
// It runs the network loops and blocks until the context is canceled.
func login(ctx context.Context, clVersion int) error {
	stagePrimarySessionLoginRequest()
	return loginSessionWithDemoCandidates(primarySession, ctx, clVersion, nil)
}

func loginWithDemoCandidates(ctx context.Context, clVersion int, demoCandidates []string) error {
	stagePrimarySessionLoginRequest()
	return loginSessionWithDemoCandidates(primarySession, ctx, clVersion, demoCandidates)
}

func stagePrimarySessionLoginRequest() sessionLoginRequest {
	request := sessionLoginRequest{
		host:         host,
		character:    name,
		password:     pass,
		passwordHash: passHash,
	}.normalized()
	primarySession.login.setRequest(request)
	return request
}

func loginSessionWithDemoCandidates(session *Session, ctx context.Context, clVersion int, demoCandidates []string) error {
	if session == nil {
		return errors.New("login session is nil")
	}
	request := session.login.requestSnapshot()
	if len(demoCandidates) > 0 {
		request = setSessionDemoLoginCandidate(session, demoCandidates[0])
	}
	if err := request.validate(); err != nil {
		return err
	}
	_, _, transportStatus := session.transport.connections()
	if transportStatus == sessionDisconnected {
		sessionCtx, cancel := context.WithCancel(ctx)
		if !session.transport.begin(cancel) {
			cancel()
			return errors.New("session transport is busy")
		}
		ctx = sessionCtx
	} else if transportStatus != sessionConnecting {
		return errors.New("session transport is busy")
	}
	defer session.transport.failConnect()
	if session == primarySession {
		resetLiveNetworkSession()
	} else {
		session.resetConnectionModels()
	}
	if session == primarySession && gs.AutoRecord {
		recordingMovie = true
	}
	go setupSynthOnce.Do(setupSynth)
	demoCandidateIndex := 0
outer:
	for {
		imagesVersion, err := readKeyFileVersion(assetFilePath(CL_ImagesFile))
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

		soundsVersion, err := readKeyFileVersion(assetFilePath(CL_SoundsFile))
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

		targets := serverTargets(request.host)
		var lastErr error
		for i, target := range targets {
			updateSessionConnectStatus(session, connectStatusMessage(target))
			err := runSessionLoginAttempt(session, ctx, request, target, sendVersion, imagesVersion, soundsVersion)
			if err == nil {
				return nil
			}
			if errors.Is(err, errRetryLogin) {
				continue outer
			}
			var resultErr *loginResultError
			if errors.As(err, &resultErr) {
				if next, ok := nextDemoCandidateIndex(err, demoCandidateIndex, len(demoCandidates)); ok {
					previous := request.character
					demoCandidateIndex = next
					request = setSessionDemoLoginCandidate(session, demoCandidates[demoCandidateIndex])
					status := fmt.Sprintf("%s is in use; trying %s...", previous, request.character)
					updateSessionConnectStatus(session, status)
					logDebug("demo character %s is online; trying %s", previous, request.character)
					continue outer
				}
				if resultErr.result == loginResultCharacterAlreadyOnline && len(demoCandidates) > 1 {
					return errDemoSlotsUsed
				}
				return err
			}
			lastErr = err
			if i < len(targets)-1 {
				next := targets[i+1]
				status := retryConnectStatusMessage(target, next, err)
				updateSessionConnectStatus(session, status)
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
	request := stagePrimarySessionLoginRequest()
	return runSessionLoginAttempt(primarySession, ctx, request, target, sendVersion, imagesVersion, soundsVersion)
}

func runSessionLoginAttempt(session *Session, ctx context.Context, request sessionLoginRequest, target serverTarget, sendVersion int, imagesVersion, soundsVersion uint32) (err error) {
	if session == nil {
		return errors.New("login session is nil")
	}
	request = request.normalized()
	if err := request.validate(); err != nil {
		return err
	}
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

	updateSessionConnectStatus(session, "TCP connected; opening UDP channel...")
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

	updateSessionConnectStatus(session, "Waiting for server handshake...")
	var idBuf [4]byte
	if _, err := io.ReadFull(tcp, idBuf[:]); err != nil {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("read id via %s: %w", target.addr, err)
	}

	handshake := append([]byte{0xff, 0xff}, idBuf[:]...)
	updateSessionConnectStatus(session, "Sending handshake...")
	if _, err := udp.Write(handshake); err != nil {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("send handshake via %s: %w", target.addr, err)
	}

	var confirm [2]byte
	updateSessionConnectStatus(session, "Confirming handshake...")
	if _, err := io.ReadFull(tcp, confirm[:]); err != nil {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("confirm handshake via %s: %w", target.addr, err)
	}
	updateSessionConnectStatus(session, "Identifying client...")
	sendVersionLocal := sendVersion
	if err := sendClientIdentifiers(tcp, encodeFullVersion(sendVersionLocal), imagesVersion, soundsVersion); err != nil {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("send identifiers via %s: %w", target.addr, err)
	}
	logDebug("connected to %v", target.addr)

	updateSessionConnectStatus(session, "Waiting for server challenge...")
	msg, err := readTCPMessage(tcp)
	if err != nil {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("read challenge via %s: %w", target.addr, err)
	}
	if len(msg) < 32 {
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

	profileCharacter := utfFold(request.character)
	session.setCharacterName(profileCharacter)
	if session == primarySession {
		playerName = profileCharacter
		dispatchMainThread(updateGameWindowTitle)
		applyLocalLabels()
		loadShortcuts()
	}

	var resp []byte
	var result int16
	updateSessionConnectStatus(session, "Authenticating...")
	for {
		var answer []byte
		if request.password != "" {
			answer, err = answerChallenge(request.password, challenge)
		} else {
			answer, err = answerChallengeHash(request.passwordHash, challenge)
		}
		if err != nil {
			tcp.Close()
			tcp = nil
			udp.Close()
			udp = nil
			return fmt.Errorf("hash: %w", err)
		}

		const kMsgLogOn = 13
		nameBytes := encodeMacRoman(request.character)
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

		updateSessionConnectStatus(session, "Sending credentials...")
		if err := sendTCPMessage(tcp, buf); err != nil {
			tcp.Close()
			tcp = nil
			udp.Close()
			udp = nil
			return fmt.Errorf("send login via %s: %w", target.addr, err)
		}

		updateSessionConnectStatus(session, "Waiting for login response...")
		resp, err = readTCPMessage(tcp)
		if err != nil {
			tcp.Close()
			tcp = nil
			udp.Close()
			udp = nil
			return fmt.Errorf("read login response via %s: %w", target.addr, err)
		}
		if len(resp) < 4 {
			tcp.Close()
			tcp = nil
			udp.Close()
			udp = nil
			return fmt.Errorf("short login response via %s", target.addr)
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
			if len(resp) < 32 {
				tcp.Close()
				tcp = nil
				udp.Close()
				udp = nil
				return fmt.Errorf("short repeated challenge via %s", target.addr)
			}
			challenge = resp[16:32]
			continue
		}
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("unexpected response tag %d", resTag)
	}

	if result == -30972 || result == -30973 {
		updateSessionConnectStatus(session, "Server requested update; retrying...")
		_, _ = autoUpdate(resp, assetsDirPath())
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return errRetryLogin
	}

	if result != 0 {
		if isBadPasswordResult(result) {
			rejectSessionPassword(session, request.character)
			if session == primarySession {
				pass = ""
				passHash = ""
			}
		}
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return &loginResultError{result: result}
	}
	commitSessionStagedPassword(session, request.character)
	if session == primarySession {
		pass = ""
		passHash = ""
	}

	logDebug("login succeeded, reading messages (Ctrl-C to quit)...")
	var scriptSession uint64
	if session == primarySession {
		defer func() { dispatchMainThread(func() { endSessionScripts(scriptSession) }) }()
		dispatchMainThread(func() {
			scriptSession = startSessionScripts(profileCharacter)
		})
		dispatchMainThread(func() { updateConnectDialog("Loading macros...") })
		if err := loadLegacyMacrosForCharacter(profileCharacter); err != nil {
			log.Printf("legacy macros: %v", err)
		}
		dispatchMainThread(func() {
			updateConnectDialog("Login successful!")
			closeConnectDialog()
			shaderWarnShown = false
			lowFPSSince = time.Time{}
			shaderWarnWin = nil
		})
	} else if err := session.loadLegacyMacrosForCharacter(profileCharacter); err != nil {
		log.Printf("legacy macros for session %d: %v", session.ID(), err)
	}

	var s inputState
	if session == primarySession {
		inputMu.Lock()
		s = latestInput
		inputMu.Unlock()
	} else {
		s = session.input.next()
	}
	if err := sendSessionPlayerInput(session, udp, s.mouseX, s.mouseY, s.mouseDown, false); err != nil {
		logError("send player input: %v", err)
	}

	if err := tcp.SetDeadline(time.Time{}); err != nil {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("clear tcp deadline %s: %w", target.addr, err)
	}
	if err := udp.SetDeadline(time.Time{}); err != nil {
		tcp.Close()
		tcp = nil
		udp.Close()
		udp = nil
		return fmt.Errorf("clear udp deadline %s: %w", target.addr, err)
	}
	transportGeneration, attached := session.transport.attach(tcp, udp)
	if !attached {
		return errors.New("session transport already active")
	}
	updateSessionConnectStatus(session, "Connected")
	if session == primarySession {
		loginMu.Lock()
		tcpConn = tcp
		loginMu.Unlock()
	}

	tcpMessages := make(chan incomingServerMessage, 16)
	udpMessages := make(chan incomingServerMessage, 16)
	dispatchDone := make(chan struct{})
	var networkLoops sync.WaitGroup
	go func() {
		serverSessionMessageDispatchLoop(session, ctx, tcpMessages, udpMessages)
		close(dispatchDone)
	}()
	networkLoops.Add(3)
	go func(udpConn, tcpConn net.Conn) {
		defer networkLoops.Done()
		sendSessionInputLoop(session, ctx, udpConn, tcpConn)
	}(udp, tcp)
	go func(udpConn net.Conn) {
		defer networkLoops.Done()
		sessionUDPReadLoop(session, ctx, udpConn, udpMessages)
	}(udp)
	go func(tcpConn net.Conn) {
		defer networkLoops.Done()
		sessionTCPReadLoop(session, ctx, tcpConn, tcpMessages)
	}(tcp)

	<-ctx.Done()
	if tcp != nil {
		tcp.Close()
		tcp = nil
	}
	if udp != nil {
		udp.Close()
	}
	<-dispatchDone
	networkLoops.Wait()
	if session.transport.finish(transportGeneration) {
		completeSessionDisconnect(session)
	}
	if session == primarySession {
		loginMu.Lock()
		tcpConn = nil
		loginMu.Unlock()
	}
	return nil
}
