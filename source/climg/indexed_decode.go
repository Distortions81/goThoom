package climg

import (
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"
	"sync"
)

const indexedImageHeaderSize = 10

const (
	maxIndexedPixelPoolBytes  = 8 << 20
	maxIndexedPixelBufferSize = 1 << 20
	maxIndexedPixelBuffers    = 16
)

var indexedPixelPool = struct {
	sync.Mutex
	buckets [21][][]byte
	bytes   int
}{}

func indexedPixelBucket(size int) (int, int) {
	if size <= 0 || size > maxIndexedPixelBufferSize {
		return 0, 0
	}
	bucket := bits.Len(uint(size - 1))
	return bucket, 1 << bucket
}

func acquireIndexedPixels(size int) []byte {
	bucket, capacity := indexedPixelBucket(size)
	if capacity == 0 {
		return make([]byte, size)
	}
	indexedPixelPool.Lock()
	pooled := indexedPixelPool.buckets[bucket]
	if len(pooled) != 0 {
		pixels := pooled[len(pooled)-1]
		indexedPixelPool.buckets[bucket] = pooled[:len(pooled)-1]
		indexedPixelPool.bytes -= cap(pixels)
		indexedPixelPool.Unlock()
		return pixels[:size]
	}
	indexedPixelPool.Unlock()
	return make([]byte, size, capacity)
}

func releaseIndexedPixels(pixels []byte) {
	bucket, capacity := indexedPixelBucket(cap(pixels))
	if capacity == 0 || capacity != cap(pixels) {
		return
	}
	indexedPixelPool.Lock()
	if indexedPixelPool.bytes+capacity <= maxIndexedPixelPoolBytes && len(indexedPixelPool.buckets[bucket]) < maxIndexedPixelBuffers {
		indexedPixelPool.buckets[bucket] = append(indexedPixelPool.buckets[bucket], pixels[:capacity])
		indexedPixelPool.bytes += capacity
	}
	indexedPixelPool.Unlock()
}

// sliceBitReader reads the CL_Images bitstream MSB-first. Reading a field a
// byte at a time avoids the interface call and per-bit loop used by BitReader.
type sliceBitReader struct {
	data     []byte
	nextByte int
	buffer   uint64
	buffered uint
	bitPos   int
}

func (r *sliceBitReader) readBits(count int) (uint32, bool) {
	if count < 0 || count > 32 || count > len(r.data)*8-r.bitPos {
		return 0, false
	}
	for r.buffered < uint(count) {
		if r.nextByte+4 <= len(r.data) {
			r.buffer = r.buffer<<32 | uint64(binary.BigEndian.Uint32(r.data[r.nextByte:r.nextByte+4]))
			r.nextByte += 4
			r.buffered += 32
			continue
		}
		if r.nextByte >= len(r.data) {
			return 0, false
		}
		r.buffer = r.buffer<<8 | uint64(r.data[r.nextByte])
		r.nextByte++
		r.buffered += 8
	}
	r.buffered -= uint(count)
	mask := uint64(1)<<uint(count) - 1
	result := uint32(r.buffer>>r.buffered) & uint32(mask)
	if r.buffered == 0 {
		r.buffer = 0
	} else {
		r.buffer &= uint64(1)<<r.buffered - 1
	}
	r.bitPos += count
	return result, true
}

// decodeIndexedImage expands the run-length encoded palette indexes stored in
// one CL_Images image resource. It intentionally does not interpret colors.
func decodeIndexedImage(archive []byte, loc *dataLocation) ([]byte, int, int, error) {
	if loc == nil {
		return nil, 0, 0, io.ErrUnexpectedEOF
	}
	start64 := uint64(loc.offset)
	end64 := start64 + uint64(loc.size)
	if end64 > uint64(len(archive)) || end64 < start64 || end64-start64 < indexedImageHeaderSize {
		return nil, 0, 0, io.ErrUnexpectedEOF
	}
	start, end := int(start64), int(end64)
	resource := archive[start:end]
	height := int(binary.BigEndian.Uint16(resource[0:2]))
	width := int(binary.BigEndian.Uint16(resource[2:4]))
	if width <= 0 || height <= 0 || height > int(^uint(0)>>1)/width {
		return nil, 0, 0, fmt.Errorf("invalid indexed image size %dx%d", width, height)
	}
	valueWidth := int(resource[8])
	blockLengthWidth := int(resource[9])
	if valueWidth > 8 || blockLengthWidth > 31 {
		return nil, 0, 0, fmt.Errorf("invalid indexed image fields %d/%d", valueWidth, blockLengthWidth)
	}

	pixels := acquireIndexedPixels(width * height)
	keepPixels := false
	defer func() {
		if !keepPixels {
			releaseIndexedPixels(pixels)
		}
	}()
	reader := sliceBitReader{data: resource[indexedImageHeaderSize:]}
	position := 0
	for position < len(pixels) {
		literal, ok := reader.readBits(1)
		if !ok {
			return nil, 0, 0, io.ErrUnexpectedEOF
		}
		runValue, ok := reader.readBits(blockLengthWidth)
		if !ok {
			return nil, 0, 0, io.ErrUnexpectedEOF
		}
		runLength := int(runValue) + 1
		endPosition := len(pixels)
		if runLength < len(pixels)-position {
			endPosition = position + runLength
		}
		if literal != 0 {
			for position < endPosition {
				value, ok := reader.readBits(valueWidth)
				if !ok {
					return nil, 0, 0, io.ErrUnexpectedEOF
				}
				pixels[position] = byte(value)
				position++
			}
			continue
		}
		value, ok := reader.readBits(valueWidth)
		if !ok {
			return nil, 0, 0, io.ErrUnexpectedEOF
		}
		repeated := uint64(byte(value)) * 0x0101010101010101
		for position+8 <= endPosition {
			binary.LittleEndian.PutUint64(pixels[position:position+8], repeated)
			position += 8
		}
		for position < endPosition {
			pixels[position] = byte(value)
			position++
		}
	}
	keepPixels = true
	return pixels, width, height, nil
}
