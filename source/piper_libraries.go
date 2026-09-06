package main

import (
	"debug/elf"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Older installations can contain the executable and libraries but lack the
// symlinks from the archive. Restore each library's own SONAME without assuming
// a particular Piper version or changing the system library search path.
func repairPiperLibraryLinks(dir string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	libraries, err := filepath.Glob(filepath.Join(dir, "lib*.so*"))
	if err != nil {
		return err
	}
	for _, path := range libraries {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		library, err := elf.Open(path)
		if err != nil {
			return err
		}
		names, err := library.DynString(elf.DT_SONAME)
		library.Close()
		if err != nil {
			return err
		}
		for _, name := range names {
			if !filepath.IsLocal(name) || filepath.Base(name) != name {
				return fmt.Errorf("invalid library name %q in %s", name, path)
			}
			link := filepath.Join(dir, name)
			if _, err := os.Lstat(link); err == nil {
				continue // Preserve existing libraries and links.
			} else if !os.IsNotExist(err) {
				return err
			}
			if err := os.Symlink(filepath.Base(path), link); err != nil {
				return err
			}
		}
	}
	return nil
}
