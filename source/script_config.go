package main

import "sync"

type scriptConfigEntry struct {
	Name string
	Type string
}

var (
	scriptConfigMu      sync.RWMutex
	scriptConfigEntries = map[string][]scriptConfigEntry{}
)

func scriptAddConfig(owner, name, typ string) {
	if name == "" || typ == "" {
		return
	}
	scriptConfigMu.Lock()
	scriptConfigEntries[owner] = append(scriptConfigEntries[owner], scriptConfigEntry{Name: name, Type: typ})
	scriptConfigMu.Unlock()
	refreshscriptsWindow()
}

func scriptRemoveConfig(owner string) {
	scriptConfigMu.Lock()
	delete(scriptConfigEntries, owner)
	scriptConfigMu.Unlock()
	refreshscriptsWindow()
}
