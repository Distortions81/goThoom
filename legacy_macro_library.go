package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	legacyMacroLibraryDirName       = "Library"
	legacyMacroLibrarySelectionName = "enabled.json"
)

// legacyMacroLibraryFS contains the public legacy macro corpus shipped with
// the client. The same source files are parsed by the compatibility tests.
//
//go:embed testdata/legacy_macros/web/*.mac
var legacyMacroLibraryFS embed.FS

// legacyMacroLibraryBundledEntry holds attribution and description metadata.
// The user-facing macro name comes from the source's // Name: comment, not
// this catalog, so users can freely rename a local copy.
type legacyMacroLibraryBundledEntry struct {
	Filename     string
	Description  string
	SourceURL    string
	EmbeddedPath string
	Note         string
}

var legacyMacroBundledLibrary = []legacyMacroLibraryBundledEntry{
	{
		Filename:     "official-example.mac",
		Description:  "Official examples, including directional movement bindings.",
		SourceURL:    "https://www.deltatao.com/clanlord/macros/example_macros.html",
		EmbeddedPath: "testdata/legacy_macros/web/official-example.mac",
	},
	{
		Filename:     "abbreviations.mac",
		Description:  "A large collection of text expansions.",
		SourceURL:    "https://clump.clanlord.net/library/index.php?title=Abbreviations.mac",
		EmbeddedPath: "testdata/legacy_macros/web/abbreviations.mac",
	},
	{
		Filename:     "keys.mac",
		Description:  "Common key bindings.",
		SourceURL:    "https://clump.clanlord.net/library/index.php?title=Keys.mac",
		EmbeddedPath: "testdata/legacy_macros/web/keys.mac",
	},
	{
		Filename:     "quickchain.mac",
		Description:  "Chain-use helper bindings.",
		SourceURL:    "https://clump.clanlord.net/library/index.php?title=Quickchain.mac",
		EmbeddedPath: "testdata/legacy_macros/web/quickchain.mac",
	},
	{
		Filename:     "sunstone.mac",
		Description:  "Sunstone helper macros.",
		SourceURL:    "https://clump.clanlord.net/library/index.php?title=Sunstone.mac",
		EmbeddedPath: "testdata/legacy_macros/web/sunstone.mac",
	},
	{
		Filename:     "dances.mac",
		Description:  "Dance commands and help.",
		SourceURL:    "https://clump.clanlord.net/library/index.php?title=Dances.mac",
		EmbeddedPath: "testdata/legacy_macros/web/dances.mac",
	},
	{
		Filename:     "directions.mac",
		Description:  "Text commands for common destinations.",
		SourceURL:    "https://clump.clanlord.net/library/index.php?title=Directions.mac",
		EmbeddedPath: "testdata/legacy_macros/web/directions.mac",
	},
	{
		Filename:     "clump-scanner.mac",
		Description:  "Advanced scanner macros.",
		SourceURL:    "https://clump.clanlord.net/library/index.php?title=Scanner.txt",
		EmbeddedPath: "testdata/legacy_macros/web/clump-scanner.mac",
	},
	{
		Filename:     "clump-omega-zu.mac",
		Description:  "Advanced macro collection for Zu.",
		SourceURL:    "https://clump.clanlord.net/library/index.php?title=Omega_Zu",
		EmbeddedPath: "testdata/legacy_macros/web/clump-omega-zu.mac",
		Note:         "The published source has one recoverable closing-brace warning.",
	},
	{
		Filename:     "gorvin-dynamicsharecads.mac",
		Description:  "Maintains a useful set of sharecads.",
		SourceURL:    "http://gorvin.50webs.com/macros/dynamicsharecads.txt",
		EmbeddedPath: "testdata/legacy_macros/web/gorvin-dynamicsharecads.mac",
	},
	{
		Filename:     "gorvin-right-clicker.mac",
		Description:  "Calls macros from right-click and wheel actions.",
		SourceURL:    "http://gorvin.50webs.com/macros/RC2.txt",
		EmbeddedPath: "testdata/legacy_macros/web/gorvin-right-clicker.mac",
	},
	{
		Filename:     "gorvin-macro-chess.mac",
		Description:  "Play chess with another player in Clan Lord.",
		SourceURL:    "http://gorvin.50webs.com/macros/chessMac.txt",
		EmbeddedPath: "testdata/legacy_macros/web/gorvin-macro-chess.mac",
	},
	{
		Filename:     "gorvin-macro-tetris.mac",
		Description:  "Play Tetris in the sidebar.",
		SourceURL:    "http://gorvin.50webs.com/macros/tetrisMac.txt",
		EmbeddedPath: "testdata/legacy_macros/web/gorvin-macro-tetris.mac",
	},
}

// legacyMacroLibraryEntry is a source file available to the library UI. ID is
// always its filename, which stays stable if its display name is edited.
type legacyMacroLibraryEntry struct {
	ID          string
	Name        string
	Path        string
	Description string
	SourceURL   string
	Note        string
	Bundled     bool
}

type legacyMacroLibraryScope uint8

const (
	legacyMacroLibraryGlobal legacyMacroLibraryScope = iota + 1
	legacyMacroLibraryPlayer
)

type legacyMacroLibrarySaveResult struct {
	Entry         legacyMacroLibraryEntry
	SourcePath    string
	SelectionPath string
	Enabled       bool
	Changed       bool
}

// legacyMacroLibrarySelection is intentionally separate from settings.json
// and user-authored macro roots. It says which files in Macros/Library are
// loaded by goThoom for every character or one named character.
type legacyMacroLibrarySelection struct {
	Global  []string            `json:"global,omitempty"`
	Players map[string][]string `json:"players,omitempty"`
}

var legacyMacroLibraryMu sync.Mutex

func legacyMacroLibraryPath() string {
	return filepath.Join(legacyMacrosDir(), legacyMacroLibraryDirName)
}

func legacyMacroLibrarySelectionPath() string {
	return filepath.Join(legacyMacroLibraryPath(), legacyMacroLibrarySelectionName)
}

func legacyMacroLibrarySource(entry legacyMacroLibraryBundledEntry) (string, error) {
	text, err := legacyMacroLibraryFS.ReadFile(entry.EmbeddedPath)
	if err != nil {
		return "", fmt.Errorf("read bundled macro %q: %w", entry.Filename, err)
	}
	return string(text), nil
}

// legacyMacroLibraryEntries returns every .mac source in Macros/Library. The
// bundled corpus is first copied there if missing, never overwriting a user
// file, so the directory is the one editable source of truth.
func legacyMacroLibraryEntries() ([]legacyMacroLibraryEntry, error) {
	if isWASM {
		return legacyMacroLibraryEmbeddedEntries()
	}
	legacyMacroLibraryMu.Lock()
	defer legacyMacroLibraryMu.Unlock()
	return legacyMacroLibraryEntriesLocked()
}

func legacyMacroLibraryEntriesLocked() ([]legacyMacroLibraryEntry, error) {
	if err := installLegacyMacroLibrarySourcesLocked(); err != nil {
		return nil, err
	}

	files, err := os.ReadDir(legacyMacroLibraryPath())
	if err != nil {
		return nil, fmt.Errorf("read legacy macro library: %w", err)
	}
	entries := make([]legacyMacroLibraryEntry, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		id, err := legacyMacroLibraryFileID(file.Name())
		if err != nil {
			continue
		}
		path := filepath.Join(legacyMacroLibraryPath(), id)
		text, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read library macro %q: %w", id, err)
		}
		entry := legacyMacroLibraryEntry{
			ID:   id,
			Name: legacyMacroLibraryDisplayName(id, string(text)),
			Path: path,
		}
		if bundled, ok := legacyMacroLibraryBundledEntryByFilename(id); ok && strings.Contains(string(text), "// goThoom bundled macro:") {
			entry.Bundled = true
			entry.Description = bundled.Description
			entry.SourceURL = bundled.SourceURL
			entry.Note = bundled.Note
		} else {
			entry.Description = "User macro source."
		}
		entries = append(entries, entry)
	}
	legacyMacroLibrarySortEntries(entries)
	return entries, nil
}

func legacyMacroLibraryEmbeddedEntries() ([]legacyMacroLibraryEntry, error) {
	entries := make([]legacyMacroLibraryEntry, 0, len(legacyMacroBundledLibrary))
	for _, bundled := range legacyMacroBundledLibrary {
		text, err := legacyMacroLibrarySource(bundled)
		if err != nil {
			return nil, err
		}
		entries = append(entries, legacyMacroLibraryEntry{
			ID:          bundled.Filename,
			Name:        legacyMacroLibraryDisplayName(bundled.Filename, text),
			Path:        filepath.Join(legacyMacroLibraryPath(), bundled.Filename),
			Description: bundled.Description,
			SourceURL:   bundled.SourceURL,
			Note:        bundled.Note,
			Bundled:     true,
		})
	}
	legacyMacroLibrarySortEntries(entries)
	return entries, nil
}

func legacyMacroLibrarySortEntries(entries []legacyMacroLibraryEntry) {
	sort.Slice(entries, func(i, j int) bool {
		left, right := strings.ToLower(entries[i].Name), strings.ToLower(entries[j].Name)
		if left == right {
			return strings.ToLower(entries[i].ID) < strings.ToLower(entries[j].ID)
		}
		return left < right
	})
}

func legacyMacroLibraryBundledEntryByFilename(filename string) (legacyMacroLibraryBundledEntry, bool) {
	for _, entry := range legacyMacroBundledLibrary {
		if strings.EqualFold(entry.Filename, filename) {
			return entry, true
		}
	}
	return legacyMacroLibraryBundledEntry{}, false
}

func legacyMacroLibraryEntryByIDLocked(id string) (legacyMacroLibraryEntry, bool, error) {
	entries, err := legacyMacroLibraryEntriesLocked()
	if err != nil {
		return legacyMacroLibraryEntry{}, false, err
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.ID, id) {
			return entry, true, nil
		}
	}
	return legacyMacroLibraryEntry{}, false, nil
}

// legacyMacroLibraryDisplayName reads the inert source convention used by the
// UI. A plain filename is intentionally retained as the fallback so a user's
// existing .mac file needs no migration.
func legacyMacroLibraryDisplayName(filename, source string) string {
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if !strings.HasPrefix(line, "//") {
			continue
		}
		comment := strings.TrimSpace(strings.TrimPrefix(line, "//"))
		const namePrefix = "Name:"
		if len(comment) < len(namePrefix) || !strings.EqualFold(comment[:len(namePrefix)], namePrefix) {
			continue
		}
		if name := strings.TrimSpace(comment[len(namePrefix):]); name != "" {
			return name
		}
	}
	return filepath.Base(filename)
}

func installLegacyMacroLibrarySourcesLocked() error {
	if isWASM {
		return fmt.Errorf("saving bundled legacy macros is unavailable in the web build")
	}
	if err := os.MkdirAll(legacyMacroLibraryPath(), 0o755); err != nil {
		return fmt.Errorf("create bundled macro directory: %w", err)
	}
	for _, entry := range legacyMacroBundledLibrary {
		if _, err := saveLegacyMacroLibrarySource(entry); err != nil {
			return err
		}
	}
	return nil
}

func saveLegacyMacroLibrarySource(entry legacyMacroLibraryBundledEntry) (string, error) {
	source, err := legacyMacroLibrarySource(entry)
	if err != nil {
		return "", err
	}
	id, err := legacyMacroLibraryFileID(entry.Filename)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(legacyMacroLibraryPath(), 0o755); err != nil {
		return "", fmt.Errorf("create bundled macro directory: %w", err)
	}
	path := filepath.Join(legacyMacroLibraryPath(), id)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if os.IsExist(err) {
		info, statErr := os.Stat(path)
		if statErr != nil {
			return "", fmt.Errorf("inspect bundled macro source %q: %w", path, statErr)
		}
		if info.IsDir() {
			return "", fmt.Errorf("bundled macro source %q is a directory", path)
		}
		return path, nil
	}
	if err != nil {
		return "", fmt.Errorf("create bundled macro source %q: %w", path, err)
	}
	header := fmt.Sprintf("// goThoom bundled macro: %s\n// Source: %s\n// Original authors retain copyright.\n\n", entry.Filename, entry.SourceURL)
	if _, err := file.WriteString(header + source); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write bundled macro source %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close bundled macro source %q: %w", path, err)
	}
	return path, nil
}

// saveLegacyMacroLibraryEntry is retained as the one-shot enable helper for
// callers that do not need to choose a state explicitly.
func saveLegacyMacroLibraryEntry(id string, scope legacyMacroLibraryScope, character string) (legacyMacroLibrarySaveResult, error) {
	return setLegacyMacroLibraryEntryEnabled(id, scope, character, true)
}

// setLegacyMacroLibraryEntryEnabled records a selection without modifying
// Default or a player's own macro file. ID always refers to a source in
// Macros/Library.
func setLegacyMacroLibraryEntryEnabled(id string, scope legacyMacroLibraryScope, character string, enabled bool) (legacyMacroLibrarySaveResult, error) {
	if isWASM {
		return legacyMacroLibrarySaveResult{}, fmt.Errorf("saving bundled legacy macros is unavailable in the web build")
	}
	id, err := legacyMacroLibraryFileID(id)
	if err != nil {
		return legacyMacroLibrarySaveResult{}, err
	}
	character, err = legacyMacroLibraryScopeCharacter(scope, character)
	if err != nil {
		return legacyMacroLibrarySaveResult{}, err
	}

	legacyMacroLibraryMu.Lock()
	defer legacyMacroLibraryMu.Unlock()

	entry, ok, err := legacyMacroLibraryEntryByIDLocked(id)
	if err != nil {
		return legacyMacroLibrarySaveResult{}, err
	}
	if !ok {
		return legacyMacroLibrarySaveResult{}, fmt.Errorf("unknown library macro %q", id)
	}
	selection, err := legacyMacroLibraryReadSelectionLocked()
	if err != nil {
		return legacyMacroLibrarySaveResult{}, err
	}
	changed, err := selection.set(scope, character, id, enabled)
	if err != nil {
		return legacyMacroLibrarySaveResult{}, err
	}
	result := legacyMacroLibrarySaveResult{
		Entry:         entry,
		SourcePath:    entry.Path,
		SelectionPath: legacyMacroLibrarySelectionPath(),
		Enabled:       enabled,
		Changed:       changed,
	}
	if !changed {
		return result, nil
	}
	if err := legacyMacroLibraryWriteSelectionLocked(selection); err != nil {
		return legacyMacroLibrarySaveResult{}, err
	}
	return result, nil
}

func legacyMacroLibraryEnabledIDs(scope legacyMacroLibraryScope, character string) (map[string]bool, error) {
	character, err := legacyMacroLibraryScopeCharacter(scope, character)
	if err != nil {
		return nil, err
	}
	legacyMacroLibraryMu.Lock()
	defer legacyMacroLibraryMu.Unlock()
	selection, err := legacyMacroLibraryReadSelectionLocked()
	if err != nil {
		return nil, err
	}
	ids, err := selection.ids(scope, character)
	if err != nil {
		return nil, err
	}
	enabled := make(map[string]bool, len(ids))
	for _, id := range ids {
		enabled[legacyMacroLibraryIDKey(id)] = true
	}
	return enabled, nil
}

func (selection *legacyMacroLibrarySelection) ids(scope legacyMacroLibraryScope, character string) ([]string, error) {
	switch scope {
	case legacyMacroLibraryGlobal:
		return append([]string(nil), selection.Global...), nil
	case legacyMacroLibraryPlayer:
		return append([]string(nil), selection.Players[character]...), nil
	default:
		return nil, fmt.Errorf("unknown legacy macro selection scope")
	}
}

func (selection *legacyMacroLibrarySelection) set(scope legacyMacroLibraryScope, character, id string, enabled bool) (bool, error) {
	switch scope {
	case legacyMacroLibraryGlobal:
		updated, changed := legacyMacroLibraryToggleID(selection.Global, id, enabled)
		selection.Global = updated
		return changed, nil
	case legacyMacroLibraryPlayer:
		updated, changed := legacyMacroLibraryToggleID(selection.Players[character], id, enabled)
		if !changed {
			return false, nil
		}
		if len(updated) == 0 {
			delete(selection.Players, character)
		} else {
			if selection.Players == nil {
				selection.Players = make(map[string][]string)
			}
			selection.Players[character] = updated
		}
		return true, nil
	default:
		return false, fmt.Errorf("unknown legacy macro selection scope")
	}
}

func legacyMacroLibraryToggleID(ids []string, id string, enabled bool) ([]string, bool) {
	found := false
	for _, current := range ids {
		if strings.EqualFold(current, id) {
			found = true
			break
		}
	}
	if enabled {
		if found {
			return ids, false
		}
		return legacyMacroLibraryNormalizeIDs(append(ids, id)), true
	}
	if !found {
		return ids, false
	}
	updated := ids[:0]
	for _, current := range ids {
		if !strings.EqualFold(current, id) {
			updated = append(updated, current)
		}
	}
	return updated, true
}

func legacyMacroLibraryReadSelectionLocked() (legacyMacroLibrarySelection, error) {
	data, err := os.ReadFile(legacyMacroLibrarySelectionPath())
	if os.IsNotExist(err) {
		return legacyMacroLibrarySelection{}, nil
	}
	if err != nil {
		return legacyMacroLibrarySelection{}, fmt.Errorf("read macro library selections: %w", err)
	}
	selection := legacyMacroLibrarySelection{}
	if err := json.Unmarshal(data, &selection); err != nil {
		return legacyMacroLibrarySelection{}, fmt.Errorf("parse macro library selections: %w", err)
	}
	selection.normalize()
	return selection, nil
}

func legacyMacroLibraryWriteSelectionLocked(selection legacyMacroLibrarySelection) error {
	selection.normalize()
	data, err := json.MarshalIndent(selection, "", "  ")
	if err != nil {
		return fmt.Errorf("encode macro library selections: %w", err)
	}
	if err := os.MkdirAll(legacyMacroLibraryPath(), 0o755); err != nil {
		return fmt.Errorf("create macro library directory: %w", err)
	}
	if err := os.WriteFile(legacyMacroLibrarySelectionPath(), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write macro library selections: %w", err)
	}
	return nil
}

func (selection *legacyMacroLibrarySelection) normalize() {
	selection.Global = legacyMacroLibraryNormalizeIDs(selection.Global)
	players := make(map[string][]string)
	for character, ids := range selection.Players {
		character, err := legacyMacroLibraryCharacter(character)
		if err != nil {
			continue
		}
		ids = legacyMacroLibraryNormalizeIDs(ids)
		if len(ids) == 0 {
			continue
		}
		players[character] = legacyMacroLibraryNormalizeIDs(append(players[character], ids...))
	}
	if len(players) == 0 {
		selection.Players = nil
	} else {
		selection.Players = players
	}
}

func legacyMacroLibraryNormalizeIDs(ids []string) []string {
	normalized := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		id, err := legacyMacroLibraryFileID(id)
		if err != nil {
			continue
		}
		key := legacyMacroLibraryIDKey(id)
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, id)
	}
	sort.Slice(normalized, func(i, j int) bool {
		return legacyMacroLibraryIDKey(normalized[i]) < legacyMacroLibraryIDKey(normalized[j])
	})
	return normalized
}

func legacyMacroLibraryScopeCharacter(scope legacyMacroLibraryScope, character string) (string, error) {
	switch scope {
	case legacyMacroLibraryGlobal:
		return "", nil
	case legacyMacroLibraryPlayer:
		return legacyMacroLibraryCharacter(character)
	default:
		return "", fmt.Errorf("unknown legacy macro selection scope")
	}
}

func legacyMacroLibraryCharacter(character string) (string, error) {
	character = strings.TrimSpace(character)
	if character == "" || character == "." || character == ".." || filepath.Base(character) != character || strings.ContainsAny(character, "/\\") {
		return "", fmt.Errorf("invalid legacy macro character name %q", character)
	}
	return character, nil
}

func legacyMacroLibraryFileID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || filepath.Base(id) != id || strings.ContainsAny(id, "/\\") || !strings.EqualFold(filepath.Ext(id), ".mac") {
		return "", fmt.Errorf("invalid library macro filename %q", id)
	}
	return id, nil
}

func legacyMacroLibraryIDKey(id string) string {
	return strings.ToLower(id)
}

// legacyMacroLibrarySelectedSources adds configured global and character
// sources after the conventional character root. Files that disappear are
// reported as diagnostics without preventing the remaining macros from
// loading.
func legacyMacroLibrarySelectedSources(character string) ([]legacyMacroSource, []legacyMacroDiagnostic) {
	if isWASM {
		return nil, nil
	}
	character = strings.TrimSpace(character)
	if character != "" {
		if _, err := legacyMacroLibraryCharacter(character); err != nil {
			return nil, []legacyMacroDiagnostic{legacyMacroLibrarySelectionDiagnostic(err.Error())}
		}
	}

	legacyMacroLibraryMu.Lock()
	selection, err := legacyMacroLibraryReadSelectionLocked()
	legacyMacroLibraryMu.Unlock()
	if err != nil {
		return nil, []legacyMacroDiagnostic{legacyMacroLibrarySelectionDiagnostic(err.Error())}
	}

	ids := append([]string(nil), selection.Global...)
	if character != "" {
		ids = append(ids, selection.Players[character]...)
	}
	seen := make(map[string]bool, len(ids))
	sources := make([]legacyMacroSource, 0, len(ids))
	var diagnostics []legacyMacroDiagnostic
	for _, id := range ids {
		id, err := legacyMacroLibraryFileID(id)
		if err != nil {
			diagnostics = append(diagnostics, legacyMacroLibrarySelectionDiagnostic(err.Error()))
			continue
		}
		key := legacyMacroLibraryIDKey(id)
		if seen[key] {
			continue
		}
		seen[key] = true
		path := filepath.Join(legacyMacroLibraryPath(), id)
		source, exists, err := readLegacyMacroSource(path)
		if err != nil {
			diagnostics = append(diagnostics, legacyMacroLibrarySelectionDiagnostic(fmt.Sprintf("read selected library macro %q: %v", id, err)))
			continue
		}
		if !exists {
			diagnostics = append(diagnostics, legacyMacroLibrarySelectionDiagnostic(fmt.Sprintf("selected library macro %q does not exist", id)))
			continue
		}
		source.Name = filepath.ToSlash(filepath.Join(legacyMacroLibraryDirName, id))
		sources = append(sources, source)
	}
	return sources, diagnostics
}

func legacyMacroLibrarySelectionDiagnostic(message string) legacyMacroDiagnostic {
	return legacyMacroDiagnostic{
		Location: legacyMacroLocation{
			Path:   legacyMacroLibrarySelectionPath(),
			Line:   1,
			Column: 1,
		},
		Message: message,
	}
}
