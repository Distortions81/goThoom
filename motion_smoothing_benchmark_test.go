package main

import "testing"

func BenchmarkPictureMobileOffsetDense(b *testing.B) {
	mobiles := make([]frameMobile, 64)
	prevMobiles := make(map[uint8]frameMobile, len(mobiles))
	for i := range mobiles {
		m := frameMobile{Index: uint8(i), H: int16(240 + i%8), V: int16(240 + i/8)}
		mobiles[i] = m
		m.H--
		prevMobiles[m.Index] = m
	}
	prevPicturePositions := make(map[picturePositionKey]struct{}, 256)
	for i := 0; i < 256; i++ {
		prevPicturePositions[picturePositionKey{pictID: 100, h: int16(i), v: int16(-i)}] = struct{}{}
	}
	p := framePicture{PictID: 100, H: 256, V: 256}

	origPlayerIndex := playerIndex
	playerIndex = 255
	b.Cleanup(func() { playerIndex = origPlayerIndex })
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pictureMobileOffset(p, mobiles, prevMobiles, prevPicturePositions, 0.5)
	}
}
