package main

import (
	"image/color"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

type cachedNameTagImage struct {
	image         *ebiten.Image
	width, height int
}

const maxSharedNameTags = 4096

var (
	sharedNameTagMu    sync.RWMutex
	sharedNameTagCache = make(map[nameTagKey]cachedNameTagImage)
)

func nameTagFrameColor(name string, opacity uint8) color.RGBA {
	if !gs.NameTagLabelColors {
		return color.RGBA{}
	}
	playersMu.RLock()
	defer playersMu.RUnlock()
	p, ok := players[name]
	if !ok || p.FriendLabel <= 0 || p.FriendLabel > len(labelColors) {
		return color.RGBA{}
	}
	lc := labelColors[p.FriendLabel-1]
	return color.RGBA{R: lc.R, G: lc.G, B: lc.B, A: opacity}
}

func makeNameTagKey(name string, colors, descriptorType, opacity, style uint8, dead bool) nameTagKey {
	// Modern dark tags do not derive their text surface from the health color
	// or descriptor type; the health bar is composed separately at draw time.
	if gs.NameHealthBarModern && gs.DarkBubblesAndNames {
		colors = 0
		descriptorType = 0
	}
	return nameTagKey{
		Text:          name,
		Colors:        colors,
		Type:          descriptorType,
		HealthOptions: nameHealthOptionsKey(),
		Opacity:       opacity,
		FontGen:       fontGen,
		Style:         style,
		Dead:          dead,
		FrameColor:    nameTagFrameColor(name, opacity),
	}
}

func reuseSharedNameTag(m *frameMobile, key nameTagKey) bool {
	sharedNameTagMu.RLock()
	cached, ok := sharedNameTagCache[key]
	sharedNameTagMu.RUnlock()
	if !ok || cached.image == nil {
		return false
	}
	m.nameTag = cached.image
	m.nameTagW = cached.width
	m.nameTagH = cached.height
	m.nameTagKey = key
	return true
}

func sharedNameTagImage(key nameTagKey) (*ebiten.Image, int, int) {
	sharedNameTagMu.Lock()
	defer sharedNameTagMu.Unlock()
	if cached, ok := sharedNameTagCache[key]; ok {
		return cached.image, cached.width, cached.height
	}
	img, width, height := buildNameTagImage(key.Text, key.Colors, key.Type, key.Opacity, key.Style, key.Dead, key.FrameColor)
	if img != nil {
		if len(sharedNameTagCache) >= maxSharedNameTags {
			clear(sharedNameTagCache)
		}
		sharedNameTagCache[key] = cachedNameTagImage{image: img, width: width, height: height}
	}
	return img, width, height
}

func clearSharedNameTagCache() {
	sharedNameTagMu.Lock()
	sharedNameTagCache = make(map[nameTagKey]cachedNameTagImage)
	sharedNameTagMu.Unlock()
}

func clearSharedNameTagCacheFor(name string) {
	sharedNameTagMu.Lock()
	for key := range sharedNameTagCache {
		if key.Text == name {
			delete(sharedNameTagCache, key)
		}
	}
	sharedNameTagMu.Unlock()
}

// killNameTagCache clears all cached mobile name tag images.
func killNameTagCache() {
	stateMu.Lock()
	for idx, m := range state.mobiles {
		m.nameTag = nil
		m.nameTagKey = nameTagKey{}
		state.mobiles[idx] = m
	}
	stateMu.Unlock()
	clearSharedNameTagCache()
}

// killNameTagCacheFor clears the cached name tag for the mobile with the given name.
func killNameTagCacheFor(name string) {
	stateMu.Lock()
	for idx, d := range state.descriptors {
		if d.Name == name {
			if m, ok := state.mobiles[idx]; ok {
				m.nameTag = nil
				m.nameTagKey = nameTagKey{}
				state.mobiles[idx] = m
			}
		}
	}
	stateMu.Unlock()
	clearSharedNameTagCacheFor(name)
}
