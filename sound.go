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
	soundMu      sync.Mutex
	clSounds     *clsnd.CLSounds
	soundCache   = make(map[uint16]*clsnd.Sound)
	audioContext = audio.NewContext(44100)
	soundPlayers = make(map[*audio.Player]struct{})
	playSound    = func(id uint16) {
		logError("sound: %v", id)
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
			samples = resampleLinear(samples, srcRate, dstRate)
		}

		pcm := make([]byte, len(samples)*2)
		for i, v := range samples {
			pcm[2*i] = byte(v)
			pcm[2*i+1] = byte(v >> 8)
		}

		p := audioContext.NewPlayerFromBytes(pcm)
		p.Play()

		soundMu.Lock()
		soundPlayers[p] = struct{}{}
		for sp := range soundPlayers {
			if !sp.IsPlaying() {
				sp.Close()
				delete(soundPlayers, sp)
			}
		}
		soundMu.Unlock()
	}
)

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
