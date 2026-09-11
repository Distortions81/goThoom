package main

// Pristine script fingerprints from releases test47 through test57, before
// installed hashes were tracked. Match whole files only to preserve local edits.
var previousBundledScriptHashes = map[string]map[string]bool{
	"follow_player.go": {
		"18abfa6ed80a90040965e657e7e927618b557f0c5ec19db5b8411375679f0cda": true, // test55, test56, test57
	},
	"mark_beasts.go": {
		"73748e70a3a92ab04a32b639d5a39a3fe6b9516ea01fced2bbb8f2b7660ef578": true, // test55, test56, test57
	},
}
