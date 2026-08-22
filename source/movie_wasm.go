//go:build js

package main

import _ "embed"

//go:embed clmovFiles/test.clMov.zip
var wasmEmbeddedMovieZip []byte

func init() {
	wasmMovieZipData = wasmEmbeddedMovieZip
}
