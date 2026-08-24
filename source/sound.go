package main

import (
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"sync"

	math32 "github.com/chewxy/math32"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/tphakala/simd/f32"

	"gothoom/clsnd"
)

const (
	maxSounds             = 64
	maxCachedSoundPlayers = 64
	dbPad                 = -3
)

type soundPlaybackKey struct {
	ids               string
	context           *audio.Context
	enhancementAmount uint64
	enhanced          bool
	highQuality       bool
}

type soundPlaybackCacheEntry struct {
	pcm     []byte
	players []*audio.Player
}

var (
	soundMu  sync.Mutex
	clSounds *clsnd.CLSounds
	pcmCache = make(map[uint16][]byte)

	audioContext          *audio.Context
	soundPlayers          = make(map[*audio.Player]struct{})
	soundPlayerCache      = make(map[soundPlaybackKey]*soundPlaybackCacheEntry)
	cachedSoundPlayers    = make(map[*audio.Player]struct{})
	reservedSoundPlayers  = make(map[*audio.Player]struct{})
	soundPlayerCacheCount int
	notifPlayers          = make(map[*audio.Player]struct{})
	notifPlayersMu        sync.Mutex

	sndDumpOnce   sync.Once
	sndDumpMu     sync.Mutex
	dumpedSndIDs  = make(map[uint16]struct{})
	sndMetaWriter *csv.Writer

	highQualityResampling bool
)

func init() {
	setHighQualityResamplingEnabled(gs.HighQualityResampling)
}

func setHighQualityResamplingEnabled(enabled bool) {
	highQualityResampling = enabled
}

func soundPlaybackCacheKey(ids []uint16, context *audio.Context, enhanced bool, enhancementAmount float64, highQuality bool) soundPlaybackKey {
	ids = append([]uint16(nil), ids...)
	slices.Sort(ids)
	encoded := make([]byte, len(ids)*2)
	for i, id := range ids {
		binary.LittleEndian.PutUint16(encoded[i*2:], id)
	}
	return soundPlaybackKey{
		ids:               string(encoded),
		context:           context,
		enhancementAmount: math.Float64bits(enhancementAmount),
		enhanced:          enhanced,
		highQuality:       highQuality,
	}
}

func pruneStoppedSoundPlayersLocked() {
	for player := range soundPlayers {
		if _, reserved := reservedSoundPlayers[player]; reserved {
			continue
		}
		if player.IsPlaying() {
			continue
		}
		delete(soundPlayers, player)
		if _, cached := cachedSoundPlayers[player]; !cached {
			_ = player.Close()
		}
	}
}

// acquireSoundPlaybackPlayer reuses an idle player for the same rendered PCM,
// or creates another player when the sound overlaps itself. When pcm is nil,
// found reports whether the rendered sound was already cached.
func acquireSoundPlaybackPlayer(key soundPlaybackKey, pcm []byte) (player *audio.Player, found bool) {
	soundMu.Lock()
	defer soundMu.Unlock()
	pruneStoppedSoundPlayersLocked()

	entry, found := soundPlayerCache[key]
	if !found {
		if pcm == nil {
			return nil, false
		}
		entry = &soundPlaybackCacheEntry{pcm: pcm}
		soundPlayerCache[key] = entry
	}

	if maxSounds > 0 && len(soundPlayers) >= maxSounds {
		return nil, true
	}
	for i := 0; i < len(entry.players); {
		player := entry.players[i]
		if player.IsPlaying() {
			i++
			continue
		}
		if err := player.Rewind(); err != nil {
			delete(cachedSoundPlayers, player)
			delete(soundPlayers, player)
			_ = player.Close()
			soundPlayerCacheCount--
			copy(entry.players[i:], entry.players[i+1:])
			entry.players[len(entry.players)-1] = nil
			entry.players = entry.players[:len(entry.players)-1]
			continue
		}
		soundPlayers[player] = struct{}{}
		reservedSoundPlayers[player] = struct{}{}
		return player, true
	}
	if soundPlayerCacheCount >= maxCachedSoundPlayers {
		return newUncachedSoundPlayerLocked(key.context, entry.pcm), true
	}
	player = key.context.NewPlayerFromBytes(entry.pcm)
	entry.players = append(entry.players, player)
	cachedSoundPlayers[player] = struct{}{}
	soundPlayerCacheCount++
	soundPlayers[player] = struct{}{}
	reservedSoundPlayers[player] = struct{}{}
	return player, true
}

func newUncachedSoundPlayerLocked(context *audio.Context, pcm []byte) *audio.Player {
	if maxSounds > 0 && len(soundPlayers) >= maxSounds {
		return nil
	}
	player := context.NewPlayerFromBytes(pcm)
	soundPlayers[player] = struct{}{}
	reservedSoundPlayers[player] = struct{}{}
	return player
}

func playGameSoundPlayer(player *audio.Player) {
	if player == nil {
		return
	}
	volume := gs.MasterVolume * gs.GameVolume
	if gs.Mute || focusMuted {
		volume = 0
	}
	player.SetVolume(volume)
	player.Play()
	soundMu.Lock()
	delete(reservedSoundPlayers, player)
	soundMu.Unlock()
}

// stopAllSounds halts and disposes all currently playing audio players.
func stopAllSounds() {
	soundMu.Lock()
	closed := make(map[*audio.Player]struct{}, len(cachedSoundPlayers))
	for player := range cachedSoundPlayers {
		_ = player.Close()
		closed[player] = struct{}{}
	}
	for player := range soundPlayers {
		if _, ok := closed[player]; !ok {
			_ = player.Close()
		}
	}
	soundPlayers = make(map[*audio.Player]struct{})
	soundPlayerCache = make(map[soundPlaybackKey]*soundPlaybackCacheEntry)
	cachedSoundPlayers = make(map[*audio.Player]struct{})
	reservedSoundPlayers = make(map[*audio.Player]struct{})
	soundPlayerCacheCount = 0
	soundMu.Unlock()

	notifPlayersMu.Lock()
	for sp := range notifPlayers {
		delete(notifPlayers, sp)
	}
	notifPlayersMu.Unlock()
}

// stopAllAudioPlayers stops and disposes every active audio player type.
func stopAllAudioPlayers() {
	stopAllSounds()
	stopAllTTS()
	stopAllMusic()
}

// playSound mixes the provided sound IDs and plays the result asynchronously.
// Each ID is loaded, mixed with simple clipping and then played at the current
// global volume. The function returns immediately after scheduling playback.
func playSound(ids []uint16) {
	if len(ids) == 0 || gs.Mute || focusMuted || !gs.GameSound {
		return
	}
	ids = append([]uint16(nil), ids...)
	useEnhancement := gs.SoundEnhancement
	useHighQuality := gs.HighQualityResampling
	enhancementAmount := clampSoundEnhancementAmount(gs.SoundEnhancementAmount)
	if !useEnhancement {
		enhancementAmount = 0
	}
	go func(ids []uint16, enableEnhancement bool, enhancementAmount float64, highQuality bool) {
		if gs.Mute || focusMuted || !gs.GameSound {
			return
		}
		//logDebug("playSound %v called", ids)
		if blockSound {
			//logDebug("playSound blocked by blockSound")
			return
		}
		if audioContext == nil {
			logDebug("playSound no audio context")
			return
		}
		context := audioContext
		cacheKey := soundPlaybackCacheKey(ids, context, enableEnhancement, enhancementAmount, highQuality)
		if player, found := acquireSoundPlaybackPlayer(cacheKey, nil); found {
			playGameSoundPlayer(player)
			return
		}

		var valid map[uint16]struct{}
		soundMu.Lock()
		c := clSounds
		soundMu.Unlock()
		if c != nil {
			vid := c.IDs()
			valid = make(map[uint16]struct{}, len(vid))
			for _, v := range vid {
				valid[uint16(v)] = struct{}{}
			}
		}

		sounds := make([][]byte, 0, len(ids))
		maxSamples := 0
		for _, id := range ids {
			if valid != nil {
				if _, ok := valid[id]; !ok {
					logDebug("playSound unknown id %d", id)
					continue
				}
			}
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

		mixSamples := maxSamples
		if mixSamples == 0 {
			return
		}

		mixed := make([]int32, mixSamples)

		chunks := runtime.NumCPU()
		if chunks > mixSamples {
			chunks = mixSamples
		}
		if chunks < 1 {
			chunks = 1
		}
		chunkSize := (mixSamples + chunks - 1) / chunks

		var wg sync.WaitGroup
		maxCh := make(chan int32, chunks)

		for start := 0; start < mixSamples; start += chunkSize {
			end := start + chunkSize
			if end > mixSamples {
				end = mixSamples
			}
			wg.Add(1)
			go func(start, end int) {
				defer wg.Done()
				localMax := int32(0)
				for i := start; i < end; i++ {
					var sum int32
					for _, pcm := range sounds {
						if n := len(pcm) / 2; i < n {
							sample := int16(binary.LittleEndian.Uint16(pcm[2*i:]))
							sum += int32(sample)
						}
					}
					mixed[i] = sum
					if sum < 0 {
						sum = -sum
					}
					if sum > localMax {
						localMax = sum
					}
				}
				maxCh <- localMax
			}(start, end)
		}
		wg.Wait()
		close(maxCh)

		enableReverb := enableEnhancement
		var left []int32
		var right []int32

		if enableReverb {
			applyGameSoundReverb(mixed, enhancementAmount)
		}

		if enableEnhancement {
			left = make([]int32, len(mixed))
			copy(left, mixed)
			right = make([]int32, len(mixed))
			copy(right, mixed)
		} else {
			left = mixed
		}

		maxVal := int32(0)
		for v := range maxCh {
			if !enableEnhancement && v > maxVal {
				maxVal = v
			}
		}
		if enableEnhancement {
			for i := 0; i < len(left); i++ {
				v := left[i]
				if v < 0 {
					v = -v
				}
				if v > maxVal {
					maxVal = v
				}
				if right != nil {
					vr := right[i]
					if vr < 0 {
						vr = -vr
					}
					if vr > maxVal {
						maxVal = vr
					}
				}
			}
		}

		// Apply peak normalization and reduce volume for overlapping sounds
		scale := 1 / float64(len(sounds))
		if maxVal > 0 {
			scale *= math.Min(1, 32767.0/float64(maxVal))
		}

		out := make([]byte, len(left)*4)

		wg = sync.WaitGroup{}
		for start := 0; start < len(left); start += chunkSize {
			end := start + chunkSize
			if end > len(left) {
				end = len(left)
			}
			wg.Add(1)
			go func(start, end int) {
				defer wg.Done()
				for i := start; i < end; i++ {
					lv := int32(float64(left[i]) * scale)
					if lv > 32767 {
						lv = 32767
					} else if lv < -32768 {
						lv = -32768
					}
					rv := lv
					if right != nil {
						rv = int32(float64(right[i]) * scale)
						if rv > 32767 {
							rv = 32767
						} else if rv < -32768 {
							rv = -32768
						}
					}
					off := 4 * i
					binary.LittleEndian.PutUint16(out[off:], uint16(int16(lv)))
					binary.LittleEndian.PutUint16(out[off+2:], uint16(int16(rv)))
				}
			}(start, end)
		}
		wg.Wait()

		player, _ := acquireSoundPlaybackPlayer(cacheKey, out)
		playGameSoundPlayer(player)
	}(ids, useEnhancement, enhancementAmount, useHighQuality)
}

// initSoundContext initializes the global audio context.
func initSoundContext() {
	if audioContext != nil {
		stopAllSounds()
	}
	rate := sampleRate
	audioContext = audio.NewContext(rate)
}

func updateSoundVolume() {
	gameVol := gs.MasterVolume * gs.GameVolume
	ttsVol := gs.MasterVolume * gs.ChatTTSVolume
	musicVol := gs.MasterVolume * gs.MusicVolume
	notifVol := gs.MasterVolume * gs.NotificationVolume
	if !gs.GameSound {
		gameVol = 0
		notifVol = 0
	}
	if !gs.ChatTTS {
		ttsVol = 0
	}
	if !gs.Music {
		musicVol = 0
	}
	if !gs.NotificationBeep {
		notifVol = 0
	}
	if gs.Mute || focusMuted {
		gameVol = 0
		ttsVol = 0
		musicVol = 0
		notifVol = 0
	}

	soundMu.Lock()
	players := make([]*audio.Player, 0, len(soundPlayers))
	for sp := range soundPlayers {
		players = append(players, sp)
	}
	soundMu.Unlock()

	notifPlayersMu.Lock()
	notif := make(map[*audio.Player]struct{}, len(notifPlayers))
	for sp := range notifPlayers {
		notif[sp] = struct{}{}
	}
	notifPlayersMu.Unlock()

	ttsPlayersMu.Lock()
	tts := make([]*audio.Player, 0, len(ttsPlayers))
	for p := range ttsPlayers {
		tts = append(tts, p)
	}
	ttsPlayersMu.Unlock()

	musicPlayersMu.Lock()
	music := make([]*audio.Player, 0, len(musicPlayers))
	for p := range musicPlayers {
		music = append(music, p)
	}
	musicPlayersMu.Unlock()

	stopped := make([]*audio.Player, 0)
	notifStopped := make([]*audio.Player, 0)
	for _, sp := range players {
		if sp.IsPlaying() {
			if _, ok := notif[sp]; ok {
				sp.SetVolume(notifVol)
			} else {
				sp.SetVolume(gameVol)
			}
		} else {
			stopped = append(stopped, sp)
			if _, ok := notif[sp]; ok {
				notifStopped = append(notifStopped, sp)
			}
		}
	}

	ttsStopped := make([]*audio.Player, 0)
	for _, p := range tts {
		if p.IsPlaying() {
			p.SetVolume(ttsVol)
		} else {
			ttsStopped = append(ttsStopped, p)
		}
	}

	for _, p := range music {
		// A music player is registered before its five-second prebuffered group
		// is released. IsPlaying is therefore false while it is validly waiting
		// to start; closing it here silences every bard in that group. The music
		// playback goroutine owns its lifecycle and removes it on completion.
		p.SetVolume(musicVol)
	}

	if len(stopped) > 0 {
		soundMu.Lock()
		for _, sp := range stopped {
			if _, reserved := reservedSoundPlayers[sp]; reserved {
				continue
			}
			if sp.IsPlaying() {
				continue
			}
			delete(soundPlayers, sp)
			if _, cached := cachedSoundPlayers[sp]; !cached {
				_ = sp.Close()
			}
		}
		soundMu.Unlock()
	}

	if len(notifStopped) > 0 {
		notifPlayersMu.Lock()
		for _, sp := range notifStopped {
			delete(notifPlayers, sp)
		}
		notifPlayersMu.Unlock()
	}

	if len(ttsStopped) > 0 {
		ttsPlayersMu.Lock()
		for _, p := range ttsStopped {
			delete(ttsPlayers, p)
			p.Close()
		}
		ttsPlayersMu.Unlock()
	}

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

func u8ToS16Fast(data []byte) []int16 {
	out := make([]int16, len(data))
	for i, b := range data {
		v := int32(b)*257 - 32768
		if v > math.MaxInt16 {
			v = math.MaxInt16
		} else if v < math.MinInt16 {
			v = math.MinInt16
		}
		out[i] = int16(v)
	}
	return out
}

func ResampleLinearInt16(src []int16, srcRate, dstRate int) []int16 {
	if len(src) == 0 {
		return nil
	}
	if srcRate <= 0 || dstRate <= 0 || srcRate == dstRate {
		out := make([]int16, len(src))
		copy(out, src)
		return out
	}

	n := int(math.Round(float64(len(src)) * float64(dstRate) / float64(srcRate)))
	if n < 1 {
		n = 1
	}
	out := make([]int16, n)
	step := float64(srcRate) / float64(dstRate)
	pos := 0.0
	lastIdx := len(src) - 1
	for i := 0; i < n; i++ {
		idx := int(pos)
		if idx > lastIdx {
			idx = lastIdx
		}
		frac := pos - float64(idx)
		s0 := float64(src[idx])
		var s1 float64
		if idx < lastIdx {
			s1 = float64(src[idx+1])
		} else {
			s1 = s0
		}
		out[i] = int16(math.Round(s0 + (s1-s0)*frac))
		pos += step
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

// applyGameSoundReverb adds a light ambience tuned to resemble a wide open
// field. The goal is mostly a dry signal with a faint but noticeable sense of
// distance, so the processing uses short delays, controlled feedback, and a
// gentle roll-off of the high end. The work is done on 32-bit intermediate
// samples so the later normalization still fits the 16-bit output range.
func applyGameSoundReverb(samples []int32, enhancementAmount float64) {
	if len(samples) == 0 {
		return
	}

	rate := sampleRate
	if audioContext != nil {
		if r := audioContext.SampleRate(); r > 0 {
			rate = r
		}
	}
	if rate <= 0 {
		return
	}

	floatSamples := make([]float32, len(samples))
	for i, v := range samples {
		floatSamples[i] = float32(v)
	}

	ambienceA := buildMicroAmbience(floatSamples, rate, 0)
	offsetSamples := int(math32.Round(float32(rate) * 0.0025))
	ambienceB := buildMicroAmbience(floatSamples, rate, offsetSamples)

	for i := range ambienceA {
		ambienceA[i] += ambienceB[i]
	}
	wetBuffer := ambienceA

	if shelf := newHighShelf(float32(rate), 3800, -2); shelf != nil {
		applyBiquad(wetBuffer, shelf)
	}

	amount := float32(clampSoundEnhancementAmount(enhancementAmount))
	mixScale := math32.Sqrt(amount)

	delayFeedback := 0.42 * mixScale
	if delayFeedback > 0.9 {
		delayFeedback = 0.9
	} else if delayFeedback < 0 {
		delayFeedback = 0
	}

	delayMix := 0.58 * mixScale
	if delayMix > 0.95 {
		delayMix = 0.95
	} else if delayMix < 0 {
		delayMix = 0
	}

	applySlapDelay(wetBuffer, rate, 0.085, delayFeedback, delayMix)

	wetMix := 0.14 * amount
	if wetMix > 0.85 {
		wetMix = 0.85
	} else if wetMix < 0 {
		wetMix = 0
	}
	const scatterRatio = 0.4
	combMix := wetMix * (1 - scatterRatio)
	if combMix < 0 {
		combMix = 0
	}
	scatterMix := wetMix - combMix
	if scatterMix < 0 {
		scatterMix = 0
	}
	dryMix := 1 - wetMix
	if dryMix < 0 {
		dryMix = 0
	}
	const wetLowpass = 0.3
	scatterFeedback := 0.1 * amount
	if scatterFeedback > 0.9 {
		scatterFeedback = 0.9
	} else if scatterFeedback < 0 {
		scatterFeedback = 0
	}
	const maxInt32 = float32(1<<31 - 1)
	const minInt32 = -float32(1 << 31)

	scatterDelay := int(math32.Round(float32(rate) * 0.03))
	var scatter []float32
	if scatterDelay > 0 && scatterMix > 0 {
		scatter = make([]float32, scatterDelay)
	}

	scatterIdx := 0
	var wetState float32
	for i, dry := range floatSamples {
		var wet float32
		if combMix > 0 && i < len(wetBuffer) {
			wet += wetBuffer[i] * combMix
		}
		if len(scatter) > 0 {
			delayed := scatter[scatterIdx]
			wet += delayed * scatterMix
			scatter[scatterIdx] = dry + delayed*scatterFeedback
			scatterIdx++
			if scatterIdx >= len(scatter) {
				scatterIdx = 0
			}
		}

		wetState += (wet - wetState) * wetLowpass
		val := dry*dryMix + wetState
		if val > maxInt32 {
			val = maxInt32
		} else if val < minInt32 {
			val = minInt32
		}
		floatSamples[i] = val
	}

	applySaturation(floatSamples, ambienceB, 1.15, 0.2)

	for i, v := range floatSamples {
		if v > maxInt32 {
			v = maxInt32
		} else if v < minInt32 {
			v = minInt32
		}
		samples[i] = int32(math32.Round(v))
	}
}

type microComb struct {
	buf   []float32
	idx   int
	fb    float32
	state float32
}

func buildMicroAmbience(input []float32, rate int, offsetSamples int) []float32 {
	n := len(input)
	out := make([]float32, n)
	if n == 0 || rate <= 0 {
		return out
	}

	taps := []struct {
		seconds  float32
		feedback float32
	}{
		{seconds: 0.021, feedback: 0.18},
		{seconds: 0.033, feedback: 0.15},
		{seconds: 0.044, feedback: 0.13},
	}

	const dampingHz = 3200.0
	lpCoef := lowpassCoefficient(rate, dampingHz)

	combs := make([]microComb, 0, len(taps))
	for i, tap := range taps {
		delay := int(math32.Round(tap.seconds * float32(rate)))
		if offsetSamples != 0 && i%2 == 1 {
			delay += offsetSamples
		}
		if delay < 1 {
			continue
		}
		combs = append(combs, microComb{
			buf: make([]float32, delay),
			fb:  tap.feedback,
		})
	}
	if len(combs) == 0 {
		return out
	}

	invCount := 1 / float32(len(combs))
	for i := 0; i < n; i++ {
		in := input[i]
		var wet float32
		for j := range combs {
			c := &combs[j]
			delayed := c.buf[c.idx]
			c.state += lpCoef * (delayed - c.state)
			wet += c.state
			c.buf[c.idx] = in + c.state*c.fb
			c.idx++
			if c.idx >= len(c.buf) {
				c.idx = 0
			}
		}
		out[i] = wet * invCount
	}
	return out
}

func lowpassCoefficient(rate int, cutoff float32) float32 {
	if rate <= 0 || cutoff <= 0 {
		return 0
	}
	rc := 1.0 / (2 * math32.Pi * cutoff)
	dt := 1.0 / float32(rate)
	alpha := dt / (rc + dt)
	if alpha < 0 {
		return 0
	}
	if alpha > 1 {
		return 1
	}
	return alpha
}

func applySlapDelay(samples []float32, rate int, delaySec, feedback, mix float32) {
	if rate <= 0 || len(samples) == 0 || delaySec <= 0 || mix <= 0 {
		return
	}
	if mix > 1 {
		mix = 1
	}
	if feedback < 0 {
		feedback = 0
	} else if feedback > 0.95 {
		feedback = 0.95
	}
	delay := int(math32.Round(delaySec * float32(rate)))
	if delay < 1 {
		delay = 1
	}
	buf := make([]float32, delay)
	coef := lowpassCoefficient(rate, 7000)
	dryMix := 1 - mix
	idx := 0
	var state float32
	for i := range samples {
		input := samples[i]
		delayed := buf[idx]
		state += coef * (delayed - state)
		wet := state
		buf[idx] = input + state*feedback
		idx++
		if idx >= delay {
			idx = 0
		}
		samples[i] = input*dryMix + wet*mix
	}
}

type biquad struct {
	b0, b1, b2 float32
	a1, a2     float32
	z1, z2     float32
}

func newBiquad(b0, b1, b2, a0, a1, a2 float32) *biquad {
	if a0 == 0 {
		return nil
	}
	invA0 := 1 / a0
	return &biquad{
		b0: b0 * invA0,
		b1: b1 * invA0,
		b2: b2 * invA0,
		a1: a1 * invA0,
		a2: a2 * invA0,
	}
}

func (b *biquad) Process(x float32) float32 {
	if b == nil {
		return x
	}
	y := b.b0*x + b.z1
	b.z1 = b.b1*x - b.a1*y + b.z2
	b.z2 = b.b2*x - b.a2*y
	return y
}

func applyBiquad(samples []float32, b *biquad) {
	if b == nil {
		return
	}
	for i := range samples {
		samples[i] = b.Process(samples[i])
	}
}

func newHighShelf(fs, freq, gainDB float32) *biquad {
	if fs <= 0 || freq <= 0 {
		return nil
	}
	if freq >= fs/2 {
		freq = fs/2 - 1
		if freq <= 0 {
			freq = fs / 4
		}
	}
	A := math32.Pow(10, gainDB/40)
	w0 := 2 * math32.Pi * freq / fs
	sinW0, cosW0 := math32.Sincos(w0)
	alpha := sinW0 / math32.Sqrt2
	sqrtA := math32.Sqrt(A)
	beta := 2 * sqrtA * alpha

	b0 := A * ((A + 1) + (A-1)*cosW0 + beta)
	b1 := -2 * A * ((A - 1) + (A+1)*cosW0)
	b2 := A * ((A + 1) + (A-1)*cosW0 - beta)
	a0 := (A + 1) - (A-1)*cosW0 + beta
	a1 := 2 * ((A - 1) - (A+1)*cosW0)
	a2 := (A + 1) - (A-1)*cosW0 - beta

	return newBiquad(b0, b1, b2, a0, a1, a2)
}

func applySaturation(samples, scratch []float32, drive, mix float32) {
	if drive <= 0 || len(samples) == 0 {
		return
	}
	if len(scratch) < len(samples) {
		return
	}
	if mix < 0 {
		mix = 0
	} else if mix > 1 {
		mix = 1
	}
	const toFloat = 1.0 / 32768.0
	const fromFloat = 32768.0
	dryMix := 1 - mix
	norm := math32.Tanh(drive)
	if norm == 0 {
		norm = 1
	}
	for i, x := range samples {
		scratch[i] = x * toFloat * drive
	}
	f32.Tanh(scratch[:len(samples)], scratch[:len(samples)])
	for i, x := range samples {
		xf := x * toFloat
		sat := scratch[i] / norm
		samples[i] = (dryMix*xf + mix*sat) * fromFloat
	}
}

// loadSound retrieves a sound by ID, resamples it to match the audio context's
// sample rate, and caches the resulting PCM bytes. The CL_Sounds archive is
// opened on first use and individual sounds are parsed lazily.
func loadSound(id uint16) []byte {
	//logDebug("loadSound(%d) called", id)
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
			//logDebug("loadSound(%d) cache hit (%d bytes)", id, len(pcm))
		}
		return pcm
	}
	c := clSounds
	soundMu.Unlock()

	if c == nil {
		logDebug("loadSound(%d) CL sounds not loaded", id)
		return nil
	}

	//logDebug("loadSound(%d) fetching from archive", id)
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
	//logDebug("loadSound(%d) loaded %d Hz %d-bit %d bytes", id, s.SampleRate, s.Bits, len(s.Data))

	srcRate := int(s.SampleRate)
	dstRate := audioContext.SampleRate()

	// Decode the sound data into 16-bit samples.
	var samples []int16
	useHighQuality := highQualityResampling
	switch s.Bits {
	case 8:
		if useHighQuality {
			if s.Channels > 1 {
				frames := len(s.Data) / int(s.Channels)
				mono := make([]byte, frames)
				for i := 0; i < frames; i++ {
					mono[i] = s.Data[i*int(s.Channels)]
				}
				samples = u8ToS16TPDF(mono, 0xC0FFEE)
			} else {
				samples = u8ToS16TPDF(s.Data, 0xC0FFEE)
			}
		} else {
			if s.Channels > 1 {
				frames := len(s.Data) / int(s.Channels)
				mono := make([]byte, frames)
				for i := 0; i < frames; i++ {
					mono[i] = s.Data[i*int(s.Channels)]
				}
				samples = u8ToS16Fast(mono)
			} else {
				samples = u8ToS16Fast(s.Data)
			}
		}
	case 16:
		if len(s.Data)%2 != 0 {
			s.Data = append(s.Data, 0x00)
		}
		if s.Channels > 1 {
			frameSize := int(s.Channels) * 2
			frames := len(s.Data) / frameSize
			samples = make([]int16, frames)
			for i := 0; i < frames; i++ {
				off := i * frameSize
				samples[i] = int16(binary.BigEndian.Uint16(s.Data[off : off+2]))
			}
		} else {
			samples = make([]int16, len(s.Data)/2)
			for i := 0; i < len(samples); i++ {
				samples[i] = int16(binary.BigEndian.Uint16(s.Data[2*i : 2*i+2]))
			}
		}
	default:
		log.Fatalf("Invalid number of bits: %v: ID: %v", s.Bits, id)
	}

	if srcRate != dstRate {
		if useHighQuality {
			samples = ResampleLanczosInt16PadDB(samples, srcRate, dstRate, dbPad)
		} else {
			samples = ResampleLinearInt16(samples, srcRate, dstRate)
			samples = PadDB(samples, dbPad)
		}
	} else {
		samples = PadDB(samples, dbPad)
	}
	defer putInt16Buf(samples) // return pooled buffer

	applyFadeInOut(samples, dstRate)

	pcm := make([]byte, len(samples)*2)
	for i, v := range samples {
		pcm[2*i] = byte(v)
		pcm[2*i+1] = byte(v >> 8)
	}

	if sndDump {
		dumpSound(id, s, pcm, dstRate)
	}

	soundMu.Lock()
	pcmCache[id] = pcm
	soundMu.Unlock()
	//logDebug("loadSound(%d) cached %d bytes", id, len(pcm))
	return pcm
}

func dumpSound(id uint16, s *clsnd.Sound, pcm []byte, rate int) {
	if isWASM {
		return
	}
	sndDumpOnce.Do(func() {
		os.MkdirAll(filepath.Join("dump", "snd"), 0755)
		if f, err := os.Create(filepath.Join("dump", "snd", "metadata.csv")); err == nil {
			sndMetaWriter = csv.NewWriter(f)
			sndMetaWriter.Write([]string{"id", "origRate", "origChannels", "origBits", "bytes"})
		}
	})
	sndDumpMu.Lock()
	if _, ok := dumpedSndIDs[id]; ok {
		sndDumpMu.Unlock()
		return
	}
	dumpedSndIDs[id] = struct{}{}
	sndDumpMu.Unlock()

	fn := filepath.Join("dump", "snd", fmt.Sprintf("%d.wav", id))
	f, err := os.Create(fn)
	if err != nil {
		log.Printf("dump sound %d: %v", id, err)
		return
	}
	defer f.Close()

	dataLen := uint32(len(pcm))
	var header [44]byte
	copy(header[0:], []byte("RIFF"))
	binary.LittleEndian.PutUint32(header[4:], 36+dataLen)
	copy(header[8:], []byte("WAVE"))
	copy(header[12:], []byte("fmt "))
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], 1)
	binary.LittleEndian.PutUint16(header[22:], 1)
	binary.LittleEndian.PutUint32(header[24:], uint32(rate))
	binary.LittleEndian.PutUint32(header[28:], uint32(rate*2))
	binary.LittleEndian.PutUint16(header[32:], 2)
	binary.LittleEndian.PutUint16(header[34:], 16)
	copy(header[36:], []byte("data"))
	binary.LittleEndian.PutUint32(header[40:], dataLen)

	if _, err := f.Write(header[:]); err != nil {
		log.Printf("dump sound %d header: %v", id, err)
		return
	}
	if _, err := f.Write(pcm); err != nil {
		log.Printf("dump sound %d data: %v", id, err)
		return
	}

	if sndMetaWriter != nil {
		sndMetaWriter.Write([]string{
			strconv.Itoa(int(id)),
			strconv.FormatUint(uint64(s.SampleRate), 10),
			strconv.FormatUint(uint64(s.Channels), 10),
			strconv.FormatUint(uint64(s.Bits), 10),
			strconv.Itoa(len(pcm)),
		})
		sndMetaWriter.Flush()
	}
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
