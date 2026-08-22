package main

var wasmPrivacyFlag bool

func enterWasmPrivacyMode() {
	if !isWASM {
		return
	}
	if !wasmPrivacyFlag {
		wasmPrivacyFlag = true
		killNameTagCache()
	}
}

func exitWasmPrivacyMode() {
	if wasmPrivacyFlag {
		wasmPrivacyFlag = false
		killNameTagCache()
	}
}

func wasmPrivacyActive() bool {
	return wasmPrivacyFlag
}
