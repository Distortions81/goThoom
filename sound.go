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

const maxSounds = 64
const mainVolume = 0.5

var (
	soundMu  sync.Mutex
	clSounds *clsnd.CLSounds
	pcmCache = make(map[uint16][]byte)

	audioContext *audio.Context
	soundPlayers = make(map[*audio.Player]struct{})
	resample     = resampleSincHQ

	playSound = func(id uint16) {
		if blockSound {
			return
		}
		//logError("sound: %v", id)
		pcm := loadSound(id)
		if pcm == nil || audioContext == nil {
			return
		}

		p := audioContext.NewPlayerFromBytes(pcm)
		p.SetVolume(mainVolume)

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

	if gs.FastSound {
		resample = resampleFast
	} else {
		resample = resampleSincHQ
	}

	audioContext = audio.NewContext(rate)
}

func resampleFast(src []int16, srcRate, dstRate int) []int16 {
	if srcRate == dstRate || len(src) == 0 {
		return append([]int16(nil), src...)
	}

	n := int(math.Round(float64(len(src)) * float64(dstRate) / float64(srcRate)))
	dst := make([]int16, n)

	ratio := float64(srcRate) / float64(dstRate)
	for i := 0; i < n; i++ {
		srcIdx := int(float64(i) * ratio)
		if srcIdx >= len(src) {
			srcIdx = len(src) - 1
		}
		dst[i] = src[srcIdx]
	}

	return dst
}

func resampleSincHQ(src []int16, srcRate, dstRate int) []int16 {
	if srcRate == dstRate || len(src) == 0 {
		return append([]int16(nil), src...)
	}

	// Number of taps (filter half-width)
	const taps = 16 // 8–16 for very high quality
	n := int(math.Round(float64(len(src)) * float64(dstRate) / float64(srcRate)))
	dst := make([]int16, n)
	ratio := float64(srcRate) / float64(dstRate)

	for i := 0; i < n; i++ {
		pos := float64(i) * ratio
		idx := int(math.Floor(pos))
		var sum float64
		var wsum float64

		for j := idx - taps + 1; j <= idx+taps; j++ {
			if j < 0 || j >= len(src) {
				continue
			}
			x := float64(j) - pos
			w := sinc(x) * blackmanWindow(x, float64(taps))
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

// sinc(x) = sin(pi*x)/(pi*x)
func sinc(x float64) float64 {
	if x == 0 {
		return 1
	}
	xpi := math.Pi * x
	return math.Sin(xpi) / xpi
}

// Blackman window for smoothing the sinc
func blackmanWindow(x, a float64) float64 {
	t := (x / a) + 0.5
	if t < 0 || t > 1 {
		return 0
	}
	// Standard Blackman coefficients
	alpha0 := 0.42
	alpha1 := 0.5
	alpha2 := 0.08
	return alpha0 - alpha1*math.Cos(2*math.Pi*t) + alpha2*math.Cos(4*math.Pi*t)
}

// fast xorshift32 PRNG
type rnd32 uint32

func (r *rnd32) next() float64 {
	x := uint32(*r)
	x ^= x << 13
	x ^= x >> 17
	x ^= x << 5
	*r = rnd32(x)
	// scale to [0,1)
	return float64(x) * (1.0 / 4294967296.0)
}

// u8 PCM (0..255) -> s16 PCM (-32768..32767) with TPDF dither and 257 scaling
func u8ToS16TPDF(data []byte, seed uint32) []int16 {
	out := make([]int16, len(data))
	r1, r2 := rnd32(seed|1), rnd32(seed*1664525+1013904223)

	for i, b := range data {
		// TPDF dither in [-0.5, +0.5): (rand - rand)
		noise := (r1.next() - r2.next()) * 0.5
		v := float64(b) + noise

		// Map 0..255 -> -32768..32767 using *257 then offset
		// (257 uses full 16-bit span slightly better than <<8)
		s := int32(math.Round(v*257.0)) - 32768
		if s > math.MaxInt16 {
			s = math.MaxInt16
		} else if s < math.MinInt16 {
			s = math.MinInt16
		}
		out[i] = int16(s)
	}
	return out
}

func lowpassIIR16(x []int16, alpha float64) {
	if len(x) == 0 {
		return
	}
	// alpha ~ 0.1..0.3 for subtle smoothing
	y := float64(x[0])
	for i := range x {
		xn := float64(x[i])
		y += alpha * (xn - y)
		x[i] = int16(math.Round(y))
	}
}

// highpassIIR16 removes DC offset using a simple one-pole high-pass filter.
// alpha should be close to 1.0 (e.g. 0.995) to only filter very low
// frequencies while leaving the audible band intact.
func highpassIIR16(x []int16, alpha float64) {
	if len(x) == 0 {
		return
	}
	var prevIn, prevOut float64
	for i := range x {
		in := float64(x[i])
		out := alpha * (prevOut + in - prevIn)
		x[i] = int16(math.Round(out))
		prevIn = in
		prevOut = out
	}
}

// loadSound retrieves a sound by ID, resamples it to match the audio context's
// sample rate, and caches the resulting PCM bytes. The CL_Sounds archive is
// opened on first use and individual sounds are parsed lazily.
func loadSound(id uint16) []byte {
	if audioContext == nil {
		return nil
	}

	soundMu.Lock()
	if pcm, ok := pcmCache[id]; ok {
		soundMu.Unlock()
		return pcm
	}
	soundMu.Unlock()

	soundMu.Lock()
	if clSounds == nil {
		var err error
		clSounds, err = clsnd.Load(filepath.Join(dataDir, "CL_Sounds"))
		if err != nil {
			log.Printf("load CL_Sounds: %v", err)
			pcmCache[id] = nil
			soundMu.Unlock()
			return nil
		}
	}
	soundMu.Unlock()

	s := clSounds.Get(uint32(id))
	if s == nil {
		log.Printf("missing sound %d", id)
		soundMu.Lock()
		pcmCache[id] = nil
		soundMu.Unlock()
		return nil
	}

	srcRate := int(s.SampleRate / 2)
	dstRate := audioContext.SampleRate()

	// Decode the sound data into 16-bit samples.
	var samples []int16
	switch s.Bits {
	case 8:
		if !gs.FastSound {
			samples = u8ToS16TPDF(s.Data, 0xC0FFEE)
			lowpassIIR16(samples, 0.5)
			highpassIIR16(samples, 0.995)
		} else {
			samples = make([]int16, len(s.Data))
			for i, b := range s.Data {
				v := int16(b) - 0x80
				samples[i] = v << 8
			}
		}
	case 16:
		if len(s.Data)%2 != 0 {
			return nil
		}
		samples = make([]int16, len(s.Data)/2)
		for i := 0; i < len(samples); i++ {
			samples[i] = int16(binary.BigEndian.Uint16(s.Data[2*i : 2*i+2]))
		}
	default:
		return nil
	}

	if srcRate != dstRate {
		samples = resample(samples, srcRate, dstRate)
	}

	pcm := make([]byte, len(samples)*2)
	for i, v := range samples {
		pcm[2*i] = byte(v)
		pcm[2*i+1] = byte(v >> 8)
	}

	soundMu.Lock()
	pcmCache[id] = pcm
	soundMu.Unlock()
	return pcm
}
