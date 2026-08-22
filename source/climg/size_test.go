package climg

import (
	"encoding/binary"
	"testing"
)

func TestSizeCachesImageHeader(t *testing.T) {
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[:2], 80)
	binary.BigEndian.PutUint16(data[2:], 160)
	imageLoc := &dataLocation{}
	images := &CLImages{
		data:   data,
		idrefs: map[uint32]*dataLocation{1: {imageID: 2}},
		images: map[uint32]*dataLocation{2: imageLoc},
	}

	if w, h := images.Size(1); w != 160 || h != 80 {
		t.Fatalf("Size(1) = %dx%d, want 160x80", w, h)
	}
	// CLImages data is immutable in production. Mutating this test buffer
	// verifies subsequent calls use the cached header rather than reparsing it.
	clear(data)
	if w, h := images.Size(1); w != 160 || h != 80 {
		t.Fatalf("cached Size(1) = %dx%d, want 160x80", w, h)
	}
}

func BenchmarkSizeCached(b *testing.B) {
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[:2], 80)
	binary.BigEndian.PutUint16(data[2:], 160)
	images := &CLImages{
		data:   data,
		idrefs: map[uint32]*dataLocation{1: {imageID: 2}},
		images: map[uint32]*dataLocation{2: {}},
	}
	images.Size(1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		images.Size(1)
	}
}
