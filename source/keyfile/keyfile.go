package keyfile

import (
	"encoding/binary"
	"fmt"
)

// Entry represents a single record in a keyfile.
type Entry struct {
	Type uint32
	ID   uint32
	Data []byte
}

// parse reads keyfile data and returns all entries.
func parse(data []byte) ([]Entry, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("short header")
	}
	if binary.BigEndian.Uint16(data[0:2]) != 0xffff {
		return nil, fmt.Errorf("bad header")
	}
	n := int(binary.BigEndian.Uint32(data[2:6]))
	table := data[12:]
	if len(table) < n*16 {
		return nil, fmt.Errorf("short table")
	}
	entries := make([]Entry, 0, n)
	for i := 0; i < n; i++ {
		off := binary.BigEndian.Uint32(table[0:4])
		size := binary.BigEndian.Uint32(table[4:8])
		typ := binary.BigEndian.Uint32(table[8:12])
		id := binary.BigEndian.Uint32(table[12:16])
		if int(off+size) > len(data) {
			return nil, fmt.Errorf("entry %d out of range", i)
		}
		// copy data slice so modifications do not affect original
		b := make([]byte, size)
		copy(b, data[off:off+size])
		entries = append(entries, Entry{Type: typ, ID: id, Data: b})
		table = table[16:]
	}
	return entries, nil
}

// Build assembles a keyfile from the provided entries.
func Build(entries []Entry) []byte {
	n := len(entries)
	header := make([]byte, 12+16*n)
	binary.BigEndian.PutUint16(header[0:2], 0xffff)
	binary.BigEndian.PutUint32(header[2:6], uint32(n))
	// pad1 and pad2 are zero
	off := uint32(12 + 16*n)
	for i, e := range entries {
		binary.BigEndian.PutUint32(header[12+16*i:12+16*i+4], off)
		binary.BigEndian.PutUint32(header[12+16*i+4:12+16*i+8], uint32(len(e.Data)))
		binary.BigEndian.PutUint32(header[12+16*i+8:12+16*i+12], e.Type)
		binary.BigEndian.PutUint32(header[12+16*i+12:12+16*i+16], e.ID)
		off += uint32(len(e.Data))
	}
	buf := append(header, make([]byte, 0, off-uint32(len(header)))...)
	for _, e := range entries {
		buf = append(buf, e.Data...)
	}
	return buf
}

// Merge overlays patch entries onto base and returns the merged keyfile.
func Merge(base, patch []byte) ([]byte, error) {
	baseEntries, err := parse(base)
	if err != nil {
		return nil, err
	}
	patchEntries, err := parse(patch)
	if err != nil {
		return nil, err
	}
	type key struct{ t, id uint32 }
	m := make(map[key]Entry, len(baseEntries)+len(patchEntries))
	for _, e := range baseEntries {
		m[key{e.Type, e.ID}] = e
	}
	for _, e := range patchEntries {
		m[key{e.Type, e.ID}] = e
	}
	// maintain base order, patch overriding; append new patch entries
	final := make([]Entry, 0, len(m))
	for _, e := range baseEntries {
		k := key{e.Type, e.ID}
		if ne, ok := m[k]; ok {
			final = append(final, ne)
			delete(m, k)
		}
	}
	for _, e := range patchEntries {
		k := key{e.Type, e.ID}
		if ne, ok := m[k]; ok {
			final = append(final, ne)
			delete(m, k)
		}
	}
	return Build(final), nil
}
