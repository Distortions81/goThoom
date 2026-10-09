package main

import "encoding/binary"

// A parser owns these buffers until all record decoding finishes. Decoders may
// compact text tags in place, so records must not alias packet data or the
// pending stream retained in drawState for checkpoints and later frames.
type drawStateRecordScratch struct {
	data    []byte
	records [][]byte
}

func (s *drawStateRecordScratch) reset() {
	clear(s.records)
	s.records = s.records[:0]
	s.data = s.data[:0]
}

func (s *drawStateRecordScratch) appendFragment(session *Session, fragment []byte) [][]byte {
	s.reset()
	if len(fragment) == 0 || session == nil || session.draw == nil {
		return nil
	}

	draw := session.draw
	draw.mu.Lock()
	stream := append(draw.current.stateDataStream, fragment...)
	consumed := 0
	for len(stream)-consumed >= 2 {
		size := int(binary.BigEndian.Uint16(stream[consumed:]))
		end := consumed + 2 + size
		if end > len(stream) {
			break
		}
		consumed = end
	}
	if consumed > 0 {
		// Copy the completed prefix once into reusable parser storage. The
		// partial tail remains independent and checkpointable under draw.mu.
		s.data = append(s.data, stream[:consumed]...)
		remaining := copy(stream, stream[consumed:])
		stream = stream[:remaining]
	}
	draw.current.stateDataStream = stream
	draw.mu.Unlock()

	for offset := 0; offset < len(s.data); {
		size := int(binary.BigEndian.Uint16(s.data[offset:]))
		end := offset + 2 + size
		var record []byte
		if size > 0 {
			// Limit capacity so an append while decoding cannot touch the
			// following record in the shared packed buffer.
			record = s.data[offset+2 : end : end]
		}
		s.records = append(s.records, record)
		offset = end
	}
	return s.records
}
