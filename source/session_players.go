package main

import (
	"bytes"
	"strings"
	"sync"
	"time"
)

// sessionPlayerState is the decoder-owned player directory for one session.
// The existing primary Players UI remains as an adapter until that window can
// bind directly to the selected session.
type sessionPlayerState struct {
	mu      sync.RWMutex
	players map[string]*Player
}

func newSessionPlayerState() *sessionPlayerState {
	return &sessionPlayerState{players: make(map[string]*Player)}
}

func (s *sessionPlayerState) observeAppearance(name string, pictID uint16, colors []byte, isNPC bool) {
	if s == nil || name == "" || isNPC {
		return
	}
	s.mu.Lock()
	p := s.players[name]
	if p == nil {
		p = &Player{Name: name}
		s.players[name] = p
	}
	p.PictID = pictID
	if !bytes.Equal(p.Colors, colors) {
		p.Colors = append([]byte(nil), colors...)
	}
	p.LastSeen = time.Now()
	p.Offline = false
	s.mu.Unlock()
}

func (s *sessionPlayerState) markOnScreen(mobiles []frameMobile, descriptors map[uint8]frameDescriptor, now time.Time, selfName string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	for _, mobile := range mobiles {
		d, ok := descriptors[mobile.Index]
		if !ok || d.Type == kDescNPC || d.Name == "" || mobile.Persist {
			continue
		}
		p := s.players[d.Name]
		if p == nil {
			p = &Player{Name: d.Name}
			s.players[d.Name] = p
		}
		self := selfName != "" && strings.EqualFold(d.Name, selfName)
		if len(d.Colors) != 0 {
			p.Sharing = !self && mobile.Colors&styleBold != 0
			p.SameClan = mobile.Colors&styleItalic != 0
		}
		if self {
			p.Sharing = false
			p.Sharee = false
		}
		p.LastSeen = now
		p.Offline = false
		if mobileActuallyVisible(mobile, d) {
			p.LastOnScreen = now
		}
	}
	s.mu.Unlock()
}

func (s *sessionPlayerState) player(name string) (Player, bool) {
	if s == nil {
		return Player{}, false
	}
	s.mu.RLock()
	p, ok := s.players[name]
	if !ok {
		s.mu.RUnlock()
		return Player{}, false
	}
	out := *p
	out.Colors = append([]byte(nil), p.Colors...)
	s.mu.RUnlock()
	return out, true
}

func (s *sessionPlayerState) snapshot() []Player {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	out := make([]Player, 0, len(s.players))
	for _, p := range s.players {
		copy := *p
		copy.Colors = append([]byte(nil), p.Colors...)
		out = append(out, copy)
	}
	s.mu.RUnlock()
	return out
}

func (s *sessionPlayerState) reset() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.players = make(map[string]*Player)
	s.mu.Unlock()
}
