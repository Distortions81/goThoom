package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"gothoom/eui"
)

var newLegacyMacroWin *eui.WindowData

func openNewLegacyMacroWindow() *eui.WindowData {
	if isWASM {
		return nil
	}
	if newLegacyMacroWin != nil {
		newLegacyMacroWin.MarkOpen()
		newLegacyMacroWin.BringForward()
		return newLegacyMacroWin
	}
	win := eui.NewWindow()
	newLegacyMacroWin = win
	win.Title = "New Macro"
	win.Closable, win.Movable, win.AutoSize = true, true, true
	win.Resizable = false
	win.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)
	win.OnClose = func() { newLegacyMacroWin = nil }
	root := eui.NewColumn()
	name, events := eui.NewInput()
	name.Label = "Filename"
	name.Text = "my_macro.mac"
	name.Size = eui.Point{X: 380, Y: 28}
	root.AddItem(name)
	hint := eui.NewLabel("Create in the macro library, then enable for Global or Player.")
	hint.FontSize = 11
	hint.Size = eui.Point{X: 380, Y: 40}
	root.AddItem(hint)
	status := eui.NewLabel("")
	status.Size = eui.Point{X: 380, Y: 40}
	root.AddItem(status)
	setStatus := func(message string) { status.UpdateText(message); win.Refresh() }
	create := eui.NewActionButton("Create", func() {
		entry, err := createLegacyMacro(name.Text)
		if err != nil {
			setStatus(err.Error())
			return
		}
		refreshLegacyMacroLibraryWindow()
		win.Close()
		openLegacyMacroEditor(entry)
	})
	create.Size = eui.Point{X: 96, Y: 24}
	cancel := eui.NewActionButton("Cancel", func() { win.Close() })
	cancel.Size = eui.Point{X: 96, Y: 24}
	root.AddItem(eui.NewRow(create, cancel))
	events.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventInputChanged {
			_, err := newLegacyMacroFilename(name.Text)
			create.Disabled = err != nil
			message := ""
			if err != nil {
				message = err.Error()
			}
			setStatus(message)
		}
	}
	win.AddItem(root)
	win.DefaultButton = create
	win.AddWindow(false)
	eui.Focus(name)
	name.CursorPos, name.SelectEnd = len([]rune(name.Text)), len([]rune(name.Text))
	return win
}

func newLegacyMacroFilename(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return "", fmt.Errorf("Enter a filename without a leading or trailing dot.")
	}
	if len(name) > 180 {
		return "", fmt.Errorf("Use a shorter filename (up to 180 bytes).")
	}
	for _, r := range name {
		if unicode.IsControl(r) || strings.ContainsRune(`/\:*?"<>|`, r) {
			return "", fmt.Errorf("Use a filename without slashes or special characters.")
		}
	}
	base := strings.ToUpper(strings.TrimSpace(strings.SplitN(name, ".", 2)[0]))
	baseRunes := []rune(base)
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" ||
		(len(baseRunes) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && strings.ContainsRune("123456789¹²³", baseRunes[3])) {
		return "", fmt.Errorf("That filename is reserved; choose another name.")
	}
	if filepath.Ext(name) == "" {
		name += ".mac"
	}
	if !legacyMacroLibraryExtension(filepath.Ext(name)) {
		return "", fmt.Errorf("Use a .mac or .txt filename.")
	}
	return name, nil
}

// Create only the source file; enabling and reloading remain explicit actions.
func createLegacyMacro(name string) (legacyMacroLibraryEntry, error) {
	if isWASM {
		return legacyMacroLibraryEntry{}, fmt.Errorf("Embedded library is read-only.")
	}
	id, err := newLegacyMacroFilename(name)
	if err != nil {
		return legacyMacroLibraryEntry{}, err
	}
	legacyMacroLibraryMu.Lock()
	defer legacyMacroLibraryMu.Unlock()
	if _, bundled := legacyMacroLibraryBundledEntryByFilename(id); bundled {
		return legacyMacroLibraryEntry{}, fmt.Errorf("That filename belongs to an included macro; choose another name.")
	}
	if err := os.MkdirAll(legacyMacroLibraryPath(), 0o755); err != nil {
		return legacyMacroLibraryEntry{}, err
	}
	files, err := os.ReadDir(legacyMacroLibraryPath())
	if err != nil {
		return legacyMacroLibraryEntry{}, err
	}
	for _, file := range files {
		if strings.EqualFold(file.Name(), id) {
			return legacyMacroLibraryEntry{}, fmt.Errorf("That filename already exists; choose another name.")
		}
	}
	path := filepath.Join(legacyMacroLibraryPath(), id)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return legacyMacroLibraryEntry{}, err
	}
	// Start with an empty UTF-8 document, ready for pasted or new macro source.
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return legacyMacroLibraryEntry{}, err
	}
	return legacyMacroLibraryEntry{ID: id, Name: strings.TrimSuffix(id, filepath.Ext(id)), Path: path}, nil
}
