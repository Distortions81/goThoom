package main

import (
	"strings"
	"sync"
	"time"
)

// sessionScriptVisualState keeps script-drawn world state out of the shared
// renderer. The primary session retains the established globals as a
// compatibility adapter; secondary runtimes use one of these stores.
type sessionScriptVisualState struct {
	mu            sync.RWMutex
	overlays      map[string][]overlayOp
	tints         map[string]map[uint16]scriptMobileTint
	outlines      map[string]map[uint16]scriptMobileTint
	namedTints    map[string]map[string]scriptMobileTint
	namedOutlines map[string]map[string]scriptMobileTint
	flashes       map[string]map[uint8]scriptMobileFlash
}

func newSessionScriptVisualState() *sessionScriptVisualState {
	return &sessionScriptVisualState{
		overlays:      make(map[string][]overlayOp),
		tints:         make(map[string]map[uint16]scriptMobileTint),
		outlines:      make(map[string]map[uint16]scriptMobileTint),
		namedTints:    make(map[string]map[string]scriptMobileTint),
		namedOutlines: make(map[string]map[string]scriptMobileTint),
		flashes:       make(map[string]map[uint8]scriptMobileFlash),
	}
}

func sessionScriptVisuals(session *Session) *sessionScriptVisualState {
	if session == nil || session == primarySession || session.automation == nil {
		return nil
	}
	return session.automation.visuals
}

func (v *sessionScriptVisualState) clearOwner(owner string) {
	if v == nil {
		return
	}
	v.mu.Lock()
	delete(v.overlays, owner)
	delete(v.tints, owner)
	delete(v.outlines, owner)
	delete(v.namedTints, owner)
	delete(v.namedOutlines, owner)
	delete(v.flashes, owner)
	v.mu.Unlock()
	markWorldRenderChanged()
}

func (v *sessionScriptVisualState) clearAll() {
	if v == nil {
		return
	}
	v.mu.Lock()
	clear(v.overlays)
	clear(v.tints)
	clear(v.outlines)
	clear(v.namedTints)
	clear(v.namedOutlines)
	clear(v.flashes)
	v.mu.Unlock()
	markWorldRenderChanged()
}

func scriptOverlayClearForSession(session *Session, owner string) {
	if visuals := sessionScriptVisuals(session); visuals != nil {
		visuals.mu.Lock()
		delete(visuals.overlays, owner)
		visuals.mu.Unlock()
		markWorldRenderChanged()
		return
	}
	scriptOverlayClear(owner)
}

func scriptOverlayAppendForSession(session *Session, owner string, op overlayOp) {
	if visuals := sessionScriptVisuals(session); visuals != nil {
		visuals.mu.Lock()
		visuals.overlays[owner] = append(visuals.overlays[owner], op)
		visuals.mu.Unlock()
		markWorldRenderChanged()
		return
	}
	overlayMu.Lock()
	scriptOverlayOps[owner] = append(scriptOverlayOps[owner], op)
	overlayMu.Unlock()
	markWorldRenderChanged()
}

func scriptOverlayRectForSession(session *Session, owner string, x, y, w, h int, r, g, b, a uint8) {
	if w <= 0 || h <= 0 {
		return
	}
	scriptOverlayAppendForSession(session, owner, overlayOp{kind: overlayRectKind, x: x, y: y, w: w, h: h, r: r, g: g, b: b, a: a})
}

func scriptOverlayCircleForSession(session *Session, owner string, x, y, radius int, r, g, b, a uint8) {
	if radius <= 0 {
		return
	}
	scriptOverlayAppendForSession(session, owner, overlayOp{kind: overlayCircleKind, x: x, y: y, radius: radius, r: r, g: g, b: b, a: a})
}

func scriptOverlayTextForSession(session *Session, owner string, x, y int, text string, r, g, b, a uint8) {
	if text == "" {
		return
	}
	scriptOverlayAppendForSession(session, owner, overlayOp{kind: overlayTextKind, x: x, y: y, text: text, r: r, g: g, b: b, a: a})
}

func scriptOverlayImageForSession(session *Session, owner string, id uint16, x, y int) {
	if id == 0 || id == 0xffff {
		return
	}
	scriptOverlayAppendForSession(session, owner, overlayOp{kind: overlayImageKind, x: x, y: y, id: id, r: 255, g: 255, b: 255, a: 255})
}

func scriptOverlayFollowForSession(session *Session, owner string, op overlayOp) {
	if op.expiresAt.IsZero() {
		return
	}
	if visuals := sessionScriptVisuals(session); visuals != nil {
		visuals.mu.Lock()
		list := visuals.overlays[owner]
		for index := range list {
			existing := &list[index]
			if existing.kind == overlayFollowCircleKind && existing.follow.kind == op.follow.kind &&
				existing.follow.pictID == op.follow.pictID && existing.follow.mobileID == op.follow.mobileID &&
				strings.EqualFold(existing.follow.name, op.follow.name) {
				list[index] = op
				visuals.overlays[owner] = list
				visuals.mu.Unlock()
				markWorldRenderChanged()
				return
			}
		}
		visuals.overlays[owner] = append(list, op)
		visuals.mu.Unlock()
		markWorldRenderChanged()
		return
	}
	scriptOverlayUpsertFollow(owner, op)
}

func scriptOverlayFollowPlayerForSession(session *Session, owner, name string, x, y, radius int, r, g, b, a uint8, maxLife time.Duration) {
	if radius <= 0 || strings.TrimSpace(name) == "" {
		return
	}
	scriptOverlayFollowForSession(session, owner, overlayOp{
		kind:   overlayFollowCircleKind,
		follow: overlayFollowOp{kind: overlayFollowKindPlayer, name: name, offsetX: x, offsetY: y},
		radius: radius, r: r, g: g, b: b, a: a, expiresAt: scriptFollowExpireAt(maxLife),
	})
}

func scriptOverlayFollowMobileForSession(session *Session, owner string, index uint8, x, y, radius int, r, g, b, a uint8, maxLife time.Duration) {
	if radius <= 0 {
		return
	}
	scriptOverlayFollowForSession(session, owner, overlayOp{
		kind:   overlayFollowCircleKind,
		follow: overlayFollowOp{kind: overlayFollowKindMobile, mobileID: index, offsetX: x, offsetY: y},
		radius: radius, r: r, g: g, b: b, a: a, expiresAt: scriptFollowExpireAt(maxLife),
	})
}

func scriptOverlayFollowBackgroundForSession(session *Session, owner string, pictID uint16, x, y, radius int, r, g, b, a uint8, maxLife time.Duration) {
	if pictID == 0 || radius <= 0 {
		return
	}
	scriptOverlayFollowForSession(session, owner, overlayOp{
		kind:   overlayFollowCircleKind,
		follow: overlayFollowOp{kind: overlayFollowKindBackground, pictID: pictID, offsetX: x, offsetY: y},
		radius: radius, r: r, g: g, b: b, a: a, expiresAt: scriptFollowExpireAt(maxLife),
	})
}

func scriptSetMobileEffectForSession(session *Session, owner string, id uint16, effect scriptMobileTint, outline bool) {
	if id == 0 || id == 0xffff {
		return
	}
	if visuals := sessionScriptVisuals(session); visuals != nil {
		visuals.mu.Lock()
		effects := visuals.tints
		if outline {
			effects = visuals.outlines
		}
		if effects[owner] == nil {
			effects[owner] = make(map[uint16]scriptMobileTint)
		}
		effects[owner][id] = effect
		visuals.mu.Unlock()
		markWorldRenderChanged()
		return
	}
	if outline {
		scriptSetMobileOutline(owner, id, effect.r, effect.g, effect.b, effect.a)
	} else {
		scriptSetMobileTint(owner, id, effect.r, effect.g, effect.b, effect.a)
	}
}

func scriptClearMobileEffectForSession(session *Session, owner string, id uint16, outline bool) {
	if visuals := sessionScriptVisuals(session); visuals != nil {
		visuals.mu.Lock()
		effects := visuals.tints
		if outline {
			effects = visuals.outlines
		}
		delete(effects[owner], id)
		if len(effects[owner]) == 0 {
			delete(effects, owner)
		}
		visuals.mu.Unlock()
		markWorldRenderChanged()
		return
	}
	if outline {
		scriptClearMobileOutline(owner, id)
	} else {
		scriptClearMobileTint(owner, id)
	}
}

func scriptClearMobileEffectsForSession(session *Session, owner string, outline bool) {
	if visuals := sessionScriptVisuals(session); visuals != nil {
		visuals.mu.Lock()
		if outline {
			delete(visuals.outlines, owner)
			delete(visuals.namedOutlines, owner)
		} else {
			delete(visuals.tints, owner)
			delete(visuals.namedTints, owner)
		}
		visuals.mu.Unlock()
		markWorldRenderChanged()
		return
	}
	if outline {
		scriptClearMobileOutlines(owner)
	} else {
		scriptClearMobileTints(owner)
	}
}

func scriptSetNamedMobileEffectForSession(session *Session, owner, name string, effect scriptMobileTint, outline bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return
	}
	if visuals := sessionScriptVisuals(session); visuals != nil {
		visuals.mu.Lock()
		effects := visuals.namedTints
		if outline {
			effects = visuals.namedOutlines
		}
		if effects[owner] == nil {
			effects[owner] = make(map[string]scriptMobileTint)
		}
		effects[owner][name] = effect
		visuals.mu.Unlock()
		markWorldRenderChanged()
		return
	}
	scriptSetNamedMobileEffect(owner, name, effect, outline)
}

func scriptClearNamedMobileEffectForSession(session *Session, owner, name string, outline bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if visuals := sessionScriptVisuals(session); visuals != nil {
		visuals.mu.Lock()
		effects := visuals.namedTints
		if outline {
			effects = visuals.namedOutlines
		}
		delete(effects[owner], name)
		if len(effects[owner]) == 0 {
			delete(effects, owner)
		}
		visuals.mu.Unlock()
		markWorldRenderChanged()
		return
	}
	scriptClearNamedMobileEffect(owner, name, outline)
}

func scriptFlashMobileForSession(session *Session, owner string, index uint8, r, g, b, a uint8, duration time.Duration) {
	if duration <= 0 {
		return
	}
	if visuals := sessionScriptVisuals(session); visuals != nil {
		visuals.mu.Lock()
		if visuals.flashes[owner] == nil {
			visuals.flashes[owner] = make(map[uint8]scriptMobileFlash)
		}
		visuals.flashes[owner][index] = scriptMobileFlash{
			scriptMobileTint: scriptMobileTint{r: r, g: g, b: b, a: a},
			expires:          time.Now().Add(duration),
		}
		visuals.mu.Unlock()
		markWorldRenderChanged()
		return
	}
	scriptFlashMobile(owner, index, r, g, b, a, duration)
}

func (v *sessionScriptVisualState) overlaySnapshot() []overlayOp {
	if v == nil {
		return nil
	}
	v.mu.RLock()
	ops := make([]overlayOp, 0, 64)
	for _, owned := range v.overlays {
		ops = append(ops, owned...)
	}
	v.mu.RUnlock()
	return ops
}

func (v *sessionScriptVisualState) mobileEffect(id uint16, name string, outline bool) (scriptMobileTint, bool) {
	if v == nil {
		return scriptMobileTint{}, false
	}
	name = strings.ToLower(strings.TrimSpace(name))
	v.mu.RLock()
	defer v.mu.RUnlock()
	named, sprites := v.namedTints, v.tints
	if outline {
		named, sprites = v.namedOutlines, v.outlines
	}
	var owner string
	var effect scriptMobileTint
	if name != "" {
		for candidate, effects := range named {
			if value, ok := effects[name]; ok && candidate > owner {
				owner, effect = candidate, value
			}
		}
	}
	if owner != "" {
		return effect, true
	}
	for candidate, effects := range sprites {
		if value, ok := effects[id]; ok && candidate > owner {
			owner, effect = candidate, value
		}
	}
	return effect, owner != ""
}

func (v *sessionScriptVisualState) mobileFlash(index uint8, now time.Time) (scriptMobileTint, bool) {
	if v == nil {
		return scriptMobileTint{}, false
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	var owner string
	var effect scriptMobileTint
	for candidate, flashes := range v.flashes {
		flash, ok := flashes[index]
		if !ok {
			continue
		}
		if !flash.expires.After(now) {
			delete(flashes, index)
			if len(flashes) == 0 {
				delete(v.flashes, candidate)
			}
			continue
		}
		if candidate > owner {
			owner, effect = candidate, flash.scriptMobileTint
		}
	}
	return effect, owner != ""
}

func (v *sessionScriptVisualState) flashesActive(now time.Time) (active, expired bool) {
	if v == nil {
		return false, false
	}
	v.mu.Lock()
	for owner, flashes := range v.flashes {
		for index, flash := range flashes {
			if flash.expires.After(now) {
				active = true
				continue
			}
			delete(flashes, index)
			expired = true
		}
		if len(flashes) == 0 {
			delete(v.flashes, owner)
		}
	}
	v.mu.Unlock()
	return active, expired
}

func scriptMobileEffectForSession(session *Session, id uint16, name string, outline bool) (scriptMobileTint, bool) {
	if visuals := sessionScriptVisuals(session); visuals != nil {
		return visuals.mobileEffect(id, name, outline)
	}
	return scriptMobileEffectForMobile(id, name, outline)
}

func scriptMobileFlashForSession(session *Session, index uint8) (scriptMobileTint, bool) {
	if visuals := sessionScriptVisuals(session); visuals != nil {
		return visuals.mobileFlash(index, time.Now())
	}
	return scriptMobileFlashForIndex(index)
}

func scriptMobileFlashesActiveForSession(session *Session) bool {
	if visuals := sessionScriptVisuals(session); visuals != nil {
		active, expired := visuals.flashesActive(time.Now())
		if expired {
			markWorldRenderChanged()
		}
		return active || expired
	}
	return scriptMobileFlashesActive()
}

func scriptSessionForID(id SessionID) *Session {
	if session, ok := appSessions.session(id); ok {
		return session
	}
	return primarySession
}
