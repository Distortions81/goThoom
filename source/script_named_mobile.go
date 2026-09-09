package main

import "strings"

func scriptSetNamedMobileEffect(owner, name string, color scriptMobileTint, outline bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return
	}
	overlayMu.Lock()
	effects := scriptNamedMobileTints
	if outline {
		effects = scriptNamedMobileOutlines
	}
	if effects[owner] == nil {
		effects[owner] = map[string]scriptMobileTint{}
	}
	effects[owner][name] = color
	overlayMu.Unlock()
	markWorldRenderChanged()
}

func scriptClearNamedMobileEffect(owner, name string, outline bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	overlayMu.Lock()
	effects := scriptNamedMobileTints
	if outline {
		effects = scriptNamedMobileOutlines
	}
	delete(effects[owner], name)
	if len(effects[owner]) == 0 {
		delete(effects, owner)
	}
	overlayMu.Unlock()
	markWorldRenderChanged()
}

// A name match takes precedence over a sprite match. Within each kind, the
// lexicographically last script ID wins, just like sprite-only effects.
func scriptMobileEffectForMobile(id uint16, name string, outline bool) (scriptMobileTint, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	overlayMu.RLock()
	defer overlayMu.RUnlock()
	named, sprites := scriptNamedMobileTints, scriptMobileTints
	if outline {
		named, sprites = scriptNamedMobileOutlines, scriptMobileOutlines
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
