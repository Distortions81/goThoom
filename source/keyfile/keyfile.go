package keyfile

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
)

const typeVersion = 0x56657273 // 'Vers'

// Entry represents a single record in a keyfile.
type Entry struct {
	Type uint32
	ID   uint32
	Data []byte
}

type entryKey struct {
	typeID uint32
	id     uint32
}

// parse reads a version 1 or 2 keyfile and returns all entries.
func parse(data []byte) ([]Entry, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("short header")
	}
	if binary.BigEndian.Uint16(data[0:2]) != 0xffff {
		return nil, fmt.Errorf("bad header")
	}
	version := binary.BigEndian.Uint16(data[10:12])
	if version != 1 && version != 2 {
		return nil, fmt.Errorf("unsupported keyfile version %d", version)
	}

	n := uint64(binary.BigEndian.Uint32(data[2:6]))
	tableEnd := uint64(12) + n*16
	if tableEnd > uint64(len(data)) {
		return nil, fmt.Errorf("short table")
	}

	entries := make([]Entry, 0, int(n))
	seen := make(map[entryKey]struct{}, int(n))
	for i := uint64(0); i < n; i++ {
		tableOffset := uint64(12) + i*16
		table := data[tableOffset : tableOffset+16]
		offset := uint64(binary.BigEndian.Uint32(table[0:4]))
		size := uint64(binary.BigEndian.Uint32(table[4:8]))
		typeID := binary.BigEndian.Uint32(table[8:12])
		id := binary.BigEndian.Uint32(table[12:16])
		if offset < tableEnd || offset+size > uint64(len(data)) {
			return nil, fmt.Errorf("entry %d out of range", i)
		}
		key := entryKey{typeID: typeID, id: id}
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("duplicate entry type %#08x id %d", typeID, id)
		}
		seen[key] = struct{}{}
		entryData := append([]byte(nil), data[offset:offset+size]...)
		entries = append(entries, Entry{Type: typeID, ID: id, Data: entryData})
	}
	return entries, nil
}

// Build assembles a compact version 2 keyfile from the provided entries.
func Build(entries []Entry) []byte {
	entries = append([]Entry(nil), entries...)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return int32(entries[i].Type) < int32(entries[j].Type)
		}
		return int32(entries[i].ID) < int32(entries[j].ID)
	})

	n := len(entries)
	header := make([]byte, 12+16*n)
	binary.BigEndian.PutUint16(header[0:2], 0xffff)
	binary.BigEndian.PutUint32(header[2:6], uint32(n))
	binary.BigEndian.PutUint32(header[6:10], 0xffffffff)
	binary.BigEndian.PutUint16(header[10:12], 2)
	offset := uint32(len(header))
	for i, entry := range entries {
		tableOffset := 12 + 16*i
		binary.BigEndian.PutUint32(header[tableOffset:tableOffset+4], offset)
		binary.BigEndian.PutUint32(header[tableOffset+4:tableOffset+8], uint32(len(entry.Data)))
		binary.BigEndian.PutUint32(header[tableOffset+8:tableOffset+12], entry.Type)
		binary.BigEndian.PutUint32(header[tableOffset+12:tableOffset+16], entry.ID)
		offset += uint32(len(entry.Data))
	}
	result := append([]byte(nil), header...)
	for _, entry := range entries {
		result = append(result, entry.Data...)
	}
	return result
}

// Merge overlays patch entries onto base and returns a compact version 2
// keyfile. The result is sorted as required by the classic keyfile library.
func Merge(base, patch []byte) ([]byte, error) {
	return merge(base, patch, nil)
}

// MergeOnly behaves like Merge, then removes record types that are not in the
// allowed list. The classic image and sound patchers perform this cleanup.
func MergeOnly(base, patch []byte, allowed ...uint32) ([]byte, error) {
	keep := make(map[uint32]struct{}, len(allowed))
	for _, typeID := range allowed {
		keep[typeID] = struct{}{}
	}
	return merge(base, patch, keep)
}

func merge(base, patch []byte, allowed map[uint32]struct{}) ([]byte, error) {
	baseEntries, err := parse(base)
	if err != nil {
		return nil, fmt.Errorf("parse base: %w", err)
	}
	patchEntries, err := parse(patch)
	if err != nil {
		return nil, fmt.Errorf("parse patch: %w", err)
	}
	if _, err := version(patchEntries); err != nil {
		return nil, fmt.Errorf("patch version: %w", err)
	}

	merged := make(map[entryKey]Entry, len(baseEntries)+len(patchEntries))
	for _, entry := range baseEntries {
		if allowed == nil {
			merged[entryKey{typeID: entry.Type, id: entry.ID}] = entry
		} else if _, ok := allowed[entry.Type]; ok {
			merged[entryKey{typeID: entry.Type, id: entry.ID}] = entry
		}
	}
	// The classic client defers Vers/0 until every other patch record succeeds.
	// Building in memory already makes the operation transactional, but retain
	// that ordering here so a malformed version record never enters the result.
	var versionEntry Entry
	for _, entry := range patchEntries {
		if entry.Type == typeVersion && entry.ID == 0 {
			versionEntry = entry
			continue
		}
		if allowed == nil {
			merged[entryKey{typeID: entry.Type, id: entry.ID}] = entry
		} else if _, ok := allowed[entry.Type]; ok {
			merged[entryKey{typeID: entry.Type, id: entry.ID}] = entry
		}
	}
	merged[entryKey{typeID: typeVersion, id: 0}] = versionEntry

	entries := make([]Entry, 0, len(merged))
	for _, entry := range merged {
		entries = append(entries, entry)
	}
	return Build(entries), nil
}

func version(entries []Entry) (uint32, error) {
	for _, entry := range entries {
		if entry.Type != typeVersion || entry.ID != 0 {
			continue
		}
		if len(entry.Data) != 4 {
			return 0, fmt.Errorf("Vers/0 has size %d, want 4", len(entry.Data))
		}
		value := binary.BigEndian.Uint32(entry.Data)
		if value <= 0xff {
			value <<= 8
		}
		return value, nil
	}
	return 0, fmt.Errorf("Vers/0 is missing")
}

// ApplyPatch merges patch into the keyfile at basePath, verifies the patch's
// destination version, and atomically replaces the original file.
func ApplyPatch(basePath string, patch []byte, expectedMajor uint32, allowed ...uint32) error {
	return ApplyPatchValidated(basePath, patch, expectedMajor, nil, allowed...)
}

// ApplyPatchValidated behaves like ApplyPatch and runs validate on the merged
// bytes before anything on disk is replaced.
func ApplyPatchValidated(basePath string, patch []byte, expectedMajor uint32, validate func([]byte) error, allowed ...uint32) error {
	patchEntries, err := parse(patch)
	if err != nil {
		return fmt.Errorf("parse patch: %w", err)
	}
	patchVersion, err := version(patchEntries)
	if err != nil {
		return fmt.Errorf("patch version: %w", err)
	}
	if patchVersion>>8 != expectedMajor {
		return fmt.Errorf("patch version %d, want %d", patchVersion>>8, expectedMajor)
	}

	info, err := os.Stat(basePath)
	if err != nil {
		return err
	}
	base, err := os.ReadFile(basePath)
	if err != nil {
		return err
	}
	merged, err := MergeOnly(base, patch, allowed...)
	if err != nil {
		return err
	}
	if validate != nil {
		if err := validate(merged); err != nil {
			return fmt.Errorf("validate patched keyfile: %w", err)
		}
	}

	temporary, err := os.CreateTemp(filepath.Dir(basePath), "."+filepath.Base(basePath)+".*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(merged); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Seek(0, io.SeekStart); err != nil {
		_ = temporary.Close()
		return err
	}
	writtenCRC := crc32.NewIEEE()
	if _, err := io.Copy(writtenCRC, temporary); err != nil {
		_ = temporary.Close()
		return err
	}
	if got, want := writtenCRC.Sum32(), crc32.ChecksumIEEE(merged); got != want {
		_ = temporary.Close()
		return fmt.Errorf("patched keyfile CRC32 %#08x, want %#08x", got, want)
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, basePath)
}
