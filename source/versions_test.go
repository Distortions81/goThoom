package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestVersionEntriesAndChangelogsStayInSync(t *testing.T) {
	var vf versionFile
	if err := json.Unmarshal(versionsJSON, &vf); err != nil {
		t.Fatalf("parse embedded versions.json: %v", err)
	}
	if len(vf.Versions) == 0 {
		t.Fatal("versions.json has no versions")
	}

	versions := make(map[int]bool, len(vf.Versions))
	previous := 0
	for _, entry := range vf.Versions {
		if entry.Version <= previous {
			t.Fatalf("versions.json is not strictly increasing at version %d", entry.Version)
		}
		if entry.CLVersion <= 0 {
			t.Fatalf("version %d has invalid Clan Lord version %d", entry.Version, entry.CLVersion)
		}
		path := fmt.Sprintf("data/changelog/%d.txt", entry.Version)
		contents, err := changelogFS.ReadFile(path)
		if err != nil {
			t.Fatalf("version %d has no changelog: %v", entry.Version, err)
		}
		if strings.TrimSpace(string(contents)) == "" {
			t.Fatalf("version %d has an empty changelog", entry.Version)
		}
		versions[entry.Version] = true
		previous = entry.Version
	}

	entries, err := changelogFS.ReadDir("data/changelog")
	if err != nil {
		t.Fatalf("read embedded changelogs: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		version, err := strconv.Atoi(strings.TrimSuffix(entry.Name(), ".txt"))
		if err != nil {
			t.Fatalf("invalid changelog filename %q", entry.Name())
		}
		if !versions[version] {
			t.Fatalf("changelog %q has no versions.json entry", entry.Name())
		}
	}
}
