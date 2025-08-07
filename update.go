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
)

const defaultUpdateBase = "https://www.deltatao.com/downloads/clanlord"

func downloadGZ(url, dest string) error {
	defer fmt.Println()
	addMessage(fmt.Sprintf("Downloading: %v...", url))

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %v: %v", url, resp.Status)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gz.Close()
	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, gz); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	addMessage("Download complete.")
	return os.Rename(tmp, dest)
}

func autoUpdate(resp []byte, dataDir string) error {
	if len(resp) < 16 {
		return fmt.Errorf("short response for update")
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
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
	fmt.Println("downloading", imgURL)
	imgPath := filepath.Join(dataDir, "CL_Images")
	if err := downloadGZ(imgURL, imgPath); err != nil {
		return err
	}
	sndURL := fmt.Sprintf("%v/data/CL_Sounds.%d.gz", base, sndVer>>8)
	fmt.Println("downloading", sndURL)
	sndPath := filepath.Join(dataDir, "CL_Sounds")
	if err := downloadGZ(sndURL, sndPath); err != nil {
		return err
	}
	return nil
}

// ensureDataFiles downloads the CL_Images and CL_Sounds archives if they are
// missing from baseDir. The files are fetched for the provided client version
// using the default update server.
func ensureDataFiles(baseDir string, clientVer int) error {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return err
	}

	imgPath := filepath.Join(baseDir, "CL_Images")
	if _, err := os.Stat(imgPath); os.IsNotExist(err) {
		imgURL := fmt.Sprintf("%v/data/CL_Images.%d.gz", defaultUpdateBase, clientVer)
		if err := downloadGZ(imgURL, imgPath); err != nil {
			return fmt.Errorf("download CL_Images: %w", err)
		}
	}

	sndPath := filepath.Join(baseDir, "CL_Sounds")
	if _, err := os.Stat(sndPath); os.IsNotExist(err) {
		sndURL := fmt.Sprintf("%v/data/CL_Sounds.%d.gz", defaultUpdateBase, clientVer)
		if err := downloadGZ(sndURL, sndPath); err != nil {
			return fmt.Errorf("download CL_Sounds: %w", err)
		}
	}
	return nil
}

type dataFilesStatus struct {
	NeedImages bool
	NeedSounds bool
	Files      []string
}

func checkDataFiles(baseDir string, clientVer int) (dataFilesStatus, error) {
	var status dataFilesStatus
	imgPath := filepath.Join(baseDir, "CL_Images")
	if v, err := readKeyFileVersion(imgPath); err != nil {
		if os.IsNotExist(err) {
			status.NeedImages = true
		} else {
			status.NeedImages = true
		}
	} else if int(v>>8) != clientVer {
		status.NeedImages = true
	}

	sndPath := filepath.Join(baseDir, "CL_Sounds")
	if v, err := readKeyFileVersion(sndPath); err != nil {
		if os.IsNotExist(err) {
			status.NeedSounds = true
		} else {
			status.NeedSounds = true
		}
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

func downloadDataFiles(baseDir string, clientVer int, status dataFilesStatus) error {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return err
	}
	if status.NeedImages {
		imgPath := filepath.Join(baseDir, "CL_Images")
		imgURL := fmt.Sprintf("%v/data/CL_Images.%d.gz", defaultUpdateBase, clientVer)
		if err := downloadGZ(imgURL, imgPath); err != nil {
			return fmt.Errorf("download CL_Images: %w", err)
		}
	}
	if status.NeedSounds {
		sndPath := filepath.Join(baseDir, "CL_Sounds")
		sndURL := fmt.Sprintf("%v/data/CL_Sounds.%d.gz", defaultUpdateBase, clientVer)
		if err := downloadGZ(sndURL, sndPath); err != nil {
			return fmt.Errorf("download CL_Sounds: %w", err)
		}
	}
	return nil
}
