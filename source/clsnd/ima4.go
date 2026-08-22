package clsnd

import (
	"encoding/binary"
	"fmt"
)

var imaIndexTable = [...]int{-1, -1, -1, -1, 2, 4, 6, 8, -1, -1, -1, -1, 2, 4, 6, 8}
var imaStepTable = [...]int{7, 8, 9, 10, 11, 12, 13, 14, 16, 17, 19, 21, 23, 25, 28, 31, 34, 37, 41, 45, 50, 55, 60, 66, 73, 80,
	88, 97, 107, 118, 130, 143, 157, 173, 190, 209, 230, 253, 279, 307, 337, 371, 408, 449, 494, 544, 598, 658, 724, 796, 876, 963,
	1060, 1166, 1282, 1411, 1552, 1707, 1878, 2066, 2272, 2499, 2749, 3024, 3327, 3660, 4026, 4428, 4871, 5358, 5894, 6484, 7132, 7845, 8630, 9493, 10442, 11487, 12635, 13899, 15289, 16818, 18500, 20350, 22385, 24623, 27086, 29794, 32767}

type ima4Entry struct {
	delta     int16
	nextIndex uint8
}

var ima4Table [89][16]ima4Entry

func init() {
	for i := range ima4Table {
		step := imaStepTable[i]
		for n := 0; n < 16; n++ {
			diff := step >> 3
			if n&1 != 0 {
				diff += step >> 2
			}
			if n&2 != 0 {
				diff += step >> 1
			}
			if n&4 != 0 {
				diff += step
			}
			if n&8 != 0 {
				diff = -diff
			}
			delta := int16(diff)
			idx := i + imaIndexTable[n]
			if idx < 0 {
				idx = 0
			} else if idx > 88 {
				idx = 88
			}
			ima4Table[i][n] = ima4Entry{delta: delta, nextIndex: uint8(idx)}
		}
	}
}

func decodeIMA4(data []byte, chans int) ([]byte, error) {
	if chans <= 0 {
		return nil, fmt.Errorf("invalid channel count")
	}
	blockSize := 36 * chans
	if len(data)%blockSize != 0 {
		return nil, fmt.Errorf("truncated ima4 data")
	}
	blocks := len(data) / blockSize
	pcm := make([]byte, blocks*64*chans*2)
	for b := 0; b < blocks; b++ {
		for ch := 0; ch < chans; ch++ {
			block := data[b*blockSize+ch*36 : b*blockSize+(ch+1)*36]
			pred := int16(binary.BigEndian.Uint16(block[0:2]))
			index := int(block[2])
			if index < 0 {
				index = 0
			} else if index > 88 {
				index = 88
			}
			p := b*64*chans + ch
			for i := 0; i < 32; i++ {
				by := block[4+i]

				e := ima4Table[index][by>>4]
				pred += e.delta
				index = int(e.nextIndex)
				binary.BigEndian.PutUint16(pcm[2*p:], uint16(pred))
				p += chans

				e = ima4Table[index][by&0x0F]
				pred += e.delta
				index = int(e.nextIndex)
				binary.BigEndian.PutUint16(pcm[2*p:], uint16(pred))
				p += chans
			}
		}
	}
	return pcm, nil
}
