package main

import (
	"encoding/binary"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"maze.io/x/math32"

	"go_client/clsnd"
)

func round32(x float32) float32 {
	if x < 0 {
		return math32.Ceil(x - 0.5)
	}
	return math32.Floor(x + 0.5)
}

const (
	maxSounds  = 64
	mainVolume = 0.5
	sincTaps   = 16   // filter half-width for high quality sinc resampling
	sincPhases = 1024 // number of fractional phases for precomputed table
)

var (
	soundMu  sync.Mutex
	clSounds *clsnd.CLSounds
	pcmCache = make(map[uint16][]byte)

	audioContext *audio.Context
	soundPlayers = make(map[*audio.Player]struct{})
	resample     = resampleSincHQ

	blackmanCosA  [2 * sincTaps]float32
	blackmanSinA  [2 * sincTaps]float32
	blackmanCosA2 [2 * sincTaps]float32
	blackmanSinA2 [2 * sincTaps]float32

	sincTable [][]float32
	sincSums  []float32

	playSound = func(id uint16) {
		logDebug("playSound(%d) called", id)
		if blockSound {
			logDebug("playSound(%d) blocked by blockSound", id)
			return
		}
		pcm := loadSound(id)
		if pcm == nil {
			logDebug("playSound(%d) no pcm returned", id)
			return
		}
		if audioContext == nil {
			logDebug("playSound(%d) no audio context", id)
			return
		}

		p := audioContext.NewPlayerFromBytes(pcm)
		p.SetVolume(mainVolume)

		soundMu.Lock()
		for sp := range soundPlayers {
			if !sp.IsPlaying() {
				if err := sp.Close(); err != nil {
					logError("close sound player: %v", err)
				}
				delete(soundPlayers, sp)
			}
		}
		if maxSounds > 0 && len(soundPlayers) >= maxSounds {
			soundMu.Unlock()
			logDebug("playSound(%d) too many sound players (%d)", id, len(soundPlayers))
			if err := p.Close(); err != nil {
				logError("close sound player: %v", err)
			}
			return
		}
		soundPlayers[p] = struct{}{}
		soundMu.Unlock()

		logDebug("playSound(%d) playing", id)
		p.Play()
	}
)

// initSoundContext initializes the global audio context and resampler based on
// the fastSound flag. The default uses linear interpolation for a balance of
// speed and quality.
func initSoundContext() {

	rate := 44100

	if gs.fastSound {
		resample = resampleLinear
	} else {
		initSinc()
		resample = resampleSincHQ
	}

	audioContext = audio.NewContext(rate)
}

func initSinc() {
	for k := -sincTaps + 1; k <= sincTaps; k++ {
		idx := k + sincTaps - 1
		t := float32(k)/float32(sincTaps) + 0.5
		a := 2 * math32.Pi * t
		blackmanCosA[idx] = math32.Cos(a)
		blackmanSinA[idx] = math32.Sin(a)
		a2 := 2 * a
		blackmanCosA2[idx] = math32.Cos(a2)
		blackmanSinA2[idx] = math32.Sin(a2)
	}

	sincTable = make([][]float32, sincPhases)
	sincSums = make([]float32, sincPhases)
	for p := 0; p < sincPhases; p++ {
		frac := float32(p) / float32(sincPhases)
		b := (2 * math32.Pi / float32(sincTaps)) * frac
		cosB, sinB := math32.Cos(b), math32.Sin(b)
		cosB2, sinB2 := math32.Cos(2*b), math32.Sin(2*b)

		coeffs := make([]float32, 2*sincTaps)
		var wsum float32
		for k := -sincTaps + 1; k <= sincTaps; k++ {
			idx := k + sincTaps - 1

			w := blackmanCosA[idx]*cosB + blackmanSinA[idx]*sinB
			w = 0.42 - 0.5*w + 0.08*(blackmanCosA2[idx]*cosB2+blackmanSinA2[idx]*sinB2)

			coeff := w * math32.Sinc(math32.Pi*(float32(k)-frac))
			coeffs[idx] = coeff
			wsum += coeff
		}
		sincTable[p] = coeffs
		sincSums[p] = wsum
	}
}

func resampleFast(src []int16, srcRate, dstRate int) []int16 {
	if srcRate == dstRate || len(src) == 0 {
		return append([]int16(nil), src...)
	}

	n := int(round32(float32(len(src)) * float32(dstRate) / float32(srcRate)))
	dst := make([]int16, n)

	ratio := float32(srcRate) / float32(dstRate)
	for i := 0; i < n; i++ {
		srcIdx := int(float32(i) * ratio)
		if srcIdx >= len(src) {
			srcIdx = len(src) - 1
		}
		dst[i] = src[srcIdx]
	}

	return dst
}

func resampleLinear(src []int16, srcRate, dstRate int) []int16 {
	if srcRate == dstRate || len(src) == 0 {
		return append([]int16(nil), src...)
	}

	n := int(round32(float32(len(src)) * float32(dstRate) / float32(srcRate)))
	dst := make([]int16, n)

	ratio := float32(srcRate) / float32(dstRate)
	for i := 0; i < n; i++ {
		pos := float32(i) * ratio
		idx := int(pos)
		frac := pos - float32(idx)
		s0 := src[idx]
		s1 := s0
		if idx+1 < len(src) {
			s1 = src[idx+1]
		}
		v := (1-frac)*float32(s0) + frac*float32(s1)
		dst[i] = int16(round32(v))
	}

	return dst
}

func resampleSincHQ(src []int16, srcRate, dstRate int) []int16 {
	if srcRate == dstRate || len(src) == 0 {
		return append([]int16(nil), src...)
	}

	n := int(round32(float32(len(src)) * float32(dstRate) / float32(srcRate)))
	dst := make([]int16, n)
	ratio := float32(srcRate) / float32(dstRate)

	pos := float32(0)
	for i := 0; i < n; i++ {
		idx := int(pos)
		frac := pos - float32(idx)

		phase := int(round32(frac * float32(sincPhases)))
		if phase >= sincPhases {
			phase = sincPhases - 1
		}
		coeffs := sincTable[phase]
		wsum := sincSums[phase]
		var sum float32

		for k := -sincTaps + 1; k <= sincTaps; k++ {
			j := idx + k
			idxk := k + sincTaps - 1
			coeff := coeffs[idxk]
			if j < 0 || j >= len(src) {
				wsum -= coeff
				continue
			}
			sum += float32(src[j]) * coeff
		}

		if wsum != 0 {
			sum /= wsum
		}
		if sum > float32(math.MaxInt16) {
			sum = float32(math.MaxInt16)
		} else if sum < float32(math.MinInt16) {
			sum = float32(math.MinInt16)
		}
		dst[i] = int16(round32(sum))
		pos += ratio
	}
	return dst
}

// fast xorshift32 PRNG
type rnd32 uint32

func (r *rnd32) next() float32 {
	x := uint32(*r)
	x ^= x << 13
	x ^= x >> 17
	x ^= x << 5
	*r = rnd32(x)
	// scale to [0,1)
	return float32(x) * (1.0 / 4294967296.0)
}

// u8 PCM (0..255) -> s16 PCM (-32768..32767) with TPDF dither and 257 scaling
func u8ToS16TPDF(data []byte, seed uint32) []int16 {
	out := make([]int16, len(data))
	r1, r2 := rnd32(seed|1), rnd32(seed*1664525+1013904223)

	for i, b := range data {
		// TPDF dither in [-0.5, +0.5): (rand - rand)
		noise := (r1.next() - r2.next()) * 0.5
		v := float32(b) + noise

		// Map 0..255 -> -32768..32767 using *257 then offset
		// (257 uses full 16-bit span slightly better than <<8)
		s := int32(round32(v*257.0)) - 32768
		if s > math.MaxInt16 {
			s = math.MaxInt16
		} else if s < math.MinInt16 {
			s = math.MinInt16
		}
		out[i] = int16(s)
	}
	return out
}

func lowpassIIR16(x []int16, alpha float32) {
	if len(x) == 0 {
		return
	}
	// alpha ~ 0.1..0.3 for subtle smoothing
	y := float32(x[0])
	for i := range x {
		xn := float32(x[i])
		y += alpha * (xn - y)
		x[i] = int16(round32(y))
	}
}

// applyFadeInOut applies a tiny fade to the start and end of the samples
// to avoid clicks when sounds begin or end abruptly. The fade length is
// approximately 5ms of audio.
func applyFadeInOut(samples []int16, rate int) {
	fade := 441
	if fade <= 1 {
		return
	}
	if len(samples) < 2*fade {
		fade = len(samples) / 2
		if fade <= 1 {
			return
		}
	}
	for i := 0; i < fade; i++ {
		inScale := float64(i) / float64(fade)
		samples[i] = int16(float64(samples[i]) * inScale)
		outScale := float64(fade-1-i) / float64(fade)
		idx := len(samples) - fade + i
		samples[idx] = int16(float64(samples[idx]) * outScale)

	}
}

// highpassIIR16 removes DC offset using a simple one-pole high-pass filter.
// alpha should be close to 1.0 (e.g. 0.995) to only filter very low
// frequencies while leaving the audible band intact.
func highpassIIR16(x []int16, alpha float32) {
	if len(x) == 0 {
		return
	}
	var prevIn, prevOut float32
	for i := range x {
		in := float32(x[i])
		out := alpha * (prevOut + in - prevIn)
		x[i] = int16(round32(out))
		prevIn = in
		prevOut = out
	}
}

// loadSound retrieves a sound by ID, resamples it to match the audio context's
// sample rate, and caches the resulting PCM bytes. The CL_Sounds archive is
// opened on first use and individual sounds are parsed lazily.
func loadSound(id uint16) []byte {
	logDebug("loadSound(%d) called", id)
	if audioContext == nil {
		logDebug("loadSound(%d) no audio context", id)
		return nil
	}

	soundMu.Lock()
	if pcm, ok := pcmCache[id]; ok {
		soundMu.Unlock()
		if pcm == nil {
			logDebug("loadSound(%d) cached as missing", id)
		} else {
			logDebug("loadSound(%d) cache hit (%d bytes)", id, len(pcm))
		}
		return pcm
	}
	c := clSounds
	soundMu.Unlock()

	if c == nil {
		logDebug("loadSound(%d) CL sounds not loaded", id)
		return nil
	}

	logDebug("loadSound(%d) fetching from archive", id)
	s, err := c.Get(uint32(id))
	if s == nil {
		if err != nil {
			logError("unable to decode sound %d: %v", id, err)
		} else {
			logError("missing sound %d", id)
		}
		soundMu.Lock()
		pcmCache[id] = nil
		soundMu.Unlock()
		return nil
	}
	statSoundLoaded(id)
	logDebug("loadSound(%d) loaded %d Hz %d-bit %d bytes", id, s.SampleRate, s.Bits, len(s.Data))

	srcRate := int(s.SampleRate / 2)
	dstRate := audioContext.SampleRate()

	// Decode the sound data into 16-bit samples.
	var samples []int16
	switch s.Bits {
	case 8:
		if gs.fastSound {
			samples = make([]int16, len(s.Data))
			for i, b := range s.Data {
				v := int16(b) - 0x80
				samples[i] = v << 8
			}
		} else {
			samples = u8ToS16TPDF(s.Data, 0xC0FFEE)
			//		lowpassIIR16(samples, 0.)
			//		highpassIIR16(samples, 0.995)
		}
	case 16:
		if len(s.Data)%2 != 0 {
			s.Data = append(s.Data, 0x00)
		}
		samples = make([]int16, len(s.Data)/2)
		for i := 0; i < len(samples); i++ {
			samples[i] = int16(binary.BigEndian.Uint16(s.Data[2*i : 2*i+2]))
		}
		//highpassIIR16(samples, 0.995)
	default:
		return nil
	}

	if srcRate != dstRate {
		logDebug("loadSound(%d) resampling from %d to %d", id, srcRate, dstRate)
		samples = resample(samples, srcRate, dstRate)
	}

	applyFadeInOut(samples, dstRate)

	pcm := make([]byte, len(samples)*2)
	for i, v := range samples {
		pcm[2*i] = byte(v)
		pcm[2*i+1] = byte(v >> 8)
	}

	soundMu.Lock()
	pcmCache[id] = pcm
	soundMu.Unlock()
	logDebug("loadSound(%d) cached %d bytes", id, len(pcm))
	return pcm
}

// soundCacheStats returns the number of cached sounds and total bytes used.
func soundCacheStats() (count, bytes int) {
	soundMu.Lock()
	defer soundMu.Unlock()
	for _, pcm := range pcmCache {
		if pcm != nil {
			count++
			bytes += len(pcm)
		}
	}
	return
}
