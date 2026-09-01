package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gothoom/clsnd"
	"gothoom/keyfile"
)

func updateTestKeyfile(version uint32, entries ...keyfile.Entry) []byte {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, version<<8)
	return keyfile.Build(append(entries, keyfile.Entry{Type: kTypeVersion, ID: 0, Data: data}))
}

func writeUpdateTestGzip(t *testing.T, w http.ResponseWriter, data []byte) {
	t.Helper()
	writer := gzip.NewWriter(w)
	if _, err := writer.Write(data); err != nil {
		t.Errorf("write gzip: %v", err)
		return
	}
	if err := writer.Close(); err != nil {
		t.Errorf("close gzip: %v", err)
	}
}

func TestDownloadPatchAppliesGzippedClassicKeyfile(t *testing.T) {
	const typeSound = 0x736e6420
	dir := t.TempDir()
	path := filepath.Join(dir, CL_SoundsFile)
	if err := os.WriteFile(path, updateTestKeyfile(1,
		keyfile.Entry{Type: typeSound, ID: 10, Data: []byte("old")},
	), 0o640); err != nil {
		t.Fatal(err)
	}
	patch := updateTestKeyfile(2,
		keyfile.Entry{Type: typeSound, ID: 10, Data: []byte("new")},
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeUpdateTestGzip(t, w, patch)
	}))
	defer server.Close()

	if err := downloadPatch(server.URL+"/CL_Sounds.1to2.gz", path, 2, clsnd.ApplyPatch); err != nil {
		t.Fatalf("download patch: %v", err)
	}
	version, err := readKeyFileVersion(path)
	if err != nil {
		t.Fatalf("read patched version: %v", err)
	}
	if got := version >> 8; got != 2 {
		t.Fatalf("patched version = %d, want 2", got)
	}
	sounds, err := clsnd.Load(path)
	if err != nil {
		t.Fatalf("load patched sounds: %v", err)
	}
	if ids := sounds.IDs(); len(ids) != 1 || ids[0] != 10 {
		t.Fatalf("patched sound IDs = %v, want [10]", ids)
	}
}

func TestDownloadAssetUpdateFallsBackToFullAfterPatchFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, CL_SoundsFile)
	base := updateTestKeyfile(1)
	if err := os.WriteFile(path, base, 0o640); err != nil {
		t.Fatal(err)
	}
	full := updateTestKeyfile(3)
	var corruptPatch bytes.Buffer
	patchWriter := gzip.NewWriter(&corruptPatch)
	if _, err := patchWriter.Write(updateTestKeyfile(3)); err != nil {
		t.Fatal(err)
	}
	if err := patchWriter.Close(); err != nil {
		t.Fatal(err)
	}
	corruptPatchBytes := corruptPatch.Bytes()
	corruptPatchBytes[len(corruptPatchBytes)-1] ^= 0xff // invalidate the gzip CRC trailer
	fullRequested := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/data/CL_Sounds.1to3.gz":
			_, _ = w.Write(corruptPatchBytes)
		case "/data/CL_Sounds.3.gz":
			fullRequested = true
			writeUpdateTestGzip(t, w, full)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	err := downloadAssetUpdate([]string{server.URL}, "CL_Sounds", path, 1, 3, clsnd.ApplyPatch)
	if err != nil {
		t.Fatalf("asset update: %v", err)
	}
	if !fullRequested {
		t.Fatal("full archive was not requested after patch failure")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, full) {
		t.Fatal("full archive did not replace the local archive")
	}
}

func TestDownloadAssetUpdateFallsBackToFullWhenPatchMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), CL_SoundsFile)
	if err := os.WriteFile(path, updateTestKeyfile(1), 0o640); err != nil {
		t.Fatal(err)
	}
	full := updateTestKeyfile(2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/data/CL_Sounds.2.gz" {
			writeUpdateTestGzip(t, w, full)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	if err := downloadAssetUpdate([]string{server.URL}, "CL_Sounds", path, 1, 2, clsnd.ApplyPatch); err != nil {
		t.Fatalf("asset update: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, full) {
		t.Fatal("missing patch did not fall back to the full archive")
	}
}

func TestDownloadAssetUpdateAppliesPatchChain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, CL_SoundsFile)
	if err := os.WriteFile(path, updateTestKeyfile(1), 0o640); err != nil {
		t.Fatal(err)
	}
	patch12 := updateTestKeyfile(2)
	patch24 := updateTestKeyfile(4)
	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.Path)
		switch r.URL.Path {
		case "/data/":
			_, _ = w.Write([]byte(`<a href="CL_Sounds.1to2.gz">one</a><a href="CL_Sounds.2to4.gz">two</a>`))
		case "/data/CL_Sounds.1to2.gz":
			writeUpdateTestGzip(t, w, patch12)
		case "/data/CL_Sounds.2to4.gz":
			writeUpdateTestGzip(t, w, patch24)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	err := downloadAssetUpdate([]string{server.URL}, "CL_Sounds", path, 1, 4, clsnd.ApplyPatch)
	if err != nil {
		t.Fatalf("asset update: %v (requests %v)", err, requested)
	}
	version, err := readKeyFileVersion(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := version >> 8; got != 4 {
		t.Fatalf("patched version = %d, want 4", got)
	}
	for _, path := range requested {
		if path == "/data/CL_Sounds.4.gz" {
			t.Fatalf("full archive requested despite complete patch chain: %v", requested)
		}
	}
}

func TestDownloadAssetUpdateUsesFullArchiveWithoutLocalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), CL_SoundsFile)
	full := updateTestKeyfile(5)
	patchRequested := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/data/CL_Sounds.5.gz":
			writeUpdateTestGzip(t, w, full)
		default:
			patchRequested = true
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// A stale caller hint must not cause patching when the local file is gone.
	err := downloadAssetUpdate([]string{server.URL}, "CL_Sounds", path, 4, 5, clsnd.ApplyPatch)
	if err != nil {
		t.Fatalf("asset update: %v", err)
	}
	if patchRequested {
		t.Fatal("patch requested without a local archive")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, full) {
		t.Fatal("downloaded full archive does not match server data")
	}
}

func TestDownloadPatchReturnsNotExistForFallback(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	err := downloadPatch(server.URL+"/missing.gz", filepath.Join(t.TempDir(), CL_SoundsFile), 2, clsnd.ApplyPatch)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing patch error = %v, want os.ErrNotExist", err)
	}
}

func TestAssetUpdateFileInfoPrefersAvailablePatch(t *testing.T) {
	originalUpdateBase := updateBase
	originalWASM := isWASM
	t.Cleanup(func() {
		updateBase = originalUpdateBase
		isWASM = originalWASM
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/data/CL_Images.1to2.gz" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Length", "123")
	}))
	defer server.Close()
	updateBase = server.URL
	isWASM = false

	got := assetUpdateFileInfo("CL_Images", 1, 2)
	if got.Name != "CL_Images.1to2.gz" || got.Size != 123 {
		t.Fatalf("asset info = %#v, want patch with size 123", got)
	}
}
