package main

// getscriptDisplayName safely returns the display name for a script owner.
// Falls back to the owner key if no display name is set.
func getscriptDisplayName(owner string) string {
	scriptMu.RLock()
	name := scriptDisplayNames[owner]
	scriptMu.RUnlock()
	if name == "" {
		return owner
	}
	return name
}
