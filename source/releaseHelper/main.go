package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const defaultCLVersion = 1353

const versionsURL = "https://m45sci.xyz/downloads/goThoom/versions.jso"

type FileInfo struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
	URL  string `json:"url"`
}

type Version struct {
	Version   int        `json:"version"`
	CLVersion int        `json:"cl_version"`
	Files     []FileInfo `json:"files"`
}

type VersionFile struct {
	Versions []Version `json:"versions"`
}

func main() {
	var (
		versionPath = flag.String("version-file", "../data/versions.json", "path to versions json")
		binariesDir = flag.String("dataPath", "../binaries", "directory containing release zips")
		baseURL     = flag.String("base-url", "", "base URL for downloading binaries")
		remote      = flag.String("remote", "", "scp target like user@host:/path/")
		changelog   = flag.String("changelog", "", "changelog entry for this release")
		clVer       = flag.Int("cl-version", defaultCLVersion, "Clan Lord client version for this release")
	)
	flag.Parse()

	if *baseURL == "" || *remote == "" {
		fmt.Fprintln(os.Stderr, "base-url and remote are required")
		os.Exit(1)
	}

	vf, err := loadVersionFile(*versionPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "load version file:", err)
		os.Exit(1)
	}
	rvf, err := loadVersionURL(versionsURL)
	if err == nil && len(rvf.Versions) > 0 {
		vf = rvf
	}

	nextVer := 1
	if len(vf.Versions) > 0 {
		nextVer = vf.Versions[len(vf.Versions)-1].Version + 1
	}

	files, err := os.ReadDir(*binariesDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read binaries:", err)
		os.Exit(1)
	}

	var entries []FileInfo
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".zip") {
			continue
		}
		osName, arch, err := parseOSArch(f.Name())
		if err != nil {
			fmt.Fprintln(os.Stderr, "skip", f.Name(), ":", err)
			continue
		}
		entries = append(entries, FileInfo{OS: osName, Arch: arch, URL: joinURL(*baseURL, f.Name())})
	}

	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "no binaries found")
		os.Exit(1)
	}

	vf.Versions = append(vf.Versions, Version{Version: nextVer, CLVersion: *clVer, Files: entries})

	if err := saveVersionFile(*versionPath, vf); err != nil {
		fmt.Fprintln(os.Stderr, "save version file:", err)
		os.Exit(1)
	}

	clPath := filepath.Join(filepath.Dir(*versionPath), "changelog", fmt.Sprintf("%d.txt", nextVer))
	if err := os.MkdirAll(filepath.Dir(clPath), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "make changelog dir:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(clPath, []byte(*changelog), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write changelog:", err)
		os.Exit(1)
	}

	// upload binaries and json
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".zip") {
			continue
		}
		local := filepath.Join(*binariesDir, f.Name())
		if err := scp(local, *remote); err != nil {
			fmt.Fprintln(os.Stderr, "scp", f.Name(), ":", err)
			os.Exit(1)
		}
	}
	if err := scp(*versionPath, *remote); err != nil {
		fmt.Fprintln(os.Stderr, "scp versions.json:", err)
		os.Exit(1)
	}
}

func parseOSArch(name string) (string, string, error) {
	base := strings.TrimSuffix(filepath.Base(name), ".zip")
	parts := strings.Split(base, "_")
	if len(parts) < 3 {
		return "", "", fmt.Errorf("filename %s does not contain os and arch", name)
	}
	osName := parts[len(parts)-2]
	arch := parts[len(parts)-1]
	return osName, arch, nil
}

func joinURL(base, file string) string {
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base + file
}

func loadVersionFile(path string) (VersionFile, error) {
	var vf VersionFile
	b, err := os.ReadFile(path)
	if err != nil {
		return vf, err
	}
	if len(b) == 0 {
		return vf, nil
	}
	if err := json.Unmarshal(b, &vf); err != nil {
		return vf, err
	}
	return vf, nil
}

func saveVersionFile(path string, vf VersionFile) error {
	b, err := json.MarshalIndent(vf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func loadVersionURL(url string) (VersionFile, error) {
	var vf VersionFile
	resp, err := http.Get(url)
	if err != nil {
		return vf, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return vf, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&vf); err != nil {
		return vf, err
	}
	return vf, nil
}

func scp(local, remote string) error {
	cmd := exec.Command("scp", local, remote)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
