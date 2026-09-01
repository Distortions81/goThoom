package climg

import (
	"bytes"
	"encoding/binary"
	"image"
	"testing"
)

func TestSliceBitReaderMatchesBitReader(t *testing.T) {
	data := []byte{0xd3, 0x69, 0xa5, 0x7e, 0x10}
	widths := []int{1, 3, 0, 8, 7, 2, 11, 8}
	fast := sliceBitReader{data: data}
	legacy := New(bytes.NewReader(data))
	for _, width := range widths {
		got, ok := fast.readBits(width)
		if !ok {
			t.Fatalf("fast reader rejected %d bits at position %d", width, fast.bitPos)
		}
		want, err := legacy.ReadInt(width)
		if err != nil {
			t.Fatalf("legacy reader failed for %d bits: %v", width, err)
		}
		if got != uint32(want) {
			t.Fatalf("read %d bits = %#x, want %#x", width, got, want)
		}
	}
	if _, ok := fast.readBits(1); ok {
		t.Fatal("reader accepted data beyond the end of the slice")
	}
}

func TestDecodeIndexedImage(t *testing.T) {
	// 4x2, 3-bit palette values and 2-bit run lengths:
	// repeat 5 three times, literals 1/2/3/4, then repeat 7 once.
	payload := packTestBits("010101111001010011100000111")
	resource := make([]byte, indexedImageHeaderSize+len(payload))
	binary.BigEndian.PutUint16(resource[0:2], 2)
	binary.BigEndian.PutUint16(resource[2:4], 4)
	resource[8] = 3
	resource[9] = 2
	copy(resource[indexedImageHeaderSize:], payload)

	got, width, height, err := decodeIndexedImage(resource, &dataLocation{size: uint32(len(resource))})
	if err != nil {
		t.Fatal(err)
	}
	defer releaseIndexedPixels(got)
	if width != 4 || height != 2 {
		t.Fatalf("decoded size = %dx%d, want 4x2", width, height)
	}
	want := []byte{5, 5, 5, 1, 2, 3, 4, 7}
	if !bytes.Equal(got, want) {
		t.Fatalf("decoded indexes = %v, want %v", got, want)
	}
}

func TestIndexedPixelPoolReusesSizeBucket(t *testing.T) {
	indexedPixelPool.Lock()
	original := indexedPixelPool.buckets
	originalBytes := indexedPixelPool.bytes
	indexedPixelPool.buckets = [21][][]byte{}
	indexedPixelPool.bytes = 0
	indexedPixelPool.Unlock()
	t.Cleanup(func() {
		indexedPixelPool.Lock()
		indexedPixelPool.buckets = original
		indexedPixelPool.bytes = originalBytes
		indexedPixelPool.Unlock()
	})

	first := acquireIndexedPixels(1000)
	firstPixel := &first[0]
	releaseIndexedPixels(first)
	second := acquireIndexedPixels(900)
	if &second[0] != firstPixel {
		t.Fatal("indexed pixel pool did not reuse its 1024-byte size bucket")
	}
	releaseIndexedPixels(second)
}

func TestWritePackedRGBAWritesPairsAndOddTail(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 5, 4))
	var table [256]uint32
	table[1] = 0x44332211
	table[2] = 0x88776655
	table[3] = 0xccbbaa99
	writePackedRGBA(img.Pix, img.Stride, []byte{1, 2, 3, 3, 2, 1}, 3, 2, &table)

	want := [][]byte{
		{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc},
		{0x99, 0xaa, 0xbb, 0xcc, 0x55, 0x66, 0x77, 0x88, 0x11, 0x22, 0x33, 0x44},
	}
	for y, rowWant := range want {
		offset := img.PixOffset(1, y+1)
		if got := img.Pix[offset : offset+len(rowWant)]; !bytes.Equal(got, rowWant) {
			t.Fatalf("row %d = % x, want % x", y, got, rowWant)
		}
	}
}

func packTestBits(bits string) []byte {
	result := make([]byte, (len(bits)+7)/8)
	for index, bit := range []byte(bits) {
		if bit == '1' {
			result[index/8] |= 0x80 >> (index % 8)
		}
	}
	return result
}
