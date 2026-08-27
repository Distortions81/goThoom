package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectoryScriptPackageUsesJailedAssets(t *testing.T) {
	scriptsDir := t.TempDir()
	packageDir := filepath.Join(scriptsDir, "rangery")
	if err := os.MkdirAll(filepath.Join(packageDir, "icons", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageDir, "rangery.go"), []byte("package main\nfunc Init() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	icon := []byte("png data")
	if err := os.WriteFile(filepath.Join(packageDir, "icons", "items", "shield.png"), icon, 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(scriptsDir, "outside.png")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}

	packages := discoverScriptPackages(scriptsDir)
	if len(packages) != 1 {
		t.Fatalf("discovered packages = %+v", packages)
	}
	script := packages[0]
	if script.err != nil || script.fallbackName != "rangery" || !strings.HasSuffix(script.sourcePath, filepath.Join("rangery", "rangery.go")) {
		t.Fatalf("directory script = %+v", script)
	}
	got, err := script.assets.read("icons/items/shield.png")
	if err != nil || string(got) != string(icon) {
		t.Fatalf("read nested asset = %q, %v", got, err)
	}
	for _, path := range []string{"../outside.png", "/outside.png", "icons/../../outside.png", `icons\items\shield.png`} {
		if _, err := script.assets.read(path); err == nil {
			t.Errorf("unsafe asset path %q was accepted", path)
		}
	}

	link := filepath.Join(packageDir, "icons", "escape.png")
	if err := os.Symlink(outside, link); err == nil {
		if _, err := script.assets.read("icons/escape.png"); err == nil {
			t.Fatal("asset symlink escaped the script root")
		}
	}
}

func TestZipScriptPackageActsAsReadOnlyScriptDirectory(t *testing.T) {
	scriptsDir := t.TempDir()
	zipPath := filepath.Join(scriptsDir, "healer-tools.zip")
	writeScriptZip(t, zipPath, map[string]string{
		"main.go":                "package main\nfunc Init() {}\n",
		"icons/actions/heal.png": "healing icon",
	})

	packages := discoverScriptPackages(scriptsDir)
	if len(packages) != 1 {
		t.Fatalf("discovered packages = %+v", packages)
	}
	script := packages[0]
	if script.err != nil || script.fallbackName != "healer-tools" || !strings.HasSuffix(script.sourcePath, filepath.Join("healer-tools.zip", "main.go")) {
		t.Fatalf("ZIP script = %+v", script)
	}
	got, err := script.assets.read("icons/actions/heal.png")
	if err != nil || string(got) != "healing icon" {
		t.Fatalf("read ZIP asset = %q, %v", got, err)
	}
	if _, err := script.assets.read("../main.go"); err == nil {
		t.Fatal("ZIP asset traversal was accepted")
	}
}

func TestScriptScannerLoadsFolderAndZipPackages(t *testing.T) {
	scriptsDir := t.TempDir()
	folder := filepath.Join(scriptsDir, "folder-script")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "entry.go"), []byte(`package main
const scriptID = "folder-id"
const scriptName = "Folder Script"
func Init() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeScriptZip(t, filepath.Join(scriptsDir, "zip-script.zip"), map[string]string{
		"script.go": `package main
const scriptID = "zip-id"
const scriptName = "ZIP Script"
func Init() {}
`,
		"icons/icon.png": "icon",
	})

	scanned := scanscripts([]string{scriptsDir}, nil)
	if info := scanned["folder-id"]; info.name != "Folder Script" || info.invalid || info.assets == nil || info.assets.zipped {
		t.Fatalf("folder scan = %+v", info)
	}
	if info := scanned["zip-id"]; info.name != "ZIP Script" || info.invalid || info.assets == nil || !info.assets.zipped {
		t.Fatalf("ZIP scan = %+v", info)
	}
}

func TestScriptPackageRejectsMultipleGoFilesAndUnsafeZipEntries(t *testing.T) {
	scriptsDir := t.TempDir()
	packageDir := filepath.Join(scriptsDir, "multiple")
	if err := os.MkdirAll(filepath.Join(packageDir, "helpers"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, source := range map[string]string{
		"main.go":                            "package main\nfunc Init() {}\n",
		filepath.Join("helpers", "extra.go"): "package main\n",
	} {
		if err := os.WriteFile(filepath.Join(packageDir, path), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	unsafeZip := filepath.Join(scriptsDir, "unsafe.zip")
	writeScriptZip(t, unsafeZip, map[string]string{
		"main.go":     "package main\nfunc Init() {}\n",
		"../icon.png": "escape",
	})

	packages := discoverScriptPackages(scriptsDir)
	if len(packages) != 2 {
		t.Fatalf("discovered packages = %+v", packages)
	}
	errors := map[string]string{}
	for _, script := range packages {
		if script.err != nil {
			errors[script.fallbackName] = script.err.Error()
		}
	}
	if !strings.Contains(errors["multiple"], "exactly one Go file") {
		t.Fatalf("multiple-Go-file error = %q", errors["multiple"])
	}
	if !strings.Contains(errors["unsafe"], "unsafe script package path") {
		t.Fatalf("unsafe ZIP error = %q", errors["unsafe"])
	}
}

func TestScriptPackageFingerprintIncludesAssetChanges(t *testing.T) {
	scriptsDir := t.TempDir()
	packageDir := filepath.Join(scriptsDir, "toolbar")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageDir, "main.go"), []byte("package main\nfunc Init() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	iconPath := filepath.Join(packageDir, "icon.png")
	if err := os.WriteFile(iconPath, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := discoverScriptPackages(scriptsDir)
	if len(before) != 1 {
		t.Fatalf("before asset edit = %+v", before)
	}
	if err := os.WriteFile(iconPath, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := discoverScriptPackages(scriptsDir)
	if len(after) != 1 {
		t.Fatalf("after asset edit = %+v", after)
	}
	if before[0].fingerprint == after[0].fingerprint {
		t.Fatal("asset edit did not change the script package fingerprint")
	}
}

func TestScriptPackageFingerprintChangesWhenContainerIsRenamed(t *testing.T) {
	scriptsDir := t.TempDir()
	oldDir := filepath.Join(scriptsDir, "old-name")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := []byte("package main\nconst scriptID = \"stable-id\"\nfunc Init() {}\n")
	if err := os.WriteFile(filepath.Join(oldDir, "main.go"), source, 0o644); err != nil {
		t.Fatal(err)
	}
	before := discoverScriptPackages(scriptsDir)
	if len(before) != 1 {
		t.Fatalf("before rename = %+v", before)
	}
	newDir := filepath.Join(scriptsDir, "new-name")
	if err := os.Rename(oldDir, newDir); err != nil {
		t.Fatal(err)
	}
	after := discoverScriptPackages(scriptsDir)
	if len(after) != 1 {
		t.Fatalf("after rename = %+v", after)
	}
	if before[0].fingerprint == after[0].fingerprint {
		t.Fatal("renamed package retained its old fingerprint")
	}
}

func writeScriptZip(t *testing.T, filename string, files map[string]string) {
	t.Helper()
	file, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for name, contents := range files {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
