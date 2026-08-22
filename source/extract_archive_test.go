package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func createTar(entries []tar.Header, contents [][]byte) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for i, hdr := range entries {
		if err := tw.WriteHeader(&hdr); err != nil {
			return nil, err
		}
		if (hdr.Typeflag == tar.TypeReg || hdr.Typeflag == 0) && len(contents) > i {
			if _, err := tw.Write(contents[i]); err != nil {
				return nil, err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func createZip(files []struct {
	hdr  *zip.FileHeader
	data []byte
}) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, err := zw.CreateHeader(f.hdr)
		if err != nil {
			return nil, err
		}
		if len(f.data) > 0 {
			if _, err := w.Write(f.data); err != nil {
				return nil, err
			}
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestExtractArchiveTarTraversal(t *testing.T) {
	dir := t.TempDir()
	hdr := tar.Header{Name: "../evil.txt", Mode: 0o600, Size: int64(len("x"))}
	data, err := createTar([]tar.Header{hdr}, [][]byte{[]byte("x")})
	if err != nil {
		t.Fatalf("createTar: %v", err)
	}
	src := filepath.Join(dir, "bad.tar.gz")
	if err := os.WriteFile(src, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	dst := filepath.Join(dir, "out")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := extractArchive(src, dst); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := os.Stat(filepath.Join(dir, "evil.txt")); err == nil {
		t.Fatalf("evil file extracted")
	}
}

func TestExtractArchiveTarSymlinkTraversal(t *testing.T) {
	dir := t.TempDir()
	hdr := tar.Header{Name: "link", Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "../evil"}
	data, err := createTar([]tar.Header{hdr}, nil)
	if err != nil {
		t.Fatalf("createTar: %v", err)
	}
	src := filepath.Join(dir, "bad.tar.gz")
	if err := os.WriteFile(src, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	dst := filepath.Join(dir, "out")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := extractArchive(src, dst); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := os.Lstat(filepath.Join(dst, "link")); err == nil {
		t.Fatalf("symlink created")
	}
}

func TestExtractArchiveZipTraversal(t *testing.T) {
	dir := t.TempDir()
	fh := &zip.FileHeader{Name: "../evil.txt", Method: zip.Store}
	fh.SetMode(0o600)
	data, err := createZip([]struct {
		hdr  *zip.FileHeader
		data []byte
	}{{fh, []byte("x")}})
	if err != nil {
		t.Fatalf("createZip: %v", err)
	}
	src := filepath.Join(dir, "bad.zip")
	if err := os.WriteFile(src, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	dst := filepath.Join(dir, "out")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := extractArchive(src, dst); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := os.Stat(filepath.Join(dir, "evil.txt")); err == nil {
		t.Fatalf("evil file extracted")
	}
}

func TestExtractArchiveZipSymlinkTraversal(t *testing.T) {
	dir := t.TempDir()
	fh := &zip.FileHeader{Name: "link", Method: zip.Store}
	fh.SetMode(os.ModeSymlink | 0o777)
	data, err := createZip([]struct {
		hdr  *zip.FileHeader
		data []byte
	}{{fh, []byte("../evil")}})
	if err != nil {
		t.Fatalf("createZip: %v", err)
	}
	src := filepath.Join(dir, "bad.zip")
	if err := os.WriteFile(src, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	dst := filepath.Join(dir, "out")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := extractArchive(src, dst); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := os.Lstat(filepath.Join(dst, "link")); err == nil {
		t.Fatalf("symlink created")
	}
}
