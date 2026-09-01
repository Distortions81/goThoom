package keyfile

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

const testTypeSound = 0x736e6420 // 'snd '

func testVersionEntry(major uint32) Entry {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, major<<8)
	return Entry{Type: typeVersion, ID: 0, Data: data}
}

func TestBuildWritesSortedClassicVersion2Keyfile(t *testing.T) {
	data := Build([]Entry{
		{Type: testTypeSound, ID: 20, Data: []byte("twenty")},
		testVersionEntry(2),
		{Type: testTypeSound, ID: 10, Data: []byte("ten")},
	})

	if got := binary.BigEndian.Uint16(data[0:2]); got != 0xffff {
		t.Fatalf("legacy version marker = %#x, want 0xffff", got)
	}
	if got := binary.BigEndian.Uint32(data[6:10]); got != 0xffffffff {
		t.Fatalf("unused header field = %#x, want 0xffffffff", got)
	}
	if got := binary.BigEndian.Uint16(data[10:12]); got != 2 {
		t.Fatalf("keyfile version = %d, want 2", got)
	}

	entries, err := parse(data)
	if err != nil {
		t.Fatalf("parse built keyfile: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entry count = %d, want 3", len(entries))
	}
	if entries[0].Type != typeVersion || entries[1].ID != 10 || entries[2].ID != 20 {
		t.Fatalf("entries are not sorted by signed type and ID: %#v", entries)
	}
	if !bytes.Equal(entries[1].Data, []byte("ten")) || !bytes.Equal(entries[2].Data, []byte("twenty")) {
		t.Fatalf("entry payloads changed: %#v", entries)
	}
}

func TestMergeOnlyMatchesClassicPatchAndCleanup(t *testing.T) {
	const obsoleteType = 0x4f6c6421 // 'Old!'
	base := Build([]Entry{
		testVersionEntry(1),
		{Type: testTypeSound, ID: 10, Data: []byte("old")},
		{Type: testTypeSound, ID: 20, Data: []byte("keep")},
		{Type: obsoleteType, ID: 0, Data: []byte("remove")},
	})
	patch := Build([]Entry{
		testVersionEntry(2),
		{Type: testTypeSound, ID: 10, Data: []byte("new")},
		{Type: testTypeSound, ID: 30, Data: []byte("add")},
	})

	merged, err := MergeOnly(base, patch, typeVersion, testTypeSound)
	if err != nil {
		t.Fatalf("merge patch: %v", err)
	}
	entries, err := parse(merged)
	if err != nil {
		t.Fatalf("parse merged keyfile: %v", err)
	}
	want := []Entry{
		testVersionEntry(2),
		{Type: testTypeSound, ID: 10, Data: []byte("new")},
		{Type: testTypeSound, ID: 20, Data: []byte("keep")},
		{Type: testTypeSound, ID: 30, Data: []byte("add")},
	}
	if len(entries) != len(want) {
		t.Fatalf("merged entries = %#v, want %#v", entries, want)
	}
	for i := range want {
		if entries[i].Type != want[i].Type || entries[i].ID != want[i].ID || !bytes.Equal(entries[i].Data, want[i].Data) {
			t.Fatalf("merged entry %d = %#v, want %#v", i, entries[i], want[i])
		}
	}
}

func TestApplyPatchRejectsWrongTargetWithoutChangingBase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CL_Sounds")
	base := Build([]Entry{testVersionEntry(1)})
	if err := os.WriteFile(path, base, 0o640); err != nil {
		t.Fatal(err)
	}
	patch := Build([]Entry{testVersionEntry(2)})

	err := ApplyPatch(path, patch, 3, typeVersion, testTypeSound)
	if err == nil {
		t.Fatal("wrong target version unexpectedly succeeded")
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, base) {
		t.Fatal("base changed after rejected patch")
	}
	info, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if gotMode := info.Mode().Perm(); gotMode != 0o640 {
		t.Fatalf("base mode = %v, want 0640", gotMode)
	}
}

func TestMergeRejectsPatchWithoutVersion(t *testing.T) {
	base := Build([]Entry{testVersionEntry(1)})
	patch := Build([]Entry{{Type: testTypeSound, ID: 1, Data: []byte("sound")}})
	if _, err := Merge(base, patch); err == nil {
		t.Fatal("patch without Vers/0 unexpectedly succeeded")
	}
}
