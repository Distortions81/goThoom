package climg

import (
	"bytes"
	"encoding/binary"
)

func simpleEncrypt(data []byte) {
	key := []byte{0x3C, 0x5A, 0x69, 0x93, 0xA5, 0xC6}
	for i := range data {
		data[i] ^= key[i%len(key)]
	}
}

func doChecksum(data []byte, s1, s2 *uint32) {
	const base = 65521
	for _, b := range data {
		*s1 = (*s1 + uint32(b)) % base
		*s2 = (*s2 + *s1) % base
	}
}

func calculateChecksum(bits, colors, light []byte, ref *dataLocation) uint32 {
	s1 := uint32(1)
	s2 := uint32(0)
	if bits == nil || colors == nil || ref == nil {
		return 0
	}
	doChecksum(bits, &s1, &s2)
	doChecksum(colors, &s1, &s2)
	if light != nil {
		doChecksum(light, &s1, &s2)
	}
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.BigEndian, ref.version)
	binary.Write(buf, binary.BigEndian, ref.imageID)
	binary.Write(buf, binary.BigEndian, ref.colorID)
	binary.Write(buf, binary.BigEndian, uint32(0))
	binary.Write(buf, binary.BigEndian, ref.flags)
	binary.Write(buf, binary.BigEndian, ref.unusedFlags)
	binary.Write(buf, binary.BigEndian, ref.unusedFlags2)
	binary.Write(buf, binary.BigEndian, ref.lightingID)
	binary.Write(buf, binary.BigEndian, ref.plane)
	binary.Write(buf, binary.BigEndian, ref.numFrames)
	binary.Write(buf, binary.BigEndian, ref.numAnims)
	n := int(ref.numAnims)
	if n > len(ref.animFrameTable) {
		n = len(ref.animFrameTable)
	}
	for i := 0; i < n; i++ {
		binary.Write(buf, binary.BigEndian, ref.animFrameTable[i])
	}
	doChecksum(buf.Bytes(), &s1, &s2)
	idbuf := make([]byte, 4)
	binary.BigEndian.PutUint32(idbuf, ref.id)
	doChecksum(idbuf, &s1, &s2)
	sum := (s2 << 16) + (s1 & 0xFFFF)
	tmp := make([]byte, 4)
	binary.BigEndian.PutUint32(tmp, sum)
	simpleEncrypt(tmp)
	return binary.BigEndian.Uint32(tmp)
}
