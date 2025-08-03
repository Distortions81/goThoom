package main

import (
	"encoding/binary"
	"log"
	"path/filepath"
	"sync"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"go_client/clsnd"
)

var (
	soundMu      sync.Mutex
	clSounds     *clsnd.CLSounds
	soundCache   = make(map[uint16]*clsnd.Sound)
	audioContext = audio.NewContext(22050)
	soundPlayers = make(map[*audio.Player]struct{})
	playSound    = func(id uint16) {
		s := loadSound(id)
		if s == nil || audioContext == nil {
			return
		}
		if int(s.SampleRate) != audioContext.SampleRate() {
			// ignore sounds with mismatched sample rate for now
			return
		}

		var pcm []byte
		switch s.Bits {
		case 8:
			pcm = make([]byte, len(s.Data)*2)
			for i, b := range s.Data {
				v := int16(b) - 0x80
				v <<= 8
				pcm[2*i] = byte(v)
				pcm[2*i+1] = byte(v >> 8)
			}
		case 16:
			if len(s.Data)%2 != 0 {
				return
			}
			pcm = make([]byte, len(s.Data))
			for i := 0; i < len(s.Data); i += 2 {
				v := binary.BigEndian.Uint16(s.Data[i : i+2])
				pcm[i] = byte(v)
				pcm[i+1] = byte(v >> 8)
			}
		default:
			return
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
