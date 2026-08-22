package main

// killNameTagCache clears all cached mobile name tag images.
func killNameTagCache() {
	stateMu.Lock()
	for idx, m := range state.mobiles {
		m.nameTag = nil
		m.nameTagKey = nameTagKey{}
		state.mobiles[idx] = m
	}
	stateMu.Unlock()
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
}
