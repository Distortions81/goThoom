package clsnd

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"sync"
)

type entry struct {
	offset uint32
	size   uint32
}

// Sound holds decoded PCM data and parameters.
type Sound struct {
	Data       []byte
	SampleRate uint32
	Channels   uint32
	Bits       uint16
}

// CLSounds provides access to sounds stored in the CL_Sounds keyfile.
type CLSounds struct {
	data  []byte
	index map[uint32]entry
	cache map[uint32]*Sound
	mu    sync.Mutex
}

const (
	typeSound      = 0x736e6420 // 'snd '
	bufferCmd      = 0x51
	dataOffsetFlag = 0x8000
)

// Load parses the CL_Sounds keyfile located at path.
func Load(path string) (*CLSounds, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("CL_Sounds file missing.")
		return nil, err
	}
	if len(data) < 12 {
		log.Printf("CL_Sounds may be corrupt.")
		return nil, fmt.Errorf("short file")
	}
	if binary.BigEndian.Uint16(data[:2]) != 0xffff {
		log.Printf("CL_Sounds invalid.")
		return nil, fmt.Errorf("bad header")
	}
	r := data[2:]
	entryCount := binary.BigEndian.Uint32(r[:4])
	r = r[4+4+2:] // skip pad1, pad2

	idx := make(map[uint32]entry, entryCount)
	for i := uint32(0); i < entryCount; i++ {
		if len(r) < 16 {
			log.Printf("CL_Sounds may be corrupt.")
			return nil, fmt.Errorf("truncated table")
		}
		off := binary.BigEndian.Uint32(r[0:4])
		size := binary.BigEndian.Uint32(r[4:8])
		typ := binary.BigEndian.Uint32(r[8:12])
		id := binary.BigEndian.Uint32(r[12:16])
		if typ == typeSound {
			idx[id] = entry{offset: off, size: size}
		}
		r = r[16:]
	}
	return &CLSounds{data: data, index: idx, cache: make(map[uint32]*Sound)}, nil
}

// Get returns the decoded sound for the given id. The sound data is loaded
// on demand and cached for subsequent calls. If the id exists but the sound
// data cannot be decoded an error is returned.
func (c *CLSounds) Get(id uint32) (*Sound, error) {
	c.mu.Lock()
	if s, ok := c.cache[id]; ok {
		c.mu.Unlock()
		return s, nil
	}
	c.mu.Unlock()

	e, ok := c.index[id]
	if !ok {
		return nil, nil
	}
	if int(e.offset+e.size) > len(c.data) {
		return nil, fmt.Errorf("sound data out of range")
	}
	sndData := c.data[e.offset : e.offset+e.size]
	hdrOff, ok := soundHeaderOffset(sndData)
	if !ok || hdrOff+22 > len(sndData) {
		return nil, fmt.Errorf("missing sound header")
	}
	s, err := decodeHeader(sndData, hdrOff, id)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.cache[id] = s
	c.mu.Unlock()
	return s, nil
}

// ClearCache discards all decoded sound data.
func (c *CLSounds) ClearCache() {
	c.mu.Lock()
	c.cache = make(map[uint32]*Sound)
	c.mu.Unlock()
}

// IDs returns all sound identifiers present in the archive.
func (c *CLSounds) IDs() []uint32 {
	ids := make([]uint32, 0, len(c.index))
	for id := range c.index {
		ids = append(ids, id)
	}
	return ids
}

// soundHeaderOffset locates the SoundHeader inside a 'snd ' resource.
func soundHeaderOffset(data []byte) (int, bool) {
	if len(data) < 6 {
		return 0, false
	}
	if binary.BigEndian.Uint16(data[0:2]) != 1 { // format-1 resource
		return 0, false
	}
	nMods := int(binary.BigEndian.Uint16(data[2:4]))
	p := 4 + nMods*6
	if p+2 > len(data) {
		return 0, false
	}
	nCmds := int(binary.BigEndian.Uint16(data[p : p+2]))
	p += 2
	for i := 0; i < nCmds; i++ {
		if p+8 > len(data) {
			return 0, false
		}
		cmd := binary.BigEndian.Uint16(data[p : p+2])
		off := int(binary.BigEndian.Uint32(data[p+4 : p+8]))
		if cmd == dataOffsetFlag|bufferCmd {
			return off, true
		}
		p += 8
	}
	return 0, false
}

func decodeHeader(data []byte, hdr int, id uint32) (*Sound, error) {
	if hdr+22 > len(data) {
		return nil, fmt.Errorf("header out of range")
	}
	encode := data[hdr+20]
	switch encode {
	case 0: // stdSH: 8-bit, mono
		length := int(binary.BigEndian.Uint32(data[hdr+4 : hdr+8]))
		rate := binary.BigEndian.Uint32(data[hdr+8:hdr+12]) >> 16
		start := hdr + 22
		if start > len(data) {
			return nil, fmt.Errorf("data out of range")
		}
		if end := start + length; end > len(data) {
			if id != 0 {
				log.Printf("truncated sound data for id %d: have %d bytes, expected %d", id, len(data)-start, length)
			} else {
				log.Printf("truncated sound data: have %d bytes, expected %d", len(data)-start, length)
			}
			length = len(data) - start
		}
		s := &Sound{
			Data:       append([]byte(nil), data[start:start+length]...),
			SampleRate: rate,
			Channels:   1,
			Bits:       8,
		}
		return s, nil
	case 0xfe: // CmpSoundHeader: may contain compression
		if hdr+64 > len(data) {
			return nil, fmt.Errorf("short cmp header")
		}
		compID := int16(binary.BigEndian.Uint16(data[hdr+56 : hdr+58]))
		format := binary.BigEndian.Uint32(data[hdr+40 : hdr+44])
		chans := binary.BigEndian.Uint32(data[hdr+4 : hdr+8])
		rate := binary.BigEndian.Uint32(data[hdr+8:hdr+12]) >> 16
		frames := int(binary.BigEndian.Uint32(data[hdr+22 : hdr+26]))
		bits := binary.BigEndian.Uint16(data[hdr+62 : hdr+64])
		start := hdr + 64
		switch compID {
		case 0, -1: // uncompressed PCM
			if format != 0x72617720 && format != 0x74776f73 { // 'raw ' or 'twos'
				if id != 0 {
					log.Printf("sound %d: unsupported format %08x for compression %d", id, format, compID)
				} else {
					log.Printf("unsupported format %08x for compression %d", format, compID)
				}
				return nil, fmt.Errorf("unsupported format %08x", format)
			}
			bytesPerSample := int(bits) / 8
			length := frames * int(chans) * bytesPerSample
			if start > len(data) {
				return nil, fmt.Errorf("data out of range")
			}
			if length > len(data)-start {
				if id != 0 {
					log.Printf("truncated sound data for id %d: have %d bytes, expected %d", id, len(data)-start, length)
				} else {
					log.Printf("truncated sound data: have %d bytes, expected %d", len(data)-start, length)
				}
				length = len(data) - start
			}
			s := &Sound{
				Data:       append([]byte(nil), data[start:start+length]...),
				SampleRate: rate,
				Channels:   chans,
				Bits:       bits,
			}
			return s, nil
		case -4: // IMA4 ADPCM
			if format != 0x696d6134 { // 'ima4'
				if id != 0 {
					log.Printf("sound %d: unsupported format %08x for compression %d", id, format, compID)
				} else {
					log.Printf("unsupported format %08x for compression %d", format, compID)
				}
				return nil, fmt.Errorf("unsupported format %08x", format)
			}
			if bits != 16 {
				if id != 0 {
					log.Printf("sound %d: ima4 unsupported bits %d", id, bits)
				} else {
					log.Printf("ima4 unsupported bits %d", bits)
				}
				return nil, fmt.Errorf("ima4 unsupported bits %d", bits)
			}
			if start > len(data) {
				return nil, fmt.Errorf("data out of range")
			}
			pcm, err := decodeIMA4(data[start:], int(chans))
			if err != nil {
				if id != 0 {
					log.Printf("sound %d: ima4 decode error: %v", id, err)
				} else {
					log.Printf("ima4 decode error: %v", err)
				}
				return nil, err
			}
			expected := frames * int(chans) * (int(bits) / 8)
			if len(pcm) != expected {
				if id != 0 {
					log.Printf("sound %d: ima4 decoded %d bytes, expected %d; continuing", id, len(pcm), expected)
				} else {
					log.Printf("ima4 decoded %d bytes, expected %d; continuing", len(pcm), expected)
				}
				// Intentionally lenient: historical assets sometimes report incorrect lengths.
			}
			s := &Sound{
				Data:       pcm,
				SampleRate: rate,
				Channels:   chans,
				Bits:       bits,
			}
			return s, nil
		case -2, -3: // MACE 3:1 or 6:1
			if id != 0 {
				log.Printf("sound %d: unsupported compression %d format %08x", id, compID, format)
			} else {
				log.Printf("unsupported compression %d format %08x", compID, format)
			}
			return nil, fmt.Errorf("unsupported compression %d", compID)
		default:
			if id != 0 {
				log.Printf("sound %d: unsupported compression %d format %08x", id, compID, format)
			} else {
				log.Printf("unsupported compression %d format %08x", compID, format)
			}
			return nil, fmt.Errorf("unsupported compression %d", compID)
		}

	case 0xff: // ExtSoundHeader: allow 16-bit or multi-channel
		if hdr+64 > len(data) {
			return nil, fmt.Errorf("short ext header")
		}
		chans := binary.BigEndian.Uint32(data[hdr+4 : hdr+8])
		rate := binary.BigEndian.Uint32(data[hdr+8:hdr+12]) >> 16
		frames := int(binary.BigEndian.Uint32(data[hdr+22 : hdr+26]))
		bits := binary.BigEndian.Uint16(data[hdr+48 : hdr+50])
		start := hdr + 64
		bytesPerSample := int(bits) / 8
		length := frames * int(chans) * bytesPerSample
		if start > len(data) {
			return nil, fmt.Errorf("data out of range")
		}
		if length > len(data)-start {
			if id != 0 {
				log.Printf("truncated sound data for id %d: have %d bytes, expected %d", id, len(data)-start, length)
			} else {
				log.Printf("truncated sound data: have %d bytes, expected %d", len(data)-start, length)
			}
			length = len(data) - start
		}
		s := &Sound{
			Data:       append([]byte(nil), data[start:start+length]...),
			SampleRate: rate,
			Channels:   chans,
			Bits:       bits,
		}
		return s, nil
	default:
		if id != 0 {
			log.Printf("sound %d: unsupported encode %d", id, encode)
		} else {
			log.Printf("unsupported encode %d", encode)
		}
		return nil, fmt.Errorf("unsupported encode %d", encode)
	}
}
