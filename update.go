package main

import (
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

const defaultUpdateBase = "https://m45sci.xyz/downloads/clanlord"

// downloadStatus, when set by the UI, receives human-readable status updates
// during downloads (e.g., connecting, bytes downloaded, completion).
var downloadStatus func(string)

// downloadProgress, when set by the UI, receives byte progress updates.
// total will be <= 0 if unknown.
var downloadProgress func(name string, read, total int64)

var downloadGZ = func(url, dest string) error {
	consoleMessage(fmt.Sprintf("Downloading: %v...", url))
	if downloadStatus != nil {
		downloadStatus(fmt.Sprintf("Connecting to %s...", url))
	}

	resp, err := http.Get(url)
	if err != nil {
		logError("GET %v: %v", url, err)
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

func autoUpdate(resp []byte, dataDir string) error {
	if len(resp) < 16 {
		return fmt.Errorf("short response for update")
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		logError("create %v: %v", dataDir, err)
		return err
	}
	base := string(resp[16:])
	if i := strings.IndexByte(base, 0); i >= 0 {
		base = base[:i]
	}
	base = strings.TrimRight(base, "/")
	clientVer := binary.BigEndian.Uint32(resp[4:8])
	logDebug("Client version: %v", clientVer)
	imgVer := binary.BigEndian.Uint32(resp[8:12])
	sndVer := binary.BigEndian.Uint32(resp[12:16])
	imgURL := fmt.Sprintf("%v/data/CL_Images.%d.gz", base, imgVer>>8)
	logDebug("downloading %v", imgURL)
	imgPath := filepath.Join(dataDir, CL_ImagesFile)
	if err := downloadGZ(imgURL, imgPath); err != nil {
		logError("download %v: %v", imgURL, err)
		return err
	}
	sndURL := fmt.Sprintf("%v/data/CL_Sounds.%d.gz", base, sndVer>>8)
	logDebug("downloading %v", sndURL)
	sndPath := filepath.Join(dataDir, CL_SoundsFile)
	if err := downloadGZ(sndURL, sndPath); err != nil {
		logError("download %v: %v", sndURL, err)
		return err
	}
	return nil
}

type dataFilesStatus struct {
	NeedImages bool
	NeedSounds bool
	Files      []string
}

func checkDataFiles(clientVer int) (dataFilesStatus, error) {
	var status dataFilesStatus
	imgPath := filepath.Join(dataDirPath, CL_ImagesFile)
	if v, err := readKeyFileVersion(imgPath); err != nil {
		if !os.IsNotExist(err) {
			logError("read %v: %v", imgPath, err)
		}
		status.NeedImages = true
	} else if int(v>>8) != clientVer {
		status.NeedImages = true
	}

	sndPath := filepath.Join(dataDirPath, CL_SoundsFile)
	if v, err := readKeyFileVersion(sndPath); err != nil {
		if !os.IsNotExist(err) {
			logError("read %v: %v", sndPath, err)
		}
		status.NeedSounds = true
	} else if int(v>>8) != clientVer {
		status.NeedSounds = true
	}

	if status.NeedImages {
		status.Files = append(status.Files, fmt.Sprintf("CL_Images.%d.gz", clientVer))
	}
	if status.NeedSounds {
		status.Files = append(status.Files, fmt.Sprintf("CL_Sounds.%d.gz", clientVer))
	}
	return status, nil
}

func downloadDataFiles(clientVer int, status dataFilesStatus) error {
	if err := os.MkdirAll(dataDirPath, 0755); err != nil {
		logError("create %v: %v", dataDirPath, err)
		return err
	}
	if status.NeedImages {
		imgPath := filepath.Join(dataDirPath, CL_ImagesFile)
		imgURL := fmt.Sprintf("%v/data/CL_Images.%d.gz", defaultUpdateBase, clientVer)
		if err := downloadGZ(imgURL, imgPath); err != nil {
			logError("download %v: %v", imgURL, err)
			return fmt.Errorf("download CL_Images: %w", err)
		}
	}
	if status.NeedSounds {
		sndPath := filepath.Join(dataDirPath, CL_SoundsFile)
		sndURL := fmt.Sprintf("%v/data/CL_Sounds.%d.gz", defaultUpdateBase, clientVer)
		if err := downloadGZ(sndURL, sndPath); err != nil {
			logError("download %v: %v", sndURL, err)
			return fmt.Errorf("download CL_Sounds: %w", err)
		}
	}
	return nil
}

// plannedDownloadURLs returns the URLs that would be downloaded for the given
// missing data file status and client version.
func plannedDownloadURLs(clientVer int, status dataFilesStatus) []string {
	urls := make([]string, 0, 2)
	if status.NeedImages {
		urls = append(urls, fmt.Sprintf("%v/data/CL_Images.%d.gz", defaultUpdateBase, clientVer))
	}
	if status.NeedSounds {
		urls = append(urls, fmt.Sprintf("%v/data/CL_Sounds.%d.gz", defaultUpdateBase, clientVer))
	}
	return urls
}
