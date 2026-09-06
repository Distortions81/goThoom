// Package demoserver implements a small, in-memory Clan Lord protocol playground.
package demoserver

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"gothoom/internal/twofish"
)

const maxPacket = 1396
const tickInterval = 100 * time.Millisecond

type player struct {
	conn      net.Conn
	id        uint32
	udp       *net.UDPAddr
	ready     chan struct{}
	out       chan []byte
	slot      byte
	x, y      float64
	mx, my    int16
	walking   bool
	pose      byte
	ack       byte
	frame     uint32
	lastInput time.Time
	messages  []message
}
type message struct {
	speaker byte
	text    []byte
}

type Server struct {
	tcp   net.Listener
	udp   *net.UDPConn
	mu    sync.Mutex
	peers map[uint32]*player
	byUDP map[string]*player
	slots int
	wg    sync.WaitGroup
	done  chan struct{}
	once  sync.Once
}

// Listen binds TCP and UDP on the same port. Zero chooses an available port.
func Listen(addr string, slots int) (*Server, error) {
	if slots < 1 || slots > 16 {
		return nil, fmt.Errorf("demo slots must be between 1 and 16")
	}
	tcp, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	a := tcp.Addr().(*net.TCPAddr)
	udp, err := net.ListenUDP("udp", &net.UDPAddr{IP: a.IP, Port: a.Port, Zone: a.Zone})
	if err != nil {
		tcp.Close()
		return nil, err
	}
	return &Server{tcp: tcp, udp: udp, peers: map[uint32]*player{}, byUDP: map[string]*player{}, slots: slots, done: make(chan struct{})}, nil
}
func (s *Server) Addr() net.Addr { return s.tcp.Addr() }
func (s *Server) Close() {
	s.once.Do(func() {
		close(s.done)
		s.tcp.Close()
		s.udp.Close()
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, p := range s.peers {
			p.conn.Close()
		}
	})
}

// Serve blocks until cancellation or Close. All session state is discarded.
func (s *Server) Serve(ctx context.Context) error {
	go func() {
		select {
		case <-ctx.Done():
			s.Close()
		case <-s.done:
		}
	}()
	s.wg.Add(2)
	go s.readUDP()
	go s.tick()
	defer s.wg.Wait()
	for {
		conn, err := s.tcp.Accept()
		if err != nil {
			select {
			case <-s.done:
				return nil
			default:
				s.Close()
				return err
			}
		}
		var idBytes [4]byte
		if _, err = rand.Read(idBytes[:]); err != nil {
			conn.Close()
			continue
		}
		p := &player{conn: conn, id: binary.BigEndian.Uint32(idBytes[:]), ready: make(chan struct{}), out: make(chan []byte, 32), lastInput: time.Now()}
		s.mu.Lock()
		if len(s.peers) >= 64 || s.peers[p.id] != nil {
			s.mu.Unlock()
			conn.Close()
			continue
		}
		s.peers[p.id] = p
		s.mu.Unlock()
		s.wg.Add(1)
		go s.session(p)
	}
}
func (s *Server) session(p *player) {
	defer s.wg.Done()
	defer p.conn.Close()
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.peers, p.id)
		if p.udp != nil {
			delete(s.byUDP, p.udp.String())
		}
		if p.slot != 0 {
			s.broadcast(message{p.slot, []byte("has left the practice area.")})
		}
	}()
	p.conn.SetDeadline(time.Now().Add(10 * time.Second))
	var id [4]byte
	binary.BigEndian.PutUint32(id[:], p.id)
	if writeAll(p.conn, id[:]) != nil {
		return
	}
	select {
	case <-p.ready:
	case <-time.After(10 * time.Second):
		return
	case <-s.done:
		return
	}
	if writeAll(p.conn, []byte{0, 0}) != nil {
		return
	}
	identifiers, err := readPacket(p.conn)
	if err != nil || len(identifiers) < 16 || binary.BigEndian.Uint16(identifiers) != 19 {
		return
	}
	challenge := make([]byte, 32)
	binary.BigEndian.PutUint16(challenge, 18)
	copy(challenge[4:16], identifiers[4:16])
	if _, err = rand.Read(challenge[16:]); err != nil {
		return
	}
	if writePacket(p.conn, challenge) != nil {
		return
	}
	request, err := readPacket(p.conn)
	if err != nil || len(request) < 33 {
		return
	}
	tag := binary.BigEndian.Uint16(request)
	if tag != 13 && tag != 14 {
		return
	}
	xor(request[16:])
	end := bytes.IndexByte(request[16:], 0)
	if end < 1 {
		return
	}
	name := string(request[16 : 16+end])
	answer := request[17+end:]
	expected := demoAnswer(challenge[16:])
	result := int16(0)
	if len(answer) != 16 || subtle.ConstantTimeCompare(answer, expected) != 1 {
		result = -30998
	}
	response := make([]byte, 16)
	binary.BigEndian.PutUint16(response, tag)
	copy(response[4:16], identifiers[4:16])
	if tag == 14 {
		if !strings.EqualFold(name, "demo") {
			result = -30998
		}
		response = append(response, make([]byte, 12)...)
		for i := 1; i <= s.slots; i++ {
			response = append(response, []byte(fmt.Sprintf("Demo%d", i))...)
			response = append(response, 0)
		}
		response = append(response, 0)
		xor(response[16:])
		binary.BigEndian.PutUint16(response[2:], uint16(result))
		writePacket(p.conn, response)
		return
	}
	s.mu.Lock()
	slot := 0
	for i := 1; i <= s.slots; i++ {
		if strings.EqualFold(name, fmt.Sprintf("Demo%d", i)) {
			slot = i
			break
		}
	}
	if slot == 0 {
		result = -30999
	}
	if result == 0 {
		for _, other := range s.peers {
			if other.slot == byte(slot) {
				result = -30981
				break
			}
		}
	}
	if result == 0 {
		p.slot = byte(slot)
		p.x = float64((slot-1)%4)*48 - 72
		p.y = float64((slot-1)/4)*48 - 24
		p.lastInput = time.Now()
	}
	// Do not enqueue frames until the login response is written.
	binary.BigEndian.PutUint16(response[2:], uint16(result))
	err = writePacket(p.conn, response)
	if err == nil && result == 0 {
		s.broadcast(message{p.slot, []byte("has joined. Welcome! Walk with the mouse; type to chat. /who /help /quit")})
	}
	s.mu.Unlock()
	if err != nil || result != 0 {
		return
	}
	p.conn.SetDeadline(time.Time{})
	writerDone := make(chan struct{})
	writerStop := make(chan struct{})
	go func() {
		defer close(writerDone)
		for {
			select {
			case packet := <-p.out:
				p.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
				if writePacket(p.conn, packet) != nil {
					p.conn.Close()
					return
				}
			case <-writerStop:
				return
			case <-s.done:
				return
			}
		}
	}()
	// Stop and join the writer before releasing this demo slot.
	defer func() { close(writerStop); p.conn.Close(); <-writerDone }()
	for {
		packet, err := readPacket(p.conn)
		if err != nil {
			return
		}
		s.mu.Lock()
		s.input(p, packet)
		s.mu.Unlock()
	}
}
func (s *Server) readUDP() {
	defer s.wg.Done()
	buf := make([]byte, maxPacket+3)
	for {
		n, addr, err := s.udp.ReadFromUDP(buf)
		if err != nil {
			return
		}
		s.mu.Lock()
		if n == 6 && buf[0] == 255 && buf[1] == 255 {
			id := binary.BigEndian.Uint32(buf[2:6])
			p := s.peers[id]
			if p != nil && p.udp == nil && p.conn.RemoteAddr().(*net.TCPAddr).IP.Equal(addr.IP) && s.byUDP[addr.String()] == nil {
				p.udp = addr
				s.byUDP[addr.String()] = p
				close(p.ready)
			}
		} else if n >= 4 && n <= maxPacket+2 && int(binary.BigEndian.Uint16(buf)) == n-2 {
			if p := s.byUDP[addr.String()]; p != nil && p.slot != 0 {
				s.input(p, buf[2:n])
			}
		}
		s.mu.Unlock()
	}
}
func (s *Server) input(p *player, b []byte) {
	if len(b) < 21 || binary.BigEndian.Uint16(b) != 3 || b[len(b)-1] != 0 {
		return
	}
	p.lastInput = time.Now()
	p.mx = int16(binary.BigEndian.Uint16(b[2:]))
	p.my = int16(binary.BigEndian.Uint16(b[4:]))
	p.walking = binary.BigEndian.Uint16(b[6:])&1 != 0
	id := byte(binary.BigEndian.Uint32(b[16:]))
	cmd := b[20 : len(b)-1]
	if len(cmd) == 0 || id == p.ack || byte(id-p.ack) > 127 {
		return
	}
	p.ack = id
	// Keep chat bounded and disallow control bytes / protocol markup injection.
	clean := make([]byte, 0, len(cmd))
	for _, c := range cmd {
		if c >= 32 && c != 127 && c != 0xc2 {
			clean = append(clean, c)
		}
	}
	if len(clean) > 240 {
		clean = clean[:240]
	}
	text := strings.TrimSpace(string(clean))
	if text == "" {
		return
	}
	if strings.HasPrefix(text, "/") {
		switch strings.ToLower(strings.Fields(text)[0]) {
		case "/quit":
			p.conn.Close()
		case "/who":
			names := []string{}
			for _, q := range s.peers {
				if q.slot != 0 {
					names = append(names, fmt.Sprintf("Demo%d", q.slot))
				}
			}
			sort.Strings(names)
			s.tell(p, "Online: "+strings.Join(names, ", "))
		case "/help":
			s.tell(p, "Demo playground: hold the mouse to walk, type to chat. Commands: /who /help /quit")
		case "/be-info", "/be-who", "/be-share": // Optional client metadata is outside this minimal server.
		default:
			s.tell(p, "This demo supports chat, walking, /who, /help and /quit.")
		}
	} else {
		s.broadcast(message{p.slot, []byte(text)})
	}
}
func (s *Server) tell(p *player, text string) {
	if len(p.messages) < 32 {
		p.messages = append(p.messages, message{p.slot, []byte(text)})
	}
}
func (s *Server) broadcast(m message) {
	for _, p := range s.peers {
		if p.slot != 0 && len(p.messages) < 32 {
			p.messages = append(p.messages, m)
		}
	}
}
func (s *Server) tick() {
	defer s.wg.Done()
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case now := <-ticker.C:
			s.mu.Lock()
			online := []*player{}
			for _, p := range s.peers {
				if p.slot == 0 {
					continue
				}
				if now.Sub(p.lastInput) > 30*time.Second {
					p.conn.Close()
					continue
				}
				online = append(online, p)
				if p.walking {
					dx, dy := float64(p.mx)-p.x, float64(p.my)-p.y
					distance := math.Hypot(dx, dy)
					if distance > 2 {
						step := math.Min(7, distance)
						p.x = math.Max(-220, math.Min(220, p.x+dx/distance*step))
						p.y = math.Max(-140, math.Min(140, p.y+dy/distance*step))
						p.pose = byte((int(math.Round(math.Atan2(dy, dx)*4/math.Pi))+10)%8)*4 + byte(p.frame%4)
					} else {
						p.pose &^= 3
					}
				} else {
					p.pose &^= 3
				}
			}
			sort.Slice(online, func(i, j int) bool { return online[i].slot < online[j].slot })
			for _, p := range online {
				p.frame++
				packet := s.frame(p, online)
				select {
				case p.out <- packet:
				default:
					p.conn.Close()
				}
			}
			s.mu.Unlock()
		}
	}
}

// Frame tables are complete snapshots. TCP preserves the state-data stream and
// frame sequence, so the prototype does not need a UDP retransmission history.
func (s *Server) frame(p *player, online []*player) []byte {
	b := []byte{0, 2, p.ack}
	b = binary.BigEndian.AppendUint32(b, p.frame)
	b = binary.BigEndian.AppendUint32(b, 0)
	b = append(b, byte(s.slots))
	for i := 1; i <= s.slots; i++ {
		b = append(b, byte(i), 1, 1, 191)
		b = append(b, []byte(fmt.Sprintf("Demo%d", i))...)
		b = append(b, 0, 0)
	} // sprite 447, the client's test humanoid
	b = append(b, 100, 100, 100, 100, 100, 100, 0, 0) // stats, daylight, no scenery pictures
	b = append(b, byte(len(online)))
	for _, q := range online {
		b = append(b, q.slot, q.pose)
		b = binary.BigEndian.AppendUint16(b, uint16(int16(q.x)))
		b = binary.BigEndian.AppendUint16(b, uint16(int16(q.y)))
		b = append(b, 0)
	}
	if len(p.messages) > 0 {
		m := p.messages[0]
		p.messages = p.messages[1:]
		record := []byte{0, 1, m.speaker, 0}
		record = append(record, m.text...)
		record = append(record, 0, 0, 0)
		b = binary.BigEndian.AppendUint16(b, uint16(len(record)))
		b = append(b, record...)
	}
	return b
}
func xor(b []byte) {
	key := [...]byte{0x3c, 0x5a, 0x69, 0x93, 0xa5, 0xc6}
	for i := range b {
		b[i] ^= key[i%len(key)]
	}
}
func demoAnswer(challenge []byte) []byte {
	digest := md5.Sum([]byte("demo"))
	key := make([]byte, 16)
	for i := 0; i < 16; i += 4 {
		binary.LittleEndian.PutUint32(key[i:], binary.BigEndian.Uint32(digest[i:]))
	}
	cipher, _ := twofish.NewCipher(key)
	plain := make([]byte, 16)
	cipher.Decrypt(plain, challenge)
	hash := md5.Sum(plain)
	answer := make([]byte, 16)
	cipher.Encrypt(answer, hash[:])
	return answer
}
func readPacket(r io.Reader) ([]byte, error) {
	var header [2]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint16(header[:]))
	if n < 2 || n > maxPacket {
		return nil, fmt.Errorf("invalid packet length %d", n)
	}
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	return b, err
}
func writePacket(w io.Writer, b []byte) error {
	if len(b) < 2 || len(b) > maxPacket {
		return fmt.Errorf("invalid packet length %d", len(b))
	}
	header := []byte{byte(len(b) >> 8), byte(len(b))}
	if err := writeAll(w, header); err != nil {
		return err
	}
	return writeAll(w, b)
}
func writeAll(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
