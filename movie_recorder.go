package main

import (
	"encoding/binary"
	"os"
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
	f        *os.File
	head     fileHead
	preFlags uint16
	preData  []byte
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
	if len(data) == 0 {
		return
	}
	m.preData = append(m.preData, data...)
	m.preFlags |= flag
}

func gameStateBlock(payload []byte) []byte {
	buf := make([]byte, 24+len(payload))
	binary.BigEndian.PutUint32(buf[12:], uint32(len(payload)))
	copy(buf[24:], payload)
	return buf
}

func (m *movieRecorder) WriteFrame(data []byte, flags uint16) error {
	if m.f == nil {
		return os.ErrClosed
	}
	fh := frameHead{
		Signature: movieSignature,
		Frame:     m.head.Frames,
		Size:      uint16(len(data)),
		Flags:     flags | m.preFlags,
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
	if len(m.preData) > 0 {
		if _, err := m.f.Write(m.preData); err != nil {
			return err
		}
		m.preData = nil
		m.preFlags = 0
	}
	_, err := m.f.Write(data)
	return err
}

func (m *movieRecorder) Close() error {
	if m.f == nil {
		return nil
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
