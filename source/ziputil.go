package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"time"
)

// compressZip creates a .zip archive at dst containing the single file src.
func compressZip(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	st, err := in.Stat()
	if err != nil {
		return err
	}

	// Ensure destination directory exists.
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		// Ensure the file is closed even on errors.
		_ = out.Close()
	}()

	zw := zip.NewWriter(out)
	// Ensure writer closes and flushes.
	defer zw.Close()

	hdr, err := zip.FileInfoHeader(st)
	if err != nil {
		return err
	}
	hdr.Name = filepath.Base(src)
	hdr.Method = zip.Deflate
	// Preserve mod time when possible.
	hdr.Modified = st.ModTime().In(time.Local)

	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, in); err != nil {
		return err
	}
	return zw.Close()
}
