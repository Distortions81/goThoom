package main

import (
	"slices"
	"testing"
)

func TestRenderCachePicturePartitionsPreserveOrderAndOwnership(t *testing.T) {
	session := mustNewSession(2)
	for _, pictures := range [][]framePicture{
		{{PictID: 1, Plane: 1}, {PictID: 2, Plane: -1, V: 3}, {PictID: 3}, {PictID: 4, Plane: -1, V: -4}, {PictID: 5, Plane: 2}},
		{{PictID: 6, Plane: -2}},
		{{PictID: 7}, {PictID: 8, H: 1}},
		{{PictID: 9, Plane: 3}},
		nil,
		{{PictID: 10, Plane: -1}, {PictID: 11}, {PictID: 12, Plane: 1}},
	} {
		original := slices.Clone(pictures)
		session.draw.current.pictures = pictures
		prepareSessionRenderCacheLocked(session)
		current := &session.draw.current
		if !slices.Equal(current.pictures, original) {
			t.Fatal("preparing picture partitions changed canonical draw order")
		}
		sorted := slices.Clone(original)
		sortPictures(sorted)
		var expected [3][]framePicture
		for _, picture := range sorted {
			plane := 1
			if picture.Plane < 0 {
				plane = 0
			} else if picture.Plane > 0 {
				plane = 2
			}
			expected[plane] = append(expected[plane], picture)
		}
		partitions := [...][]framePicture{current.picsNeg, current.picsZero, current.picsPos}
		for plane, partition := range partitions {
			if !slices.Equal(partition, expected[plane]) {
				t.Fatalf("partition %d = %+v, want %+v", plane, partition, expected[plane])
			}
			_ = append(partition, framePicture{PictID: 60000})
		}
		if !slices.Equal(current.sortedPics, sorted) {
			t.Fatal("appending to a plane overwrote another picture partition")
		}
		var snapshot drawSnapshot
		captureSessionDrawSnapshot(session, &snapshot)
		// Reuse the cache with a different scene. A renderer's prior snapshot
		// must retain its original pictures after the network state changes.
		session.draw.current.pictures = []framePicture{{PictID: 100, Plane: -1}}
		prepareSessionRenderCacheLocked(session)
		for plane, partition := range [...][]framePicture{snapshot.picsNeg, snapshot.picsZero, snapshot.picsPos} {
			if !slices.Equal(partition, expected[plane]) {
				t.Fatalf("cache reuse changed snapshot partition %d", plane)
			}
		}
	}
}

func BenchmarkPreparePictureRenderCache(b *testing.B) {
	pictures := make([]framePicture, 512)
	for i := range pictures {
		pictures[i] = framePicture{PictID: uint16(i), Plane: i%3 - 1, H: int16(i%32) * 20, V: int16(i/32) * 20}
	}
	for _, cold := range []bool{false, true} {
		name := "Warm"
		if cold {
			name = "Cold"
		}
		b.Run(name, func(b *testing.B) {
			session := &Session{draw: &sessionDrawState{current: drawState{pictures: pictures}}}
			prepareSessionRenderCacheLocked(session)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if cold {
					session.draw.current = drawState{pictures: pictures}
				}
				prepareSessionRenderCacheLocked(session)
			}
		})
	}
}
