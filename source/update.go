package main

import (
	"compress/gzip"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"gothoom/climg"
	"gothoom/clsnd"
)

const defaultUpdateBase = "https://m45sci.xyz/downloads/clanlord"
const fallbackUpdateBase = "https://www.deltatao.com/downloads/clanlord"
const wasmUpdateBase = "https://gothoom.m45sci.xyz"
const soundFontURL = "https://m45sci.xyz/u/dist/goThoom/soundfont.sf2.gz"
const soundFontFile = "soundfont.sf2"
const extraDataBase = "https://m45sci.xyz/u/dist/goThoom/"
const piperVoiceBase = "https://huggingface.co/rhasspy/piper-voices/resolve/main"
const piperFemaleVoice = "en_US-hfc_female-medium"
const piperMaleVoice = "en_US-hfc_male-medium"

var piperVoicePaths = map[string]string{
	piperFemaleVoice: "en/en_US/hfc_female/medium",
	piperMaleVoice:   "en/en_US/hfc_male/medium",
}

var updateBase = defaultUpdateBase

// downloadStatus, when set by the UI, receives human-readable status updates
// during downloads (e.g., connecting, bytes downloaded, completion).
var downloadStatus func(string)

// downloadProgress, when set by the UI, receives byte progress updates.
// total will be <= 0 if unknown.
var downloadProgress func(name string, read, total int64)

// downloadCtx and downloadCancel allow in-flight downloads to be aborted.
var downloadCtx = context.Background()
var downloadCancel context.CancelFunc = func() {}

func assetBases(candidates ...string) []string {
	seen := make(map[string]struct{})
	var bases []string
	add := func(b string) {
		b = strings.TrimRight(b, "/")
		if b == "" {
			return
		}
		if _, ok := seen[b]; ok {
			return
		}
		seen[b] = struct{}{}
		bases = append(bases, b)
	}
	if isWASM {
		add(wasmUpdateBase)
	}
	for _, c := range candidates {
		c = strings.TrimRight(c, "/")
		if c == "" {
			continue
		}
		if isWASM {
			if !strings.Contains(c, "/webgt") {
				add(c + "/webgt")
			}
		}
		add(c)
	}
	return bases
}

func isSilentWASMNetErr(err error) bool {
	if !isWASM || err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "TypeError: Failed to fetch")
}

var downloadGZ = func(url, dest string) error {
	consoleMessage(fmt.Sprintf("Downloading: %v...", url))
	if downloadStatus != nil {
		downloadStatus(fmt.Sprintf("Connecting to %s...", url))
	}

	req, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, url, nil)
	if err != nil {
		if downloadStatus != nil {
			downloadStatus(fmt.Sprintf("Error creating request: %v", err))
		}
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		silent := isSilentWASMNetErr(err)
		if !silent {
			logError("GET %v: %v", url, err)
		}
		if downloadStatus != nil {
			downloadStatus(fmt.Sprintf("Error connecting: %v", err))
		}
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("GET %v: %v", url, resp.Status)
		logError("download %v: %v", url, err)
		if downloadStatus != nil {
			downloadStatus(fmt.Sprintf("HTTP error: %v", resp.Status))
		}
		return err
	}
	// Inform UI that we are connected and initialize progress.
	if downloadStatus != nil {
		// Show a succinct state transition so "Connecting" doesn't linger.
		host := resp.Request.URL.Host
		humanTotal := "unknown"
		if resp.ContentLength > 0 {
			humanTotal = humanize.Bytes(uint64(resp.ContentLength))
		}
		downloadStatus(fmt.Sprintf("Connected to %s — starting download (%s)", host, humanTotal))
	}

	// Set up compressed byte counter for progress percentage and speed/ETA.
	pc := &progCounter{name: filepath.Base(dest), size: resp.ContentLength}
	// Kick the UI once so it can switch the bar from idle to active.
	if downloadProgress != nil {
		downloadProgress(pc.name, 0, pc.size)
	}
	body := io.TeeReader(resp.Body, pc)
	gz, err := gzip.NewReader(body)
	if err != nil {
		logError("gzip reader %v: %v", url, err)
		if downloadStatus != nil {
			downloadStatus(fmt.Sprintf("Error: %v", err))
		}
		return err
	}
	defer gz.Close()
	// For WASM, keep CL_Images/CL_Sounds in-memory instead of writing to disk.
	if isWASM {
		base := filepath.Base(dest)
		if base == CL_ImagesFile || base == CL_SoundsFile {
			data, err := io.ReadAll(gz)
			if err != nil {
				if downloadStatus != nil {
					downloadStatus(fmt.Sprintf("Error: %v", err))
				}
				return err
			}
			if base == CL_ImagesFile {
				wasmCLImagesData = data
			} else {
				wasmCLSoundsData = data
			}
			// Ensure a final 100% progress update when size is known.
			if downloadProgress != nil && pc.size > 0 {
				downloadProgress(pc.name, pc.size, pc.size)
			}
			consoleMessage("Download complete.")
			if downloadStatus != nil {
				downloadStatus(fmt.Sprintf("Download complete: %s", filepath.Base(dest)))
			}
			return nil
		}
	}

	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		logError("create %v: %v", tmp, err)
		if downloadStatus != nil {
			downloadStatus(fmt.Sprintf("Error: %v", err))
		}
		return err
	}
	removeTmp := true
	defer func() {
		if removeTmp {
			os.Remove(tmp)
		}
	}()
	// Copy the payload to disk while the progCounter (on the compressed stream)
	// drives progress updates.
	if _, err := io.Copy(f, gz); err != nil {
		f.Close()
		logError("copy %v: %v", tmp, err)
		if downloadStatus != nil {
			downloadStatus(fmt.Sprintf("Error: %v", err))
		}
		return err
	}
	if err := f.Close(); err != nil {
		logError("close %v: %v", tmp, err)
		if downloadStatus != nil {
			downloadStatus(fmt.Sprintf("Error: %v", err))
		}
		return err
	}
	// Ensure a final 100% progress update when size is known.
	if downloadProgress != nil && pc.size > 0 {
		downloadProgress(pc.name, pc.size, pc.size)
	}
	consoleMessage("Download complete.")
	if downloadStatus != nil {
		downloadStatus(fmt.Sprintf("Download complete: %s", filepath.Base(dest)))
	}
	if err := os.Rename(tmp, dest); err != nil {
		logError("rename %v to %v: %v", tmp, dest, err)
		return err
	}
	removeTmp = false
	return nil
}

var downloadFile = func(url, dest string) error {
	consoleMessage(fmt.Sprintf("Downloading: %v...", url))
	if downloadStatus != nil {
		downloadStatus(fmt.Sprintf("Connecting to %s...", url))
	}

	req, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, url, nil)
	if err != nil {
		if downloadStatus != nil {
			downloadStatus(fmt.Sprintf("Error creating request: %v", err))
		}
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		silent := isSilentWASMNetErr(err)
		if !silent {
			logError("GET %v: %v", url, err)
		}
		if downloadStatus != nil {
			downloadStatus(fmt.Sprintf("Error connecting: %v", err))
		}
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("GET %v: %v", url, resp.Status)
		logError("download %v: %v", url, err)
		if downloadStatus != nil {
			downloadStatus(fmt.Sprintf("HTTP error: %v", resp.Status))
		}
		return err
	}
	if downloadStatus != nil {
		host := resp.Request.URL.Host
		humanTotal := "unknown"
		if resp.ContentLength > 0 {
			humanTotal = humanize.Bytes(uint64(resp.ContentLength))
		}
		downloadStatus(fmt.Sprintf("Connected to %s — starting download (%s)", host, humanTotal))
	}

	pc := &progCounter{name: filepath.Base(dest), size: resp.ContentLength}
	if downloadProgress != nil {
		downloadProgress(pc.name, 0, pc.size)
	}
	body := io.TeeReader(resp.Body, pc)
	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		logError("create %v: %v", tmp, err)
		if downloadStatus != nil {
			downloadStatus(fmt.Sprintf("Error: %v", err))
		}
		return err
	}
	removeTmp := true
	defer func() {
		if removeTmp {
			os.Remove(tmp)
		}
	}()
	if _, err := io.Copy(f, body); err != nil {
		f.Close()
		logError("copy %v: %v", tmp, err)
		if downloadStatus != nil {
			downloadStatus(fmt.Sprintf("Error: %v", err))
		}
		return err
	}
	if err := f.Close(); err != nil {
		logError("close %v: %v", tmp, err)
		if downloadStatus != nil {
			downloadStatus(fmt.Sprintf("Error: %v", err))
		}
		return err
	}
	if downloadProgress != nil && pc.size > 0 {
		downloadProgress(pc.name, pc.size, pc.size)
	}
	consoleMessage("Download complete.")
	if downloadStatus != nil {
		downloadStatus(fmt.Sprintf("Download complete: %s", filepath.Base(dest)))
	}
	if err := os.Rename(tmp, dest); err != nil {
		logError("rename %v to %v: %v", tmp, dest, err)
		return err
	}
	removeTmp = false
	return nil
}

func headSize(url string) int64 {
	resp, err := http.Head(url)
	if err != nil {
		return -1
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return -1
	}
	return resp.ContentLength
}

func assetUpdateFileInfo(name string, currentVersion, targetVersion int) fileInfo {
	fullName := fmt.Sprintf("%s.%d.gz", name, targetVersion)
	for _, base := range assetBases(updateBase, fallbackUpdateBase) {
		if !isWASM && currentVersion > 0 {
			patchName := fmt.Sprintf("%s.%dto%d.gz", name, currentVersion, targetVersion)
			if size := headSize(fmt.Sprintf("%s/data/%s", base, patchName)); size >= 0 {
				return fileInfo{Name: patchName, Size: size}
			}
		}
		if size := headSize(fmt.Sprintf("%s/data/%s", base, fullName)); size >= 0 {
			return fileInfo{Name: fullName, Size: size}
		}
	}
	return fileInfo{Name: fullName, Size: -1}
}

func urlExists(url string) bool {
	resp, err := http.Head(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// progCounter tracks compressed bytes for progress percentage.
type progCounter struct {
	last  time.Time
	total int64
	size  int64
	name  string
}

func (pc *progCounter) Write(p []byte) (int, error) {
	n := len(p)
	pc.total += int64(n)
	if time.Since(pc.last) >= 200*time.Millisecond {
		if downloadProgress != nil {
			downloadProgress(pc.name, pc.total, pc.size)
		}
		pc.last = time.Now()
	}
	return n, nil
}

func autoUpdate(resp []byte, dataDir string) (int, error) {
	if len(resp) < 16 {
		return 0, fmt.Errorf("short response for update")
	}
	if !isWASM {
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			logError("create %v: %v", dataDir, err)
			return 0, err
		}
	}
	base := string(resp[16:])
	if i := strings.IndexByte(base, 0); i >= 0 {
		base = base[:i]
	}
	base = strings.TrimRight(base, "/")
	clientVer := binary.BigEndian.Uint32(resp[4:8])
	logDebug("Client version: %v", clientVer)
	imgVer := int(binary.BigEndian.Uint32(resp[8:12]) >> 8)
	sndVer := int(binary.BigEndian.Uint32(resp[12:16]) >> 8)
	bases := assetBases(base, updateBase, fallbackUpdateBase)
	imgPath := filepath.Join(dataDir, CL_ImagesFile)
	var imgOld int
	if old, err := readKeyFileVersion(imgPath); err == nil {
		imgOld = int(old >> 8)
	}
	err := downloadAssetUpdate(bases, "CL_Images", imgPath, imgOld, imgVer, climg.ApplyPatch)
	if err != nil {
		logError("update CL_Images: %v", err)
		return 0, err
	}
	sndPath := filepath.Join(dataDir, CL_SoundsFile)
	var sndOld int
	if old, err := readKeyFileVersion(sndPath); err == nil {
		sndOld = int(old >> 8)
	}
	err = downloadAssetUpdate(bases, "CL_Sounds", sndPath, sndOld, sndVer, clsnd.ApplyPatch)
	if err != nil {
		logError("update CL_Sounds: %v", err)
		return 0, err
	}
	return int(clientVer >> 8), nil
}

type fileInfo struct {
	Name string
	Size int64
}

type dataFilesStatus struct {
	NeedImages    bool
	NeedSounds    bool
	NeedSoundfont bool
	NeedPiper     bool
	NeedPiperFem  bool
	NeedPiperMale bool
	Files         []fileInfo
	SoundfontSize int64
	PiperSize     int64
	PiperFemSize  int64
	PiperMaleSize int64
	Version       int
	ImageVersion  int
	SoundVersion  int
}

func checkDataFiles(clientVer int) (dataFilesStatus, error) {
	var status dataFilesStatus

	imgPath := assetFilePath(CL_ImagesFile)
	if v, err := readKeyFileVersion(imgPath); err != nil {
		if !os.IsNotExist(err) {
			logError("read %v: %v", imgPath, err)
		}
		status.NeedImages = true
	} else {
		ver := int(v >> 8)
		status.Version = ver
		status.ImageVersion = ver
		if ver < clientVer {
			status.NeedImages = true
		}
	}
	if isWASM && status.NeedImages && (len(wasmCLImagesData) > 0 || clImages != nil) {
		status.NeedImages = false
		status.ImageVersion = clientVer
		if status.Version < clientVer {
			status.Version = clientVer
		}
	}

	sndPath := assetFilePath(CL_SoundsFile)
	if v, err := readKeyFileVersion(sndPath); err != nil {
		if !os.IsNotExist(err) {
			logError("read %v: %v", sndPath, err)
		}
		status.NeedSounds = true
	} else {
		ver := int(v >> 8)
		status.SoundVersion = ver
		if status.Version == 0 || ver > status.Version {
			status.Version = ver
		}
		if ver < clientVer {
			status.NeedSounds = true
		}
	}
	if isWASM && status.NeedSounds && (len(wasmCLSoundsData) > 0 || currentCLSoundsArchive() != nil) {
		status.NeedSounds = false
		if status.SoundVersion < clientVer {
			status.SoundVersion = clientVer
		}
		if status.Version < clientVer {
			status.Version = clientVer
		}
	}

	if status.NeedImages {
		status.Files = append(status.Files, assetUpdateFileInfo("CL_Images", status.ImageVersion, clientVer))
	}
	if status.NeedSounds {
		status.Files = append(status.Files, assetUpdateFileInfo("CL_Sounds", status.SoundVersion, clientVer))
	}
	// In WASM mode, only offer core assets (images/sounds) to save bandwidth.
	if isWASM {
		return status, nil
	}

	sfPath := soundFontPath()
	if _, err := os.Stat(sfPath); errors.Is(err, os.ErrNotExist) {
		status.NeedSoundfont = true
		status.SoundfontSize = headSize(soundFontURL)
	}

	piperDir := piperDirPath()
	binDir := filepath.Join(piperDir, "bin")
	var archiveName, binName string
	switch runtime.GOOS {
	case "linux":
		binName = "piper"
		switch runtime.GOARCH {
		case "amd64":
			archiveName = "piper_linux_x86_64.tar.gz"
		case "arm64":
			archiveName = "piper_linux_aarch64.tar.gz"
		case "arm":
			archiveName = "piper_linux_armv7l.tar.gz"
		}
	case "darwin":
		binName = "piper"
		switch runtime.GOARCH {
		case "amd64":
			archiveName = "piper_macos_x64.tar.gz"
		case "arm64":
			archiveName = "piper_macos_aarch64.tar.gz"
		}
	case "windows":
		binName = "piper.exe"
		archiveName = "piper_windows_amd64.zip"
	}
	exists := func() bool {
		candidates := []string{
			filepath.Join(binDir, binName),
			filepath.Join(binDir, "piper", binName),
		}
		if runtime.GOOS == "windows" {
			candidates = append(candidates, filepath.Join(binDir, "piper", "piper"))
		}
		for _, p := range candidates {
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return true
			}
		}
		return false
	}
	if !exists() {
		if _, err := os.Stat(filepath.Join(piperDir, archiveName)); errors.Is(err, os.ErrNotExist) {
			status.NeedPiper = true
			status.PiperSize = headSize(extraDataBase + archiveName)
		}
	}

	voicesDir := filepath.Join(piperDir, "voices")
	femPath := filepath.Join(voicesDir, piperFemaleVoice, piperFemaleVoice+".onnx")
	if _, err := os.Stat(femPath); errors.Is(err, os.ErrNotExist) {
		status.NeedPiperFem = true
		base := fmt.Sprintf("%s/%s/%s", piperVoiceBase, piperVoicePaths[piperFemaleVoice], piperFemaleVoice)
		tarURL := base + ".tar.gz"
		if urlExists(tarURL) {
			status.PiperFemSize = headSize(tarURL)
		} else {
			url := base + ".onnx"
			status.PiperFemSize = headSize(url)
		}
	}
	malePath := filepath.Join(voicesDir, piperMaleVoice, piperMaleVoice+".onnx")
	if _, err := os.Stat(malePath); errors.Is(err, os.ErrNotExist) {
		status.NeedPiperMale = true
		base := fmt.Sprintf("%s/%s/%s", piperVoiceBase, piperVoicePaths[piperMaleVoice], piperMaleVoice)
		tarURL := base + ".tar.gz"
		if urlExists(tarURL) {
			status.PiperMaleSize = headSize(tarURL)
		} else {
			url := base + ".onnx"
			status.PiperMaleSize = headSize(url)
		}
	}

	return status, nil
}

func downloadDataFiles(clientVer int, status dataFilesStatus, getSoundfont, getPiper, getFem, getMale bool) error {
	if isWASM {
		// Restrict downloads to CL_Images/CL_Sounds only in WASM.
		getSoundfont, getPiper, getFem, getMale = false, false, false, false
	} else {
		if err := os.MkdirAll(assetsDirPath(), 0755); err != nil {
			logError("create %v: %v", assetsDirPath(), err)
			return err
		}
	}
	bases := assetBases(updateBase, fallbackUpdateBase)
	if status.NeedImages {
		imgPath := assetFilePath(CL_ImagesFile)
		err := downloadAssetUpdate(bases, "CL_Images", imgPath, status.ImageVersion, clientVer, climg.ApplyPatch)
		if err != nil {
			logError("download CL_Images: %v", err)
			return fmt.Errorf("download CL_Images: %w", err)
		}
	}
	if status.NeedSounds {
		sndPath := assetFilePath(CL_SoundsFile)
		err := downloadAssetUpdate(bases, "CL_Sounds", sndPath, status.SoundVersion, clientVer, clsnd.ApplyPatch)
		if err != nil {
			logError("download CL_Sounds: %v", err)
			return fmt.Errorf("download CL_Sounds: %w", err)
		}
	}
	if getSoundfont {
		if err := os.MkdirAll(soundFontsDirPath(), 0o755); err != nil {
			return fmt.Errorf("create soundfont directory: %w", err)
		}
		sfPath := soundFontPath()
		if err := downloadGZ(soundFontURL, sfPath); err != nil {
			logError("download %v: %v", soundFontURL, err)
			return fmt.Errorf("download soundfont: %w", err)
		}
	}
	piperDir := piperDirPath()
	voicesDir := filepath.Join(piperDir, "voices")
	if getPiper || getFem || getMale {
		if err := os.MkdirAll(piperDir, 0o755); err != nil {
			logError("create %v: %v", piperDir, err)
			return err
		}
	}
	if getFem || getMale {
		if err := os.MkdirAll(voicesDir, 0o755); err != nil {
			logError("create %v: %v", voicesDir, err)
			return err
		}
	}
	if getPiper {
		var archiveName string
		switch runtime.GOOS {
		case "linux":
			switch runtime.GOARCH {
			case "amd64":
				archiveName = "piper_linux_x86_64.tar.gz"
			case "arm64":
				archiveName = "piper_linux_aarch64.tar.gz"
			case "arm":
				archiveName = "piper_linux_armv7l.tar.gz"
			}
		case "darwin":
			switch runtime.GOARCH {
			case "amd64":
				archiveName = "piper_macos_x64.tar.gz"
			case "arm64":
				archiveName = "piper_macos_aarch64.tar.gz"
			}
		case "windows":
			archiveName = "piper_windows_amd64.zip"
		}
		if archiveName != "" {
			archPath := filepath.Join(piperDir, archiveName)
			if err := downloadFile(extraDataBase+archiveName, archPath); err != nil {
				logError("download %v: %v", archiveName, err)
				return fmt.Errorf("download piper: %w", err)
			}
		}
	}
	if getFem {
		path := piperVoicePaths[piperFemaleVoice]
		base := fmt.Sprintf("%s/%s/%s", piperVoiceBase, path, piperFemaleVoice)
		tarURL := base + ".tar.gz"
		archPath := filepath.Join(voicesDir, piperFemaleVoice+".tar.gz")
		if urlExists(tarURL) {
			if err := downloadFile(tarURL, archPath); err != nil {
				logError("download %v: %v", tarURL, err)
				return fmt.Errorf("download piper female voice: %w", err)
			}
			if err := extractArchive(archPath, voicesDir); err != nil {
				logError("extract %v: %v", archPath, err)
				return fmt.Errorf("extract piper female voice: %w", err)
			}
			_ = os.Remove(archPath)
		} else {
			vdir := filepath.Join(voicesDir, piperFemaleVoice)
			if err := os.MkdirAll(vdir, 0o755); err != nil {
				logError("create %v: %v", vdir, err)
				return fmt.Errorf("create piper female voice dir: %w", err)
			}
			if err := downloadFile(base+".onnx", filepath.Join(vdir, piperFemaleVoice+".onnx")); err != nil {
				logError("download %v.onnx: %v", piperFemaleVoice, err)
				return fmt.Errorf("download piper female voice model: %w", err)
			}
			if err := downloadFile(base+".onnx.json", filepath.Join(vdir, piperFemaleVoice+".onnx.json")); err != nil {
				logError("download %v.onnx.json: %v", piperFemaleVoice, err)
				return fmt.Errorf("download piper female voice config: %w", err)
			}
			_ = downloadFile(fmt.Sprintf("%s/%s/MODEL_CARD", piperVoiceBase, path), filepath.Join(vdir, "MODEL_CARD"))
		}
	}
	if getMale {
		path := piperVoicePaths[piperMaleVoice]
		base := fmt.Sprintf("%s/%s/%s", piperVoiceBase, path, piperMaleVoice)
		tarURL := base + ".tar.gz"
		archPath := filepath.Join(voicesDir, piperMaleVoice+".tar.gz")
		if urlExists(tarURL) {
			if err := downloadFile(tarURL, archPath); err != nil {
				logError("download %v: %v", tarURL, err)
				return fmt.Errorf("download piper male voice: %w", err)
			}
			if err := extractArchive(archPath, voicesDir); err != nil {
				logError("extract %v: %v", archPath, err)
				return fmt.Errorf("extract piper male voice: %w", err)
			}
			_ = os.Remove(archPath)
		} else {
			vdir := filepath.Join(voicesDir, piperMaleVoice)
			if err := os.MkdirAll(vdir, 0o755); err != nil {
				logError("create %v: %v", vdir, err)
				return fmt.Errorf("create piper male voice dir: %w", err)
			}
			if err := downloadFile(base+".onnx", filepath.Join(vdir, piperMaleVoice+".onnx")); err != nil {
				logError("download %v.onnx: %v", piperMaleVoice, err)
				return fmt.Errorf("download piper male voice model: %w", err)
			}
			if err := downloadFile(base+".onnx.json", filepath.Join(vdir, piperMaleVoice+".onnx.json")); err != nil {
				logError("download %v.onnx.json: %v", piperMaleVoice, err)
				return fmt.Errorf("download piper male voice config: %w", err)
			}
			_ = downloadFile(fmt.Sprintf("%s/%s/MODEL_CARD", piperVoiceBase, path), filepath.Join(vdir, "MODEL_CARD"))
		}
	}
	if getPiper || getFem || getMale {
		if path, model, cfg, err := preparePiperDirectory(piperDirPath()); err == nil {
			piperPath, piperModel, piperConfig = path, model, cfg
			settingsDirty = true
			go playChatTTS(chatTTSCtx, ttsTestPhrase)
		} else {
			logError("prepare piper: %v", err)
		}
	}
	return nil
}

type assetPatchStep struct {
	from int
	to   int
}

func downloadAssetUpdate(
	bases []string,
	name, dest string,
	currentVersion, targetVersion int,
	apply func(string, []byte, uint32) error,
) error {
	var lastErr error
	for _, base := range bases {
		current := currentVersion
		if isWASM {
			current = 0
		} else if version, err := readKeyFileVersion(dest); err == nil {
			current = int(version >> 8)
		} else {
			current = 0
		}
		if current >= targetVersion {
			return nil
		}

		if current > 0 {
			patchURL := fmt.Sprintf("%s/data/%s.%dto%d.gz", base, name, current, targetVersion)
			patchErr := downloadPatch(patchURL, dest, targetVersion, apply)
			if patchErr == nil {
				return nil
			}

			if errors.Is(patchErr, os.ErrNotExist) {
				steps := availableAssetPatchChain(base, name, current, targetVersion)
				if len(steps) > 0 {
					patchErr = nil
					for _, step := range steps {
						stepURL := fmt.Sprintf("%s/data/%s.%dto%d.gz", base, name, step.from, step.to)
						if err := downloadPatch(stepURL, dest, step.to, apply); err != nil {
							patchErr = err
							break
						}
					}
					if patchErr == nil {
						return nil
					}
				}
			}
			logDebug("patch %s %d to %d unavailable or failed (%v); downloading full archive", name, current, targetVersion, patchErr)
			if downloadStatus != nil {
				downloadStatus(fmt.Sprintf("Patch unavailable or failed; downloading full %s archive...", name))
			}
		}

		fullURL := fmt.Sprintf("%s/data/%s.%d.gz", base, name, targetVersion)
		lastErr = downloadGZ(fullURL, dest)
		if lastErr == nil {
			return nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no download source available for %s version %d", name, targetVersion)
	}
	return lastErr
}

func availableAssetPatchChain(base, name string, currentVersion, targetVersion int) []assetPatchStep {
	req, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, strings.TrimRight(base, "/")+"/data/", nil)
	if err != nil {
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	listing, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil
	}
	pattern, err := regexp.Compile(regexp.QuoteMeta(name) + `\.(\d+)to(\d+)\.gz`)
	if err != nil {
		return nil
	}
	edges := make(map[int][]int)
	for _, match := range pattern.FindAllSubmatch(listing, -1) {
		from, fromErr := strconv.Atoi(string(match[1]))
		to, toErr := strconv.Atoi(string(match[2]))
		if fromErr != nil || toErr != nil || from < currentVersion || to <= from || to > targetVersion {
			continue
		}
		edges[from] = append(edges[from], to)
	}
	for from := range edges {
		sort.Sort(sort.Reverse(sort.IntSlice(edges[from])))
	}

	type route struct {
		version int
		steps   []assetPatchStep
	}
	queue := []route{{version: currentVersion}}
	visited := map[int]bool{currentVersion: true}
	for len(queue) > 0 {
		candidate := queue[0]
		queue = queue[1:]
		for _, next := range edges[candidate.version] {
			steps := append(append([]assetPatchStep(nil), candidate.steps...), assetPatchStep{from: candidate.version, to: next})
			if next == targetVersion {
				return steps
			}
			if !visited[next] {
				visited[next] = true
				queue = append(queue, route{version: next, steps: steps})
			}
		}
	}
	return nil
}

func downloadPatch(url, dest string, expectedMajor int, apply func(string, []byte, uint32) error) error {
	// Keep it simple: under WASM, skip patching and force full download path.
	if isWASM {
		return os.ErrNotExist
	}
	consoleMessage(fmt.Sprintf("Downloading: %v...", url))
	if downloadStatus != nil {
		downloadStatus(fmt.Sprintf("Connecting to %s...", url))
	}
	req, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return os.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %v: %v", url, resp.Status)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gz.Close()
	data, err := io.ReadAll(gz)
	if err != nil {
		return err
	}
	if err := apply(dest, data, uint32(expectedMajor)); err != nil {
		return err
	}
	if downloadStatus != nil {
		downloadStatus(fmt.Sprintf("Patch applied: %s", filepath.Base(dest)))
	}
	return nil
}
