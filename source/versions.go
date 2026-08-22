package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/browser"
)

//go:embed data/versions.json
var versionsJSON []byte

//go:embed data/changelog/*
var changelogFS embed.FS

var (
	appVersion int
	clVersion  = baseVersion
	changelog  string

	changelogVersions   []int
	changelogVersionIdx int
)

const versionsURL = "https://m45sci.xyz/u/dist/goThoom/versions.json"

var uiReady bool

// versionEntry mirrors the structure of the entries in data/versions.json.
// The JSON file uses capitalized field names, so the tags here must match
// those exactly in order for decoding to succeed.
type versionEntry struct {
	Version   int `json:"Version"`
	CLVersion int `json:"CLVersion"`
}

type versionFile struct {
	Versions []versionEntry `json:"versions"`
}

func init() {
	var vf versionFile
	if err := json.Unmarshal(versionsJSON, &vf); err != nil {
		log.Printf("parse versions.json: %v", err)
		return
	}
	if len(vf.Versions) == 0 {
		return
	}
	latest := vf.Versions[0]
	for _, v := range vf.Versions[1:] {
		if v.Version > latest.Version {
			latest = v
		}
	}
	appVersion = latest.Version
	if latest.CLVersion != 0 {
		clVersion = latest.CLVersion
	}

	// Discover available changelog versions.
	entries, err := changelogFS.ReadDir("data/changelog")
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".txt")
			if v, err := strconv.Atoi(name); err == nil {
				changelogVersions = append(changelogVersions, v)
			}
		}
		sort.Ints(changelogVersions)
		for i, v := range changelogVersions {
			if v == appVersion {
				changelogVersionIdx = i
				break
			}
		}
	}

	loadChangelogAt(changelogVersionIdx)
	if changelog == "" {
		b, err := changelogFS.ReadFile(fmt.Sprintf("data/changelog/%d.txt", appVersion))
		if err != nil {
			log.Printf("read changelog: %v", err)
		} else {
			changelog = string(b)
		}
	}
}

func loadChangelogAt(idx int) bool {
	if idx < 0 || idx >= len(changelogVersions) {
		return false
	}
	v := changelogVersions[idx]
	b, err := changelogFS.ReadFile(fmt.Sprintf("data/changelog/%d.txt", v))
	if err != nil {
		log.Printf("read changelog: %v", err)
		return false
	}
	changelog = string(b)
	changelogVersionIdx = idx
	return true
}

func checkForNewVersion() {
	gs.LastUpdateCheck = time.Now()
	settingsDirty = true

	resp, err := http.Get(versionsURL)
	if err != nil {
		log.Printf("check new version: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("check new version: %v", resp.Status)
		return
	}
	var vf versionFile
	if err := json.NewDecoder(resp.Body).Decode(&vf); err != nil {
		log.Printf("check new version: %v", err)
		return
	}
	if len(vf.Versions) == 0 {
		return
	}
	latest := vf.Versions[0]
	for _, v := range vf.Versions[1:] {
		if v.Version > latest.Version {
			latest = v
		}
	}
	if latest.Version > appVersion {
		consoleMessage(fmt.Sprintf("New goThoom version %d available", latest.Version))
		if tcpConn != nil {
			if gs.NotifiedVersion >= latest.Version {
				return
			}
			gs.NotifiedVersion = latest.Version
			settingsDirty = true
			go func(ver int) {
				for !uiReady {
					time.Sleep(100 * time.Millisecond)
				}
				showNotification(fmt.Sprintf("goThoom version %d is available!", ver))
			}(latest.Version)
			return
		}
		gs.NotifiedVersion = latest.Version
		settingsDirty = true
		go func(ver int) {
			for !uiReady {
				time.Sleep(100 * time.Millisecond)
			}
			showPopup(
				"Update Available",
				fmt.Sprintf("goThoom version %d is available!", ver),
				[]popupButton{
					{Text: "Cancel"},
					{Text: "Download", Action: func() {
						browser.OpenURL("https://github.com/Distortions81/goThoom/releases")
						os.Exit(0)
					}},
				},
			)
		}(latest.Version)
		return
	}
	consoleMessage("This version of goThoom is the latest version!")
}

func versionCheckLoop() {
	for {
		wait := 3*time.Hour - time.Since(gs.LastUpdateCheck)
		if wait > 0 {
			time.Sleep(wait)
		}
		checkForNewVersion()
	}
}
