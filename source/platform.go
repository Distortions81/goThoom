package main

import "runtime"

// isWASM is true when running in a WebAssembly (browser) environment.
var isWASM = (runtime.GOOS == "js" || runtime.GOARCH == "wasm")

// In-memory archives when running under WASM.
var wasmCLImagesData []byte
var wasmCLSoundsData []byte
var wasmMovieZipData []byte
