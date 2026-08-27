package main

import (
	"encoding/binary"
	"sync"
	"testing"

	"gothoom/clsnd"
)

func TestAppendUniqueSoundPreservesDistinctIDs(t *testing.T) {
	var sounds []uint16
	for _, id := range []uint16{11, 22, 11, 33, 22} {
		sounds = appendUniqueSound(sounds, id)
	}
	want := []uint16{11, 22, 33}
	if len(sounds) != len(want) {
		t.Fatalf("unique sounds = %v, want %v", sounds, want)
	}
	for index := range want {
		if sounds[index] != want[index] {
			t.Fatalf("unique sounds = %v, want %v", sounds, want)
		}
	}
}

func TestMixLoadedSoundPlaybackPCMCombinesEverySound(t *testing.T) {
	first := monoPCM(1000, -1000)
	second := monoPCM(3000, 1000)
	mixed := mixLoadedSoundPlaybackPCM([][]byte{first, second}, false, 0)
	if len(mixed) != 8 {
		t.Fatalf("stereo mix bytes = %d, want 8", len(mixed))
	}
	want := []int16{2000, 2000, 0, 0}
	for index, sample := range want {
		got := int16(binary.LittleEndian.Uint16(mixed[index*2:]))
		if got != sample {
			t.Errorf("mixed sample %d = %d, want %d", index, got, sample)
		}
	}
}

func TestSoundWorkerCountsRemainBounded(t *testing.T) {
	for _, test := range []struct {
		numCPU   int
		effects  int
		precache int
	}{
		{numCPU: 1, effects: 1, precache: 1},
		{numCPU: 2, effects: 2, precache: 1},
		{numCPU: 8, effects: 8, precache: 2},
		{numCPU: 32, effects: 32, precache: 2},
	} {
		if got := soundEffectWorkerCount(test.numCPU); got != test.effects {
			t.Errorf("effect workers at NumCPU %d = %d, want %d", test.numCPU, got, test.effects)
		}
		if got := soundPrecacheWorkerCount(test.numCPU); got != test.precache {
			t.Errorf("precache workers at GOMAXPROCS %d = %d, want %d", test.numCPU, got, test.precache)
		}
	}
}

func TestSoundEffectQueueNeverBlocksWhenFull(t *testing.T) {
	queue := make(chan soundPlaybackRequest, 1)
	if !tryQueueSoundEffect(queue, soundPlaybackRequest{ids: []uint16{1}}) {
		t.Fatal("first sound was not queued")
	}
	if tryQueueSoundEffect(queue, soundPlaybackRequest{ids: []uint16{2}}) {
		t.Fatal("full sound queue accepted another effect")
	}
}

func TestNotificationSoundQueueNeverBlocksWhenFull(t *testing.T) {
	queue := make(chan notificationSoundRequest, 1)
	if !tryQueueNotificationSound(queue, notificationSoundRequest{key: 60}) {
		t.Fatal("first notification sound was not queued")
	}
	if tryQueueNotificationSound(queue, notificationSoundRequest{key: 61}) {
		t.Fatal("full notification sound queue accepted another effect")
	}
}

func TestConcurrentSoundDecodeSharesCachedPCM(t *testing.T) {
	archive, err := clsnd.LoadBytes(testCLSoundsArchive(77, []byte{128, 255, 0, 128}))
	if err != nil {
		t.Fatal(err)
	}
	originalArchive := currentCLSoundsArchive()
	originalPrecached := soundsPrecached.Load()
	replaceCLSoundsArchive(archive)
	t.Cleanup(func() {
		replaceCLSoundsArchive(originalArchive)
		soundsPrecached.Store(originalPrecached)
	})

	const callers = 16
	results := make([][]byte, callers)
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(callers)
	for index := range results {
		index := index
		go func() {
			defer workers.Done()
			<-start
			results[index] = loadSoundForPlayback(77, sampleRate, true)
		}()
	}
	close(start)
	workers.Wait()

	if len(results[0]) == 0 {
		t.Fatal("concurrent sound decode returned no PCM")
	}
	first := &results[0][0]
	for index, pcm := range results[1:] {
		if len(pcm) != len(results[0]) {
			t.Fatalf("caller %d PCM bytes = %d, want %d", index+1, len(pcm), len(results[0]))
		}
		if &pcm[0] != first {
			t.Fatalf("caller %d received a duplicate PCM allocation", index+1)
		}
	}
}

func monoPCM(samples ...int16) []byte {
	pcm := make([]byte, len(samples)*2)
	for index, sample := range samples {
		binary.LittleEndian.PutUint16(pcm[index*2:], uint16(sample))
	}
	return pcm
}

func testCLSoundsArchive(id uint32, samples []byte) []byte {
	const (
		headerSize = 12
		entrySize  = 16
		soundType  = uint32(0x736e6420)
	)
	resource := make([]byte, 14+22+len(samples))
	binary.BigEndian.PutUint16(resource[0:2], 1)
	binary.BigEndian.PutUint16(resource[4:6], 1)
	binary.BigEndian.PutUint16(resource[6:8], 0x8051)
	binary.BigEndian.PutUint32(resource[10:14], 14)
	header := resource[14:]
	binary.BigEndian.PutUint32(header[4:8], uint32(len(samples)))
	binary.BigEndian.PutUint32(header[8:12], 11025<<16)
	header[20] = 0
	copy(header[22:], samples)

	archive := make([]byte, headerSize+entrySize+len(resource))
	binary.BigEndian.PutUint16(archive[0:2], 0xffff)
	binary.BigEndian.PutUint32(archive[2:6], 1)
	entry := archive[headerSize : headerSize+entrySize]
	binary.BigEndian.PutUint32(entry[0:4], headerSize+entrySize)
	binary.BigEndian.PutUint32(entry[4:8], uint32(len(resource)))
	binary.BigEndian.PutUint32(entry[8:12], soundType)
	binary.BigEndian.PutUint32(entry[12:16], id)
	copy(archive[headerSize+entrySize:], resource)
	return archive
}

func BenchmarkMixLoadedSoundPlaybackPCM(b *testing.B) {
	sound := make([]int16, sampleRate)
	for index := range sound {
		sound[index] = int16(index%2000 - 1000)
	}
	pcm := monoPCM(sound...)
	sounds := [][]byte{pcm, pcm, pcm}
	b.ReportAllocs()
	b.SetBytes(int64(len(pcm) * len(sounds)))
	for b.Loop() {
		_ = mixLoadedSoundPlaybackPCM(sounds, false, 0)
	}
}
