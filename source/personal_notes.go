package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type personalNote struct {
	Subject string   `json:"subject"`
	Tags    []string `json:"tags,omitempty"`
	Player  string   `json:"player,omitempty"`
	id      string
	details *sourceDocument
}

func personalNotesDir() string { return filepath.Join(dataDirPath, "Notes") }

func personalNotePath(id, filename string) (string, error) {
	if !strings.HasPrefix(id, "note-") || id == "note-" || filepath.Base(id) != id || strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid note ID")
	}
	return filepath.Abs(filepath.Join(personalNotesDir(), id, filename))
}

func normalizeNoteTags(tags []string) []string {
	var result []string
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tag), "#"))
		key := strings.ToLower(tag)
		if tag != "" && !seen[key] {
			result = append(result, tag)
			seen[key] = true
		}
	}
	return result
}

func loadPersonalNote(id string) (*personalNote, error) {
	filename, err := personalNotePath(id, "note.json")
	if err != nil {
		return nil, err
	}
	doc, err := loadSourceDocument(filename, false)
	if err != nil {
		return nil, err
	}
	note := &personalNote{id: id, details: doc}
	if err := json.Unmarshal([]byte(doc.savedText), note); err != nil {
		return nil, err
	}
	note.Subject, note.Player = strings.TrimSpace(note.Subject), strings.TrimSpace(note.Player)
	note.Tags = normalizeNoteTags(note.Tags)
	if note.Subject == "" {
		return nil, fmt.Errorf("note has no subject")
	}
	return note, nil
}

func listPersonalNotes() ([]*personalNote, error) {
	entries, err := os.ReadDir(personalNotesDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var notes []*personalNote
	var problems []error
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "note-") {
			continue
		}
		note, err := loadPersonalNote(entry.Name())
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		notes = append(notes, note)
	}
	sort.Slice(notes, func(i, j int) bool {
		a, b := strings.ToLower(notes[i].Subject), strings.ToLower(notes[j].Subject)
		if a != b {
			return a < b
		}
		return notes[i].id < notes[j].id
	})
	return notes, errors.Join(problems...)
}

func createPersonalNote(subject, tags, player string) (*personalNote, error) {
	if isWASM {
		return nil, fmt.Errorf("personal notes are available in the desktop client")
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return nil, fmt.Errorf("enter a subject")
	}
	if err := os.MkdirAll(personalNotesDir(), 0755); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(personalNotesDir(), "note-")
	if err != nil {
		return nil, err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(dir)
		}
	}()
	note := &personalNote{Subject: subject, Tags: normalizeNoteTags(strings.Split(tags, ",")), Player: strings.TrimSpace(player)}
	data, err := json.MarshalIndent(note, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := createEditorFile(filepath.Join(dir, "note.json"), append(data, '\n')); err != nil {
		return nil, err
	}
	if err := createEditorFile(filepath.Join(dir, "text.txt"), nil); err != nil {
		return nil, err
	}
	note, err = loadPersonalNote(filepath.Base(dir))
	if err != nil {
		return nil, err
	}
	complete = true
	return note, nil
}

func savePersonalNoteDetails(note *personalNote, subject, tags, player string) error {
	next := *note
	next.Subject, next.Player = strings.TrimSpace(subject), strings.TrimSpace(player)
	next.Tags = normalizeNoteTags(strings.Split(tags, ","))
	if next.Subject == "" {
		return fmt.Errorf("enter a subject")
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err := note.details.save(string(data) + "\n"); err != nil {
		return err
	}
	*note = next
	if filename, err := personalNotePath(note.id, "text.txt"); err == nil {
		if ed := sourceEditors[filename]; ed != nil {
			ed.options.displayName = note.Subject
			ed.refreshStatus()
		}
	}
	return nil
}

func personalNoteMatches(note *personalNote, query, player, scope string) bool {
	forPlayer := player != "" && strings.EqualFold(strings.TrimSpace(player), note.Player)
	switch scope {
	case "Global only":
		if note.Player != "" {
			return false
		}
	case "Player only":
		if note.Player == "" || !forPlayer {
			return false
		}
	case "All notes":
	default:
		if note.Player != "" && !forPlayer {
			return false
		}
	}
	text := strings.ToLower(note.Subject + " " + strings.Join(note.Tags, " "))
	for _, term := range strings.Fields(strings.ToLower(query)) {
		if !strings.Contains(text, strings.TrimPrefix(term, "#")) {
			return false
		}
	}
	return true
}

func openPersonalNote(note *personalNote) *sourceEditor {
	filename, err := personalNotePath(note.id, "text.txt")
	if err != nil {
		consoleMessage("[notes] " + err.Error())
		return nil
	}
	return openTextFileEditor(filename, sourceEditorOptions{kind: "Note", displayName: note.Subject})
}

func trashPersonalNote(note *personalNote) error {
	if isWASM {
		return fmt.Errorf("personal notes are available in the desktop client")
	}
	filename, err := personalNotePath(note.id, "text.txt")
	if err != nil {
		return err
	}
	ed := sourceEditors[filename]
	if ed != nil && ed.dirty() {
		return fmt.Errorf("save or discard the open draft before moving this note to Trash")
	}
	trash := filepath.Join(personalNotesDir(), "Trash")
	if err := os.MkdirAll(trash, 0755); err != nil {
		return err
	}
	destination := filepath.Join(trash, note.id)
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		if err != nil {
			return err
		}
		return fmt.Errorf("a note with this ID is already in Trash")
	}
	if err := os.Rename(filepath.Dir(filename), destination); err != nil {
		return err
	}
	if ed != nil {
		ed.win.Close()
	}
	return nil
}
