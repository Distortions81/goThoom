package climg

import (
	"os"

	"gothoom/keyfile"
)

// ApplyPatch merges patch data into the CL_Images keyfile at basePath.
// The patch data must be an uncompressed keyfile. The update is written
// atomically: a temporary file is written and replaces the original on
// success.
func ApplyPatch(basePath string, patch []byte) error {
	base, err := os.ReadFile(basePath)
	if err != nil {
		return err
	}
	merged, err := keyfile.Merge(base, patch)
	if err != nil {
		return err
	}
	tmp := basePath + ".tmp"
	if err := os.WriteFile(tmp, merged, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, basePath); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
