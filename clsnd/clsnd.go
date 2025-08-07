package clsnd

import (
	"encoding/binary"
	"fmt"
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
	typeSound = 0x736e6420 // 'snd '
)

// Load parses the CL_Sounds keyfile located at path.
func Load(path string) (*CLSounds, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("CL_Sounds file missing.")
		return nil, err
	}
	if len(data) < 12 {
		fmt.Println("CL_Sounds may be corrupt.")
		return nil, fmt.Errorf("short file")
	}
	if binary.BigEndian.Uint16(data[:2]) != 0xffff {
		fmt.Println("CL_Sounds invalid.")
		return nil, fmt.Errorf("bad header")
	}
	r := data[2:]
	entryCount := binary.BigEndian.Uint32(r[:4])
	r = r[4+4+2:] // skip pad1, pad2

	idx := make(map[uint32]entry, entryCount)
	for i := uint32(0); i < entryCount; i++ {
		if len(r) < 16 {
			fmt.Println("CL_Sounds may be corrupt.")
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
// on demand and cached for subsequent calls.
func (c *CLSounds) Get(id uint32) *Sound {
	c.mu.Lock()
	if s, ok := c.cache[id]; ok {
		c.mu.Unlock()
		return s
	}
	c.mu.Unlock()

	e, ok := c.index[id]
	if !ok {
		return nil
	}
	if int(e.offset+e.size) > len(c.data) {
		return nil
	}
	sndData := c.data[e.offset : e.offset+e.size]
	hdrOff, ok := soundHeaderOffset(sndData)
	if !ok || hdrOff+22 > len(sndData) {
		return nil
	}
	s, err := decodeHeader(sndData, hdrOff, id)
	if err != nil {
		fmt.Printf("sound get error: %v\n", err)
		return nil
	}
	c.mu.Lock()
	c.cache[id] = s
	c.mu.Unlock()
	return s
}

// ClearCache discards all decoded sound data.
func (c *CLSounds) ClearCache() {
	c.mu.Lock()
	c.cache = make(map[uint32]*Sound)
	c.mu.Unlock()
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
		if cmd&0x8000 != 0 { // high bit indicates offset form
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
			fmt.Printf("truncated sound data")
			if id != 0 {
				fmt.Printf(" for id %d", id)
			}
			fmt.Printf(": have %d bytes, expected %d\n", len(data)-start, length)
			length = len(data) - start
		}
		s := &Sound{
			Data:       append([]byte(nil), data[start:start+length]...),
			SampleRate: rate,
			Channels:   1,
			Bits:       8,
		}
		return s, nil
	case 0xff: // ExtSoundHeader: allow 16-bit or multi-channel
		if hdr+44 > len(data) {
			return nil, fmt.Errorf("short ext header")
		}
		frames := int(binary.BigEndian.Uint32(data[hdr+32 : hdr+36]))
		rate := binary.BigEndian.Uint32(data[hdr+8:hdr+12]) >> 16
		chans := binary.BigEndian.Uint32(data[hdr+24 : hdr+28])
		bits := binary.BigEndian.Uint16(data[hdr+28 : hdr+30])
		start := hdr + 44
		bytesPerSample := int(bits) / 8
		length := frames * int(chans) * bytesPerSample
		if start > len(data) {
			return nil, fmt.Errorf("data out of range")
		}
		if length > len(data)-start {
			fmt.Printf("truncated sound data")
			if id != 0 {
				fmt.Printf(" for id %d", id)
			}
			fmt.Printf(": have %d bytes, expected %d\n", len(data)-start, length)
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
		return nil, fmt.Errorf("unsupported encode %d", encode)
	}
}
