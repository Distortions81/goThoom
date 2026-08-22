package climg

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
)

// AlphaMask represents a quarter-resolution 1-bit alpha mask where each mask
// pixel covers a 4x4 block of the original image.
type AlphaMask struct {
	OrigW int
	OrigH int
	W     int
	H     int
	Bits  []uint64
}

// Opaque reports whether the mask has an opaque pixel at the given mask
// coordinates.
func (m *AlphaMask) Opaque(x, y int) bool {
	if m == nil || x < 0 || y < 0 || x >= m.W || y >= m.H {
		return false
	}
	idx := y*m.W + x
	return (m.Bits[idx/64]>>(idx%64))&1 != 0
}

// AlphaMaskQuarter returns a quarter-resolution 1-bit alpha mask for the given
// image ID without reading from GPU caches. When forceTransparent is true,
// palette index 0 is treated as fully transparent regardless of sprite flags.
func (c *CLImages) AlphaMaskQuarter(id uint32, forceTransparent bool) *AlphaMask {
	key := fmt.Sprintf("%d-%t", id, forceTransparent)
	c.mu.Lock()
	if m, ok := c.masks[key]; ok {
		c.mu.Unlock()
		return m
	}
	c.mu.Unlock()

	ref := c.idrefs[id]
	if ref == nil {
		return nil
	}
	imgLoc := c.images[ref.imageID]
	if imgLoc == nil {
		return nil
	}

	r := bytes.NewReader(c.data)
	if _, err := r.Seek(int64(imgLoc.offset), io.SeekStart); err != nil {
		log.Printf("seek image %d: %v", id, err)
		return nil
	}
	var h, w uint16
	var pad uint32
	var v, b byte
	if err := binary.Read(r, binary.BigEndian, &h); err != nil {
		log.Printf("read h for %d: %v", id, err)
		return nil
	}
	if err := binary.Read(r, binary.BigEndian, &w); err != nil {
		log.Printf("read w for %d: %v", id, err)
		return nil
	}
	if err := binary.Read(r, binary.BigEndian, &pad); err != nil {
		log.Printf("read pad for %d: %v", id, err)
		return nil
	}
	if err := binary.Read(r, binary.BigEndian, &v); err != nil {
		log.Printf("read v for %d: %v", id, err)
		return nil
	}
	if err := binary.Read(r, binary.BigEndian, &b); err != nil {
		log.Printf("read b for %d: %v", id, err)
		return nil
	}

	width := int(w)
	height := int(h)
	valueW := int(v)
	blockLenW := int(b)
	pixelCount := width * height
	br := New(r)
	data := make([]byte, pixelCount)
	pixPos := 0
	for pixPos < pixelCount {
		t, err := br.ReadBit()
		if err != nil {
			log.Printf("read bit for %d: %v", id, err)
			return nil
		}
		s, err := br.ReadInt(blockLenW)
		if err != nil {
			log.Printf("read int for %d: %v", id, err)
			return nil
		}
		s++
		if t {
			for i := 0; i < s && pixPos < pixelCount; i++ {
				val, err := br.ReadBits(valueW)
				if err != nil {
					log.Printf("read bits for %d: %v", id, err)
					return nil
				}
				data[pixPos] = val
				pixPos++
			}
		} else {
			val, err := br.ReadBits(valueW)
			if err != nil {
				log.Printf("read bits for %d: %v", id, err)
				return nil
			}
			for i := 0; i < s && pixPos < pixelCount; i++ {
				data[pixPos] = val
				pixPos++
			}
		}
	}

	if ref.flags&pictDefCustomColors != 0 && len(data) >= width {
		data = data[width:]
		height--
	}

	// Always treat palette index 0 as transparent for mask purposes.

	qW := (width + 3) / 4
	qH := (height + 3) / 4
	bits := make([]uint64, (qW*qH+63)/64)
	for by := 0; by < qH; by++ {
		for bx := 0; bx < qW; bx++ {
			opaque := false
			for y := 0; y < 4 && !opaque; y++ {
				py := by*4 + y
				if py >= height {
					break
				}
				row := py * width
				for x := 0; x < 4; x++ {
					px := bx*4 + x
					if px >= width {
						break
					}
					idx := data[row+px]
					if idx != 0 {
						opaque = true
						break
					}
				}
			}
			if opaque {
				bit := by*qW + bx
				bits[bit/64] |= 1 << (bit % 64)
			}
		}
	}

	m := &AlphaMask{OrigW: width, OrigH: height, W: qW, H: qH, Bits: bits}
	c.mu.Lock()
	if c.masks == nil {
		c.masks = make(map[string]*AlphaMask)
	}
	c.masks[key] = m
	c.mu.Unlock()
	return m
}
