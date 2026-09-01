package clsnd

import "gothoom/keyfile"

// ApplyPatch applies a classic-client CL_Sounds patch and verifies its target
// version before replacing the archive.
func ApplyPatch(basePath string, patch []byte, expectedMajor uint32) error {
	validate := func(data []byte) error {
		_, err := LoadBytes(data)
		return err
	}
	return keyfile.ApplyPatchValidated(basePath, patch, expectedMajor, validate,
		0x3c566572, // '<Ver' metadata present in official full archives
		0x56657273, // 'Vers'
		typeSound,
	)
}
