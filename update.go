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
	gz, err := gzip.NewReader(resp.Body)
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
	// Track progress while decompressing and writing to disk.
	sw := &statusWriter{name: filepath.Base(dest)}
	// Copy while counting bytes to update UI status (decompressed size).
	if _, err := io.Copy(f, readerWithProgress{r: gz, w: sw}); err != nil {
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

// readerWithProgress forwards reads and triggers writer progress updates.
// It exists to ensure periodic UI updates even if the underlying gzip reader
// yields large chunks infrequently.
type readerWithProgress struct {
	r io.Reader
	w *statusWriter
}

func (rp readerWithProgress) Read(p []byte) (int, error) {
	n, err := rp.r.Read(p)
	if n > 0 {
		// Push through the status writer without duplicating data on disk.
		// We only care about counting; the actual disk write is handled by mw.
		rp.w.count(int64(n))
	}
	return n, err
}

// statusWriter implements counting + throttled status updates.
type statusWriter struct {
	last  time.Time
	total int64
	name  string
}

// Count-only; used by readerWithProgress to update bytes without duplicate writes.
func (sw *statusWriter) count(n int64) {
	sw.total += n
	if time.Since(sw.last) >= 200*time.Millisecond {
		if downloadStatus != nil {
			downloadStatus(fmt.Sprintf("Downloading %s: %s", sw.name, humanize.Bytes(uint64(sw.total))))
		}
		sw.last = time.Now()
	}
}

// Write satisfies io.Writer; used with io.MultiWriter to catch bytes written.
func (sw *statusWriter) Write(p []byte) (int, error) {
	n := len(p)
	sw.count(int64(n))
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
