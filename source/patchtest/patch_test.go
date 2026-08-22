package main_test

import (
	"compress/gzip"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gothoom/climg"
	"gothoom/clsnd"
	"gothoom/keyfile"
)

const kTypeVersion = 0x56657273

func buildKeyfile(version int) []byte {
	v := make([]byte, 4)
	binary.BigEndian.PutUint32(v, uint32(version))
	return keyfile.Build([]keyfile.Entry{{Type: kTypeVersion, ID: 0, Data: v}})
}

func readKeyFileVersion(path string) (uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var hdr [12]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return 0, err
	}
	count := int(binary.BigEndian.Uint32(hdr[2:6]))
	entry := make([]byte, 16)
	for i := 0; i < count; i++ {
		if _, err := io.ReadFull(f, entry); err != nil {
			return 0, err
		}
		off := binary.BigEndian.Uint32(entry[0:4])
		size := binary.BigEndian.Uint32(entry[4:8])
		typ := binary.BigEndian.Uint32(entry[8:12])
		id := binary.BigEndian.Uint32(entry[12:16])
		if typ == kTypeVersion && id == 0 {
			if _, err := f.Seek(int64(off), io.SeekStart); err != nil {
				return 0, err
			}
			buf := make([]byte, size)
			if _, err := io.ReadFull(f, buf); err != nil {
				return 0, err
			}
			v := binary.BigEndian.Uint32(buf)
			if v <= 0xff {
				v <<= 8
			}
			return v, nil
		}
	}
	return 0, os.ErrNotExist
}

func TestApplyPatchImages(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "CL_Images")
	if err := os.WriteFile(base, buildKeyfile(1), 0644); err != nil {
		t.Fatal(err)
	}
	patch := buildKeyfile(2)
	if err := climg.ApplyPatch(base, patch); err != nil {
		t.Fatalf("apply patch: %v", err)
	}
	if v, err := readKeyFileVersion(base); err != nil || int(v>>8) != 2 {
		t.Fatalf("expected version 2, got %d, %v", v>>8, err)
	}
	if _, err := climg.Load(base); err != nil {
		t.Fatalf("load patched: %v", err)
	}
}

func TestApplyPatchSounds(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "CL_Sounds")
	if err := os.WriteFile(base, buildKeyfile(1), 0644); err != nil {
		t.Fatal(err)
	}
	patch := buildKeyfile(2)
	if err := clsnd.ApplyPatch(base, patch); err != nil {
		t.Fatalf("apply patch: %v", err)
	}
	if v, err := readKeyFileVersion(base); err != nil || int(v>>8) != 2 {
		t.Fatalf("expected version 2, got %d, %v", v>>8, err)
	}
	if _, err := clsnd.Load(base); err != nil {
		t.Fatalf("load patched: %v", err)
	}
}

func TestPatchFallback(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "CL_Images")
	if err := os.WriteFile(base, buildKeyfile(1), 0644); err != nil {
		t.Fatal(err)
	}
	full := buildKeyfile(2)

	mux := http.NewServeMux()
	mux.HandleFunc("/data/CL_Images.1to2.gz", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/data/CL_Images.2.gz", func(w http.ResponseWriter, r *http.Request) {
		gz := gzip.NewWriter(w)
		gz.Write(full)
		gz.Close()
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	patchURL := srv.URL + "/data/CL_Images.1to2.gz"
	resp, err := http.Get(patchURL)
	if err != nil {
		t.Fatalf("get patch: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	fullURL := srv.URL + "/data/CL_Images.2.gz"
	resp, err = http.Get(fullURL)
	if err != nil {
		t.Fatalf("get full: %v", err)
	}
	defer resp.Body.Close()
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	data, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("read full: %v", err)
	}
	gz.Close()
	if err := os.WriteFile(base, data, 0644); err != nil {
		t.Fatalf("write full: %v", err)
	}
	if _, err := climg.Load(base); err != nil {
		t.Fatalf("load fallback: %v", err)
	}
	if v, err := readKeyFileVersion(base); err != nil || int(v>>8) != 2 {
		t.Fatalf("expected version 2 after fallback, got %d, %v", v>>8, err)
	}
}
