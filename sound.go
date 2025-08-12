package main

import (
	"encoding/binary"
	"log"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"maze.io/x/math32"

	"go_client/clsnd"
)

const (
	maxSounds  = 64
	sincTaps   = 16   // filter half-width for high quality sinc resampling
	sincPhases = 1024 // number of fractional phases for precomputed table
)

var (
	soundMu  sync.Mutex
	clSounds *clsnd.CLSounds
	pcmCache = make(map[uint16][]byte)

	audioContext *audio.Context
	soundPlayers = make(map[*audio.Player]struct{})
	resample     = resampleLinear

	sincTable [][]float32
)

// playSound mixes the provided sound IDs and plays the result asynchronously.
// Each ID is loaded, mixed with simple clipping and then played at the current
// global volume. The function returns immediately after scheduling playback.
func playSound(ids ...uint16) {
	if len(ids) == 0 {
		return
	}
	go func(ids []uint16) {
		logDebug("playSound %v called", ids)
		if blockSound {
			logDebug("playSound blocked by blockSound")
			return
		}
		if audioContext == nil {
			logDebug("playSound no audio context")
			return
		}

		sounds := make([][]byte, 0, len(ids))
		maxSamples := 0
		for _, id := range ids {
			pcm := loadSound(id)
			if pcm == nil {
				continue
			}
			sounds = append(sounds, pcm)
			if n := len(pcm) / 2; n > maxSamples {
				maxSamples = n
			}
		}
		if len(sounds) == 0 {
			logDebug("playSound no pcm returned")
			return
		}

		mixed := make([]int32, maxSamples)
		for _, pcm := range sounds {
			n := len(pcm) / 2
			for i := 0; i < n; i++ {
				sample := int16(binary.LittleEndian.Uint16(pcm[2*i:]))
				mixed[i] += int32(sample)
			}
		}

		out := make([]byte, len(mixed)*2)
		for i, v := range mixed {
			if v > 32767 {
				v = 32767
			} else if v < -32768 {
				v = -32768
			}
			binary.LittleEndian.PutUint16(out[2*i:], uint16(int16(v)))
		}

		p := audioContext.NewPlayerFromBytes(out)
		vol := gs.Volume
		if gs.Mute {
			vol = 0
		}
		p.SetVolume(vol)

		soundMu.Lock()
		for sp := range soundPlayers {
			if !sp.IsPlaying() {
				sp.Close()
				delete(soundPlayers, sp)
			}
		}
		if maxSounds > 0 && len(soundPlayers) >= maxSounds {
			soundMu.Unlock()
			logDebug("playSound too many sound players (%d)", len(soundPlayers))
			p.Close()
			return
		}
		soundPlayers[p] = struct{}{}
		soundMu.Unlock()

		logDebug("playSound playing")
		p.Play()
	}(append([]uint16(nil), ids...))
}

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

func updateSoundVolume() {
	vol := gs.Volume
	if gs.Mute {
		vol = 0
	}

	soundMu.Lock()
	players := make([]*audio.Player, 0, len(soundPlayers))
	for sp := range soundPlayers {
		players = append(players, sp)
	}
	soundMu.Unlock()

	stopped := make([]*audio.Player, 0)
	for _, sp := range players {
		if sp.IsPlaying() {
			sp.SetVolume(vol)
		} else {
			stopped = append(stopped, sp)
		}
	}

	if len(stopped) > 0 {
		soundMu.Lock()
		defer soundMu.Unlock()
		for _, sp := range stopped {
			delete(soundPlayers, sp)
			sp.Close()
		}
	}
}

func initSinc() {
	sincTable = make([][]float32, sincPhases)
	for p := 0; p < sincPhases; p++ {
		frac := float32(p) / float32(sincPhases)
		coeffs := make([]float32, 2*sincTaps)
		var sum float64
		for k := -sincTaps + 1; k <= sincTaps; k++ {
			idx := k + sincTaps - 1
			t := float32(k) - frac
			n := float64(idx) / float64(2*sincTaps-1)
			w := 0.42 - 0.5*math.Cos(2*math.Pi*n) + 0.08*math.Cos(4*math.Pi*n)
			coeff := float32(w) * math32.Sinc(float32(math.Pi)*t)
			coeffs[idx] = coeff
			sum += float64(coeff)
		}
		if sum != 0 {
			inv := float32(1 / sum)
			for i := range coeffs {
				coeffs[i] *= inv
			}
		}
		sincTable[p] = coeffs
	}
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
		s0 := src[idx]
		s1 := s0
		if idx+1 < len(src) {
			s1 = src[idx+1]
		}
		v := (1-frac)*float64(s0) + frac*float64(s1)
		dst[i] = int16(math.Round(v))
	}

	return dst
}

func resampleSincHQ(src []int16, srcRate, dstRate int) []int16 {
	if srcRate == dstRate || len(src) == 0 {
		return append([]int16(nil), src...)
	}

	n := int(math.Round(float64(len(src)) * float64(dstRate) / float64(srcRate)))
	dst := make([]int16, n)
	ratio := float64(srcRate) / float64(dstRate)

	pos := 0.0
	for i := 0; i < n; i++ {
		idx := int(pos)
		frac := pos - float64(idx)
		phase := int(frac*float64(sincPhases) + 0.5)
		if phase >= sincPhases {
			phase = sincPhases - 1
		}
		coeffs := sincTable[phase]
		var sum float64
		for k := -sincTaps + 1; k <= sincTaps; k++ {
			j := idx + k
			if j < 0 || j >= len(src) {
				continue
			}
			coeff := coeffs[k+sincTaps-1]
			sum += float64(src[j]) * float64(coeff)
		}
		if sum > float64(math.MaxInt16) {
			sum = float64(math.MaxInt16)
		} else if sum < float64(math.MinInt16) {
			sum = float64(math.MinInt16)
		}
		dst[i] = int16(math.Round(sum))
		pos += ratio
	}
	return dst
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

// applyFadeInOut applies a tiny fade to the start and end of the samples
// to avoid clicks when sounds begin or end abruptly. The fade length is
// approximately 5ms of audio.
func applyFadeInOut(samples []int16, rate int) {
	fade := 220
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
		samples = u8ToS16TPDF(s.Data, 0xC0FFEE)
	case 16:
		if len(s.Data)%2 != 0 {
			s.Data = append(s.Data, 0x00)
		}
		samples = make([]int16, len(s.Data)/2)
		for i := 0; i < len(samples); i++ {
			samples[i] = int16(binary.BigEndian.Uint16(s.Data[2*i : 2*i+2]))
		}
	default:
		log.Fatalf("Invalid number of bits: %v: ID: %v", s.Bits, id)
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
