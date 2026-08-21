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
	"runtime"
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
	return resp.ContentLength
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
	var err error
	for _, b := range bases {
		if imgOld > 0 && experimental {
			patchURL := fmt.Sprintf("%v/data/CL_Images.%dto%d.gz", b, imgOld, imgVer)
			if err = downloadPatch(patchURL, imgPath, climg.ApplyPatch); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					imgURL := fmt.Sprintf("%v/data/CL_Images.%d.gz", b, imgVer)
					err = downloadGZ(imgURL, imgPath)
				}
			}
		} else {
			imgURL := fmt.Sprintf("%v/data/CL_Images.%d.gz", b, imgVer)
			err = downloadGZ(imgURL, imgPath)
		}
		if err == nil {
			break
		}
	}
	if err != nil {
		logError("update CL_Images: %v", err)
		return 0, err
	}
	sndPath := filepath.Join(dataDir, CL_SoundsFile)
	var sndOld int
	if old, err := readKeyFileVersion(sndPath); err == nil {
		sndOld = int(old >> 8)
	}
	for _, b := range bases {
		if sndOld > 0 && experimental {
			patchURL := fmt.Sprintf("%v/data/CL_Sounds.%dto%d.gz", b, sndOld, sndVer)
			if err = downloadPatch(patchURL, sndPath, clsnd.ApplyPatch); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					sndURL := fmt.Sprintf("%v/data/CL_Sounds.%d.gz", b, sndVer)
					err = downloadGZ(sndURL, sndPath)
				}
			}
		} else {
			sndURL := fmt.Sprintf("%v/data/CL_Sounds.%d.gz", b, sndVer)
			err = downloadGZ(sndURL, sndPath)
		}
		if err == nil {
			break
		}
	}
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

	imgPath := filepath.Join(dataDirPath, CL_ImagesFile)
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

	sndPath := filepath.Join(dataDirPath, CL_SoundsFile)
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
	if isWASM && status.NeedSounds && (len(wasmCLSoundsData) > 0 || clSounds != nil) {
		status.NeedSounds = false
		if status.SoundVersion < clientVer {
			status.SoundVersion = clientVer
		}
		if status.Version < clientVer {
			status.Version = clientVer
		}
	}

	if status.NeedImages {
		name := fmt.Sprintf("CL_Images.%d.gz", clientVer)
		size := int64(-1)
		for _, b := range assetBases(updateBase, fallbackUpdateBase) {
			size = headSize(fmt.Sprintf("%v/data/%s", b, name))
			if size >= 0 {
				break
			}
		}
		status.Files = append(status.Files, fileInfo{Name: name, Size: size})
	}
	if status.NeedSounds {
		name := fmt.Sprintf("CL_Sounds.%d.gz", clientVer)
		size := int64(-1)
		for _, b := range assetBases(updateBase, fallbackUpdateBase) {
			size = headSize(fmt.Sprintf("%v/data/%s", b, name))
			if size >= 0 {
				break
			}
		}
		status.Files = append(status.Files, fileInfo{Name: name, Size: size})
	}
	// In WASM mode, only offer core assets (images/sounds) to save bandwidth.
	if isWASM {
		return status, nil
	}

	sfPath := filepath.Join(dataDirPath, soundFontFile)
	if _, err := os.Stat(sfPath); errors.Is(err, os.ErrNotExist) {
		status.NeedSoundfont = true
		status.SoundfontSize = headSize(soundFontURL)
	}

	piperDir := filepath.Join(dataDirPath, "piper")
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
		if err := os.MkdirAll(dataDirPath, 0755); err != nil {
			logError("create %v: %v", dataDirPath, err)
			return err
		}
	}
	bases := assetBases(updateBase, fallbackUpdateBase)
	if status.NeedImages {
		imgPath := filepath.Join(dataDirPath, CL_ImagesFile)
		var err error
		for _, base := range bases {
			if status.ImageVersion > 0 && experimental {
				patchURL := fmt.Sprintf("%v/data/CL_Images.%dto%d.gz", base, status.ImageVersion, clientVer)
				if err = downloadPatch(patchURL, imgPath, climg.ApplyPatch); err != nil {
					if errors.Is(err, os.ErrNotExist) {
						imgURL := fmt.Sprintf("%v/data/CL_Images.%d.gz", base, clientVer)
						err = downloadGZ(imgURL, imgPath)
					}
				}
			} else {
				imgURL := fmt.Sprintf("%v/data/CL_Images.%d.gz", base, clientVer)
				err = downloadGZ(imgURL, imgPath)
			}
			if err == nil {
				break
			}
		}
		if err != nil {
			logError("download CL_Images: %v", err)
			return fmt.Errorf("download CL_Images: %w", err)
		}
	}
	if status.NeedSounds {
		sndPath := filepath.Join(dataDirPath, CL_SoundsFile)
		var err error
		for _, base := range bases {
			if status.SoundVersion > 0 && experimental {
				patchURL := fmt.Sprintf("%v/data/CL_Sounds.%dto%d.gz", base, status.SoundVersion, clientVer)
				if err = downloadPatch(patchURL, sndPath, clsnd.ApplyPatch); err != nil {
					if errors.Is(err, os.ErrNotExist) {
						sndURL := fmt.Sprintf("%v/data/CL_Sounds.%d.gz", base, clientVer)
						err = downloadGZ(sndURL, sndPath)
					}
				}
			} else {
				sndURL := fmt.Sprintf("%v/data/CL_Sounds.%d.gz", base, clientVer)
				err = downloadGZ(sndURL, sndPath)
			}
			if err == nil {
				break
			}
		}
		if err != nil {
			logError("download CL_Sounds: %v", err)
			return fmt.Errorf("download CL_Sounds: %w", err)
		}
	}
	if getSoundfont {
		sfPath := filepath.Join(dataDirPath, soundFontFile)
		if err := downloadGZ(soundFontURL, sfPath); err != nil {
			logError("download %v: %v", soundFontURL, err)
			return fmt.Errorf("download soundfont: %w", err)
		}
	}
	piperDir := filepath.Join(dataDirPath, "piper")
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
		if path, model, cfg, err := preparePiper(dataDirPath); err == nil {
			piperPath, piperModel, piperConfig = path, model, cfg
			settingsDirty = true
			go playChatTTS(chatTTSCtx, ttsTestPhrase)
		} else {
			logError("prepare piper: %v", err)
		}
	}
	return nil
}

func downloadPatch(url, dest string, apply func(string, []byte) error) error {
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
	if err := apply(dest, data); err != nil {
		return err
	}
	if downloadStatus != nil {
		downloadStatus(fmt.Sprintf("Patch applied: %s", filepath.Base(dest)))
	}
	return nil
}
