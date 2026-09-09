package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type scriptPermission struct {
	ID, Label, Help string
}

var scriptPermissionCatalog = []scriptPermission{
	{"commands", "Register commands and shortcuts", "Add slash commands and text shortcuts to the client."},
	{"hotkeys", "Bind keys and mouse buttons", "React to bound keys or mouse buttons, inspect their input event and consume that input."},
	{"send", "Send server commands", "Send text and commands to the server as your character, including equipment commands."},
	{"data", "Game data and events", "Read players, inventory, selections, vitals, scenery and world updates."},
	{"messages", "Chat and server messages", "Read chat and server messages, including private messages."},
	{"movement", "Automatic movement", "Steer your character using the movement mouse."},
	{"windows", "Windows, toolbars and overlays", "Create client windows, toolbar buttons and world overlays."},
	{"input", "Input box and pointer", "Read or replace unsent input text and inspect the pointer."},
	{"notifications", "Notifications and sound", "Show notifications and play sounds."},
	{"storage", "Persistent script storage", "Read and write this script's private saved data, shared across characters."},
	{"timers", "Background timers", "Run timers and cancellable background tasks."},
	{"session", "Session events", "React to login, logout, character changes and script stop events."},
}

// Every exported function must be classified. Empty means basic output,
// configuration or cleanup contract. Unknown functions fail closed.
var scriptFunctionPermissions = map[string]string{
	"CharacterStore": "storage", "HasPermission": "", "StartTask": "timers", "After": "timers", "QueueCommand": "send", "OnPlayerChange": "data",
	"Command": "commands", "Bind": "hotkeys", "Send": "send", "Print": "", "AddShortcut": "commands",
	"Equip": "send", "Unequip": "send", "Wait": "", "WaitTicks": "", "OnStop": "session", "StopMoving": "", "OverlayClear": "",
	"Bool": "", "Integer": "", "Decimal": "", "Text": "", "Choice": "", "KeyBinding": "",
	"Self": "data", "Players": "data", "Inventory": "data", "EquippedItems": "data",
	"FindItemExact": "data", "FindItem": "data", "FindItems": "data", "SearchItems": "data",
	"Equipped": "data", "HasItem": "data", "IsEquipped": "data", "WithEquipment": "data",
	"SelectedPlayer": "data", "SelectedItem": "data", "CurrentWorld": "data", "OnWorld": "data", "OnChange": "data",
	"WorldSize": "data", "ImageSize": "data", "WaitForInventory": "data", "WaitForEquipment": "data", "ItemSelector": "data",
	"OnChat": "messages", "OnServerMessage": "messages", "LatestServerMessage": "messages",
	"Move": "movement", "Movement": "movement",
	"CreateWindow": "windows", "AddToolbar": "windows", "OverlayRect": "windows", "OverlayText": "windows", "OverlayImage": "windows",
	"SetMobileTint": "windows", "ClearMobileTint": "windows", "ClearMobileTints": "windows",
	"SetMobileOutline": "windows", "ClearMobileOutline": "windows", "ClearMobileOutlines": "windows",
	"SetNamedMobileTint": "windows", "ClearNamedMobileTint": "windows",
	"SetNamedMobileOutline": "windows", "ClearNamedMobileOutline": "windows",
	"FlashMobile": "windows",
	"InputText":   "input", "SetInputText": "input", "LastClick": "input", "Hover": "input",
	"ShowNotification": "notifications", "PlaySound": "notifications",
	"Store": "storage", "LoadString": "storage", "LoadBool": "storage", "LoadInteger": "storage", "LoadDecimal": "storage",
	"LoadStrings": "storage", "LoadJSON": "storage", "DeleteStored": "storage", "MigrateStorage": "storage",
	"Repeat": "timers", "OnLogin": "session", "OnLogout": "session", "OnCharacterChange": "session",
}

var scriptPermissionMu sync.RWMutex
var scriptPermissionGrants = map[string]map[string]bool{}
var scriptPermissionReviews = map[string]map[string]bool{}

func scriptPermissionsReviewed(owner string) bool {
	scriptPermissionMu.RLock()
	defer scriptPermissionMu.RUnlock()
	_, ok := scriptPermissionReviews[owner]
	return ok
}

func scriptHasPermission(owner, permission string) bool {
	scriptPermissionMu.RLock()
	defer scriptPermissionMu.RUnlock()
	return scriptPermissionGrants[owner][permission]
}

func guardScriptExports(owner string, candidate *scriptCandidate, symbols map[string]reflect.Value) {
	for name, fn := range symbols {
		if fn.Kind() != reflect.Func {
			continue
		}
		permission, known := scriptFunctionPermissions[name]
		symbols[name] = reflect.MakeFunc(fn.Type(), func(args []reflect.Value) []reflect.Value {
			if !candidate.apiCallAllowed(owner, permission) || !known || (permission != "" && !scriptHasPermission(owner, permission)) || (name == "WithEquipment" && !scriptHasPermission(owner, "send")) {
				results := make([]reflect.Value, fn.Type().NumOut())
				for index := range results {
					results[index] = reflect.Zero(fn.Type().Out(index))
				}
				return results
			}
			checkScriptTask(candidate.runtimeEventQueue(owner))
			return fn.Call(args)
		})
	}
}

// Discover references, including stored function values, without executing any
// script code. Aliased and dot imports are supported. Conservative discovery
// may include unused helpers; runtime guards remain the enforcement boundary.
func scriptRequiredPermissions(source []byte) (map[string]bool, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "_.go", source, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	aliases := map[string]bool{}
	dot := false
	for _, imp := range file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if path != "gt2" {
			continue
		}
		name := "gt2"
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name == "." {
			dot = true
		} else if name != "_" {
			aliases[name] = true
		}
	}
	required := map[string]bool{}
	add := func(name string) {
		if permission := scriptFunctionPermissions[name]; permission != "" {
			required[permission] = true
			if name == "WithEquipment" {
				required["send"] = true
			}
			if name == "AddToolbar" {
				required["hotkeys"] = true
			}
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.SelectorExpr:
			if id, ok := n.X.(*ast.Ident); ok && aliases[id.Name] {
				add(n.Sel.Name)
			}
		case *ast.Ident:
			if dot {
				add(n.Name)
			}
		}
		return true
	})
	return required, nil
}

type scriptPermissionReviewError struct{ detail string }

func (err *scriptPermissionReviewError) Error() string {
	return "permissions required: " + err.detail + "; open Scripts > Info > Permissions"
}

func checkScriptPermissions(owner string, source []byte) error {
	required, err := scriptRequiredPermissions(source)
	if err != nil {
		return err
	}
	scriptPermissionMu.RLock()
	reviewed, exists := scriptPermissionReviews[owner]
	var additional []string
	for _, permission := range scriptPermissionCatalog {
		if required[permission.ID] && !reviewed[permission.ID] {
			additional = append(additional, permission.Label)
		}
	}
	scriptPermissionMu.RUnlock()
	if !exists || len(additional) > 0 {
		detail := "review this script before it runs"
		if len(additional) > 0 {
			detail = strings.Join(additional, ", ")
		}
		return &scriptPermissionReviewError{detail: detail}
	}
	return nil
}

// This file belongs to the client, outside the scripts' private Store API.
// Grants are per stable script ID and shared across characters. New capabilities
// default to denied even when an existing script is updated.
func scriptPermissionsPath() string {
	return filepath.Join(dataDirPath, scriptEnablementDirName, "permissions.json")
}

type scriptPermissionDecision struct {
	Requested []string `json:"requested"`
	Granted   []string `json:"granted"`
}

func loadScriptPermissions() {
	grants := map[string]map[string]bool{}
	reviews := map[string]map[string]bool{}
	defer func() {
		scriptPermissionMu.Lock()
		scriptPermissionGrants, scriptPermissionReviews = grants, reviews
		scriptPermissionMu.Unlock()
	}()
	if isWASM {
		return
	}
	data, err := os.ReadFile(scriptPermissionsPath())
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		log.Printf("read script permissions: %v", err)
		return
	}
	var saved map[string]scriptPermissionDecision
	if err := json.Unmarshal(data, &saved); err != nil {
		log.Printf("parse script permissions: %v", err)
		return
	}
	for owner, decision := range saved {
		if !validScriptEnablementID(owner) {
			continue
		}
		grants[owner], reviews[owner] = map[string]bool{}, map[string]bool{}
		for _, permission := range scriptPermissionCatalog {
			for _, id := range decision.Requested {
				if id == permission.ID {
					reviews[owner][id] = true
				}
			}
			for _, id := range decision.Granted {
				if id == permission.ID && reviews[owner][id] {
					grants[owner][id] = true
				}
			}
		}
	}
}

func saveScriptPermissions(grants, reviews map[string]map[string]bool) error {
	if isWASM {
		return fmt.Errorf("Go scripts are unavailable in this client")
	}
	saved := map[string]scriptPermissionDecision{}
	for owner, reviewed := range reviews {
		decision := scriptPermissionDecision{Requested: []string{}, Granted: []string{}}
		for _, permission := range scriptPermissionCatalog {
			if reviewed[permission.ID] {
				decision.Requested = append(decision.Requested, permission.ID)
				if grants[owner][permission.ID] {
					decision.Granted = append(decision.Granted, permission.ID)
				}
			}
		}
		sort.Strings(decision.Requested)
		sort.Strings(decision.Granted)
		saved[owner] = decision
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	path := scriptPermissionsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return legacyMacroAtomicWriteFile(path, append(data, '\n'), 0o600)
}

func setScriptPermissions(owner string, selected map[string]bool) error {
	return applyScriptPermissions(owner, selected, false)
}

// Called by the client UI only. Persist before changing live grants. An empty
// reviewed set is still saved: a script with no sensitive hooks also needs review.
func applyScriptPermissions(owner string, selected map[string]bool, blockAll bool) error {
	if !validScriptEnablementID(owner) {
		return fmt.Errorf("invalid script ID")
	}
	scriptMu.RLock()
	info, exists := scriptPackages[owner]
	scriptMu.RUnlock()
	if !exists {
		return fmt.Errorf("script no longer exists")
	}
	required, err := scriptRequiredPermissions(info.src)
	if err != nil {
		return err
	}
	scriptPermissionMu.Lock()
	nextGrants := make(map[string]map[string]bool, len(scriptPermissionGrants)+1)
	nextReviews := make(map[string]map[string]bool, len(scriptPermissionReviews)+1)
	for id, grants := range scriptPermissionGrants {
		nextGrants[id] = grants
	}
	for id, review := range scriptPermissionReviews {
		nextReviews[id] = review
	}
	nextGrants[owner], nextReviews[owner] = map[string]bool{}, required
	for id := range required {
		if selected[id] && !blockAll {
			nextGrants[owner][id] = true
		}
	}
	if err := saveScriptPermissions(nextGrants, nextReviews); err != nil {
		scriptPermissionMu.Unlock()
		return err
	}
	scriptPermissionGrants, scriptPermissionReviews = nextGrants, nextReviews
	scriptPermissionMu.Unlock()
	if blockAll {
		scriptMu.Lock()
		delete(scriptEnabledFor, owner)
		scriptMu.Unlock()
	}
	// Restart releases subscriptions, windows, movement and queued actions.
	if scriptIsRunning(owner) {
		disablescript(owner, "permissions changed")
	}
	scriptMu.RLock()
	enabled := scriptEnabledFor[owner].enablesFor(scriptExecutionCharacter())
	scriptMu.RUnlock()
	if enabled {
		enablescript(owner)
	} else {
		scriptMu.Lock()
		if strings.Contains(scriptErrors[owner], "permissions required:") {
			delete(scriptErrors, owner)
			delete(scriptReloadFailed, owner)
		}
		scriptMu.Unlock()
	}
	saveScriptEnablement()
	refreshscriptsWindow()
	refreshscriptDetails()
	return nil
}
