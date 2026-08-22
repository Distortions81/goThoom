package main

import (
	"encoding/binary"
	"os"
	"sort"
	"sync"
	"time"
)

type fileHead struct {
	Signature    uint32
	Version      uint16
	Len          uint16
	Frames       int32
	StartTime    uint32
	Revision     int32
	OldestReader int32
}

type frameHead struct {
	Signature uint32
	Frame     int32
	Size      uint16
	Flags     uint16
}

type movieRecorder struct {
	mu   sync.Mutex
	f    *os.File
	head fileHead
	// preData holds optional pre-frame blocks (e.g., GameState) that
	// should be written immediately before the next frame payload. These
	// bytes are not counted in the frame Size field; the parser consumes
	// them based on Flags before reading Size bytes of payload.
	preData  []byte
	preFlags uint16
}

const macEpochDelta = 2082844800

func newMovieRecorder(path string, version, revision int) (*movieRecorder, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	mr := &movieRecorder{f: f}
	mr.head = fileHead{
		Signature:    movieSignature,
		Version:      uint16(version),
		Len:          24,
		Frames:       0,
		StartTime:    uint32(time.Now().Unix() + macEpochDelta),
		Revision:     int32(revision),
		OldestReader: int32((353 << 8) + 0),
	}
	if err := mr.writeHeader(); err != nil {
		f.Close()
		return nil, err
	}
	return mr, nil
}

func (m *movieRecorder) writeHeader() error {
	buf := make([]byte, 24)
	binary.BigEndian.PutUint32(buf[0:], m.head.Signature)
	binary.BigEndian.PutUint16(buf[4:], m.head.Version)
	binary.BigEndian.PutUint16(buf[6:], m.head.Len)
	binary.BigEndian.PutUint32(buf[8:], uint32(m.head.Frames))
	binary.BigEndian.PutUint32(buf[12:], m.head.StartTime)
	binary.BigEndian.PutUint32(buf[16:], uint32(m.head.Revision))
	binary.BigEndian.PutUint32(buf[20:], uint32(m.head.OldestReader))
	if _, err := m.f.Seek(0, 0); err != nil {
		return err
	}
	_, err := m.f.Write(buf)
	return err
}

func (m *movieRecorder) AddBlock(data []byte, flag uint16) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addBlockLocked(data, flag)
}

func (m *movieRecorder) addBlockLocked(data []byte, flag uint16) {
	if len(data) == 0 {
		return
	}
	m.preData = append(m.preData, data...)
	m.preFlags |= flag
}

func gameStateBlock(leftPictID, rightPictID, mode, maxSize, curSize, expectedSize int, payload []byte) []byte {
	buf := make([]byte, 24+len(payload))
	binary.BigEndian.PutUint32(buf[0:], uint32(leftPictID))
	binary.BigEndian.PutUint32(buf[4:], uint32(rightPictID))
	binary.BigEndian.PutUint32(buf[8:], uint32(mode))
	binary.BigEndian.PutUint32(buf[12:], uint32(maxSize))
	binary.BigEndian.PutUint32(buf[16:], uint32(curSize))
	binary.BigEndian.PutUint32(buf[20:], uint32(expectedSize))
	copy(buf[24:], payload)
	return buf
}

func (m *movieRecorder) WriteFrame(data []byte, flags uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeFrameLocked(data, flags)
}

func (m *movieRecorder) writeFrameLocked(data []byte, flags uint16) error {
	if m.f == nil {
		return os.ErrClosed
	}
	// Merge any pending pre-frame blocks and flags into this frame.
	mergedFlags := flags | m.preFlags
	fh := frameHead{
		Signature: movieSignature,
		Frame:     m.head.Frames,
		Size:      uint16(len(data)),
		Flags:     mergedFlags,
	}
	m.head.Frames++
	buf := make([]byte, 12)
	binary.BigEndian.PutUint32(buf[0:], fh.Signature)
	binary.BigEndian.PutUint32(buf[4:], uint32(fh.Frame))
	binary.BigEndian.PutUint16(buf[8:], fh.Size)
	binary.BigEndian.PutUint16(buf[10:], fh.Flags)
	if _, err := m.f.Write(buf); err != nil {
		return err
	}
	// Write any pre-frame blocks first, then the frame payload.
	if len(m.preData) > 0 {
		if _, err := m.f.Write(m.preData); err != nil {
			return err
		}
		// Clear for next frame.
		m.preData = nil
		m.preFlags = 0
	}
	_, err := m.f.Write(data)
	return err
}

// WriteNetworkMessage stores state-table messages as pre-frame blocks and all
// other messages as ordinary movie frames. This matches what parseMovie reads.
func (m *movieRecorder) WriteNetworkMessage(data []byte, flags uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(data) < 2 {
		return m.writeFrameLocked(data, flags&flagStale)
	}
	tag := binary.BigEndian.Uint16(data[:2])
	blockFlags := flags & (flagGameState | flagMobileData | flagPictureTable)
	if tag != 2 && blockFlags != 0 {
		payload := append([]byte(nil), data[2:]...)
		switch {
		case blockFlags&flagGameState != 0:
			l := len(payload)
			m.addBlockLocked(gameStateBlock(0, 0, 0, l, l, l, payload), flagGameState)
		case blockFlags&flagMobileData != 0:
			m.addBlockLocked(payload, flagMobileData)
		case blockFlags&flagPictureTable != 0:
			m.addBlockLocked(payload, flagPictureTable)
		}
		return nil
	}
	return m.writeFrameLocked(data, flags&flagStale)
}

func encodePictureTableSnapshot(pictures []framePicture) []byte {
	if len(pictures) > 0xffff {
		pictures = pictures[:0xffff]
	}
	buf := make([]byte, 2+6*len(pictures)+4)
	binary.BigEndian.PutUint16(buf[:2], uint16(len(pictures)))
	pos := 2
	for _, p := range pictures {
		binary.BigEndian.PutUint16(buf[pos:pos+2], p.PictID)
		binary.BigEndian.PutUint16(buf[pos+2:pos+4], uint16(p.H))
		binary.BigEndian.PutUint16(buf[pos+4:pos+6], uint16(p.V))
		pos += 6
	}
	return buf
}

func encodeMobileTableSnapshot(s drawState, version uint16) []byte {
	l, ok := layoutForMobileTable(version)
	if !ok {
		return nil
	}
	indexes := make([]int, 0, len(s.descriptors)+len(s.mobiles))
	seen := make(map[uint8]struct{}, len(s.descriptors)+len(s.mobiles))
	for idx := range s.descriptors {
		seen[idx] = struct{}{}
		indexes = append(indexes, int(idx))
	}
	for idx := range s.mobiles {
		if _, exists := seen[idx]; exists {
			continue
		}
		indexes = append(indexes, int(idx))
	}
	sort.Ints(indexes)

	buf := make([]byte, 0, len(indexes)*(4+16+l.descSize)+4)
	for _, rawIdx := range indexes {
		idx := uint8(rawIdx)
		mob, hasMobile := s.mobiles[idx]
		tableIdx := int32(rawIdx)
		if !hasMobile {
			tableIdx += 266
		}
		entry := make([]byte, 4)
		binary.BigEndian.PutUint32(entry, uint32(tableIdx))
		buf = append(buf, entry...)
		if hasMobile {
			mobile := make([]byte, 16)
			binary.BigEndian.PutUint32(mobile[0:4], uint32(mob.State))
			binary.BigEndian.PutUint32(mobile[4:8], uint32(int32(mob.H)))
			binary.BigEndian.PutUint32(mobile[8:12], uint32(int32(mob.V)))
			binary.BigEndian.PutUint32(mobile[12:16], uint32(mob.Colors))
			buf = append(buf, mobile...)
		}

		d := s.descriptors[idx]
		desc := make([]byte, l.descSize)
		binary.BigEndian.PutUint32(desc[0:4], uint32(d.PictID))
		binary.BigEndian.PutUint32(desc[16:20], uint32(d.Type))
		colors := d.Colors
		if len(colors) > 30 {
			colors = colors[:30]
		}
		binary.BigEndian.PutUint32(desc[l.numColorsOffset:l.numColorsOffset+4], uint32(len(colors)))
		copy(desc[l.colorsOffset:], colors)
		name := encodeMacRoman(d.Name)
		if len(name) > 47 {
			name = name[:47]
		}
		copy(desc[l.nameOffset:l.nameOffset+48], name)
		buf = append(buf, desc...)
	}
	buf = append(buf, 0xff, 0xff, 0xff, 0xff)
	return buf
}

func (m *movieRecorder) AddStateSnapshot(s drawState, version uint16) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addBlockLocked(encodeMobileTableSnapshot(s, version), flagMobileData)
	m.addBlockLocked(encodePictureTableSnapshot(s.pictures), flagPictureTable)
}

func (m *movieRecorder) WriteBlock(data []byte, flag uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(data) == 0 {
		return nil
	}
	if m.f == nil {
		return os.ErrClosed
	}
	fh := frameHead{
		Signature: movieSignature,
		Frame:     m.head.Frames,
		Size:      0,
		Flags:     flag,
	}
	m.head.Frames++
	buf := make([]byte, 12)
	binary.BigEndian.PutUint32(buf[0:], fh.Signature)
	binary.BigEndian.PutUint32(buf[4:], uint32(fh.Frame))
	binary.BigEndian.PutUint16(buf[8:], fh.Size)
	binary.BigEndian.PutUint16(buf[10:], fh.Flags)
	if _, err := m.f.Write(buf); err != nil {
		return err
	}
	_, err := m.f.Write(data)
	return err
}

func (m *movieRecorder) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.f == nil {
		return nil
	}
	// Preserve a start-state snapshot even when recording is stopped before
	// another network frame arrives.
	if len(m.preData) > 0 {
		if err := m.writeFrameLocked(nil, 0); err != nil {
			m.f.Close()
			m.f = nil
			return err
		}
	}
	if err := m.writeHeader(); err != nil {
		m.f.Close()
		m.f = nil
		return err
	}
	err := m.f.Close()
	m.f = nil
	return err
}
