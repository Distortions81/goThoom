package climg

import "log"

// AlphaMask represents a quarter-resolution 1-bit alpha mask where each mask
// pixel covers a 4x4 block of the original image.
type AlphaMask struct {
	OrigW int
	OrigH int
	W     int
	H     int
	Bits  []uint64
}

type alphaMaskKey struct {
	id               uint32
	forceTransparent bool
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
	key := alphaMaskKey{id: id, forceTransparent: forceTransparent}
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

	data, width, height, err := decodeIndexedImage(c.data, imgLoc)
	if err != nil {
		log.Printf("decode image %d: %v", id, err)
		return nil
	}
	indexedPixels := data
	defer releaseIndexedPixels(indexedPixels)

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
		c.masks = make(map[alphaMaskKey]*AlphaMask)
	}
	c.masks[key] = m
	c.mu.Unlock()
	return m
}
