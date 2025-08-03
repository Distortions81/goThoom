package main

import (
	"archive/tar"
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

func downloadTGZ(url, destDir string) error {
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
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		path := filepath.Join(destDir, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
	addMessage("Download complete.")
	return nil
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
	fmt.Printf("Client version: %v\n", clientVer)
	imgVer := binary.BigEndian.Uint32(resp[8:12])
	sndVer := binary.BigEndian.Uint32(resp[12:16])
	imgURL := fmt.Sprintf("%v/data/CL_Images.%d.gz", base, imgVer>>8)
	fmt.Println("downloading", imgURL)
	imgPath := filepath.Join(dataDir, "CL_Images")
	if err := downloadGZ(imgURL, imgPath); err != nil {
		alt := fmt.Sprintf("%v/data/CL_Images.tgz", base)
		fmt.Printf("download %v failed: %v; trying %v\n", imgURL, err, alt)
		if err := downloadTGZ(alt, dataDir); err != nil {
			return err
		}
	}
	sndURL := fmt.Sprintf("%v/data/CL_Sounds.%d.gz", base, sndVer>>8)
	fmt.Println("downloading", sndURL)
	sndPath := filepath.Join(dataDir, "CL_Sounds")
	if err := downloadGZ(sndURL, sndPath); err != nil {
		alt := fmt.Sprintf("%v/data/CL_Sounds.tgz", base)
		fmt.Printf("download %v failed: %v; trying %v\n", sndURL, err, alt)
		if err := downloadTGZ(alt, dataDir); err != nil {
			return err
		}
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
			alt := fmt.Sprintf("%v/data/CL_Images.tgz", defaultUpdateBase)
			if err := downloadTGZ(alt, baseDir); err != nil {
				return fmt.Errorf("download CL_Images: %w", err)
			}
		}
	}

	sndPath := filepath.Join(baseDir, "CL_Sounds")
	if _, err := os.Stat(sndPath); os.IsNotExist(err) {
		sndURL := fmt.Sprintf("%v/data/CL_Sounds.%d.gz", defaultUpdateBase, clientVer)
		if err := downloadGZ(sndURL, sndPath); err != nil {
			alt := fmt.Sprintf("%v/data/CL_Sounds.tgz", defaultUpdateBase)
			if err := downloadTGZ(alt, baseDir); err != nil {
				return fmt.Errorf("download CL_Sounds: %w", err)
			}
		}
	}
	return nil
}
