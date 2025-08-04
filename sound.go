package main

import (
	"encoding/binary"
	"log"
	"math"
	"path/filepath"
	"sync"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"go_client/clsnd"
)

var (
	soundMu    sync.Mutex
	clSounds   *clsnd.CLSounds
	soundCache = make(map[uint16]*clsnd.Sound)

	audioContext *audio.Context
	soundPlayers = make(map[*audio.Player]struct{})

	// resample points to the resampling implementation to use.
	resample = resampleSinc

	playSound = func(id uint16) {
		//logError("sound: %v", id)
		s := loadSound(id)
		if s == nil || audioContext == nil {
			return
		}

		srcRate := int(s.SampleRate / 2)
		dstRate := audioContext.SampleRate()

		// Decode the sound data into 16-bit samples.
		var samples []int16
		switch s.Bits {
		case 8:
			samples = make([]int16, len(s.Data))
			for i, b := range s.Data {
				v := int16(b) - 0x80
				samples[i] = v << 8
			}
		case 16:
			if len(s.Data)%2 != 0 {
				return
			}
			samples = make([]int16, len(s.Data)/2)
			for i := 0; i < len(samples); i++ {
				samples[i] = int16(binary.BigEndian.Uint16(s.Data[2*i : 2*i+2]))
			}
		default:
			return
		}

		if srcRate != dstRate {
			samples = resample(samples, srcRate, dstRate)
		}

		pcm := make([]byte, len(samples)*2)
		for i, v := range samples {
			pcm[2*i] = byte(v)
			pcm[2*i+1] = byte(v >> 8)
		}

		p := audioContext.NewPlayerFromBytes(pcm)
		p.SetVolume(0.2)

		soundMu.Lock()
		for sp := range soundPlayers {
			if !sp.IsPlaying() {
				sp.Close()
				delete(soundPlayers, sp)
			}
		}
		if maxSounds > 0 && len(soundPlayers) >= maxSounds {
			soundMu.Unlock()
			p.Close()
			return
		}
		soundPlayers[p] = struct{}{}
		soundMu.Unlock()

		p.Play()
	}
)

// initSoundContext initializes the global audio context and resampler based on
// the fastSound flag.
func initSoundContext() {
	rate := 44100
	resample = resampleSinc
	if fastSound {
		rate = 22050
		resample = resampleLinear
	}
	audioContext = audio.NewContext(rate)
}

// resampleLinear resamples the given 16-bit samples from srcRate to dstRate
// using simple linear interpolation.
func resampleLinear(src []int16, srcRate, dstRate int) []int16 {
	if srcRate == dstRate || len(src) == 0 {
		return append([]int16(nil), src...)
	}
	n := int(math.Round(float64(len(src)) * float64(dstRate) / float64(srcRate)))
	dst := make([]int16, n)
	ratio := float64(srcRate) / float64(dstRate)
	for i := 0; i < n; i++ {
		pos := float64(i) * ratio
		idx := int(pos)
		frac := pos - float64(idx)
		if idx+1 < len(src) {
			dst[i] = int16(float64(src[idx])*(1-frac) + float64(src[idx+1])*frac)
		} else {
			dst[i] = src[idx]
		}
	}
	return dst
}

// resampleSinc resamples the given 16-bit samples from srcRate to dstRate using
// a windowed-sinc (Lanczos) filter for high quality.
func resampleSinc(src []int16, srcRate, dstRate int) []int16 {
	if srcRate == dstRate || len(src) == 0 {
		return append([]int16(nil), src...)
	}
	n := int(math.Round(float64(len(src)) * float64(dstRate) / float64(srcRate)))
	dst := make([]int16, n)
	ratio := float64(srcRate) / float64(dstRate)
	const a = 3 // filter width
	for i := 0; i < n; i++ {
		pos := float64(i) * ratio
		idx := int(math.Floor(pos))
		var sum float64
		var wsum float64
		for j := idx - a + 1; j <= idx+a; j++ {
			if j < 0 || j >= len(src) {
				continue
			}
			x := float64(j) - pos
			w := sinc(x) * sinc(x/float64(a))
			sum += float64(src[j]) * w
			wsum += w
		}
		if wsum != 0 {
			sum /= wsum
		}
		if sum > math.MaxInt16 {
			sum = math.MaxInt16
		} else if sum < math.MinInt16 {
			sum = math.MinInt16
		}
		dst[i] = int16(math.Round(sum))
	}
	return dst
}

func sinc(x float64) float64 {
	if x == 0 {
		return 1
	}
	x *= math.Pi
	return math.Sin(x) / x
}

// loadSound retrieves and caches a sound by ID. The CL_Sounds archive is
// opened on first use and individual sounds are parsed lazily.
func loadSound(id uint16) *clsnd.Sound {
	soundMu.Lock()
	defer soundMu.Unlock()
	if s, ok := soundCache[id]; ok {
		return s
	}
	if clSounds == nil {
		var err error
		clSounds, err = clsnd.Load(filepath.Join(dataDir, "CL_Sounds"))
		if err != nil {
			log.Printf("load CL_Sounds: %v", err)
			soundCache[id] = nil
			return nil
		}
	}
	s := clSounds.Get(uint32(id))
	if s == nil {
		log.Printf("missing sound %d", id)
	}
	soundCache[id] = s
	return s
}
