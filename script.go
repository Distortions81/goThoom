package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

const scriptAPICurrentVersion = 1

type scriptScope struct {
	All   bool
	Chars map[string]bool
}

func (s scriptScope) enablesFor(effChar string) bool {
	if s.All {
		return true
	}
	if effChar == "" || s.Chars == nil {
		return false
	}
	return s.Chars[effChar]
}

func (s *scriptScope) addChar(name string) {
	if name == "" {
		return
	}
	if s.Chars == nil {
		s.Chars = map[string]bool{}
	}
	s.Chars[name] = true
}

func (s *scriptScope) removeChar(name string) {
	if s.Chars == nil || name == "" {
		return
	}
	delete(s.Chars, name)
}

func (s scriptScope) empty() bool { return !s.All && (s.Chars == nil || len(s.Chars) == 0) }

// Expose the script API under both a short and a module-qualified path so
// Yaegi can resolve imports regardless of how the script refers to it.
var basescriptExports = interp.Exports{
	// Short path used by simple script scripts: import "gt"
	// Yaegi expects keys as "importPath/pkgName".
	"gt/gt": {
		"Console":          reflect.ValueOf(scriptConsole),
		"ShowNotification": reflect.ValueOf(scriptShowNotification),
		"clVersion":        reflect.ValueOf(&clVersion).Elem(),
		"PlayerName":       reflect.ValueOf(scriptPlayerName),
		"Players":          reflect.ValueOf(scriptPlayers),
		"Inventory":        reflect.ValueOf(scriptInventory),
		"InventoryItem":    reflect.ValueOf((*InventoryItem)(nil)),
		"Player":           reflect.ValueOf((*Player)(nil)),
		"PlaySound":        reflect.ValueOf(scriptPlaySound),
		"InputText":        reflect.ValueOf(scriptInputText),
		"SetInputText":     reflect.ValueOf(scriptSetInputText),
		"KeyJustPressed":   reflect.ValueOf(scriptKeyJustPressed),
		"MouseJustPressed": reflect.ValueOf(scriptMouseJustPressed),
		"MouseWheel":       reflect.ValueOf(scriptMouseWheel),
		"LastClick":        reflect.ValueOf(scriptLastClick),
		"ClickInfo":        reflect.ValueOf((*ClickInfo)(nil)),
		"Mobile":           reflect.ValueOf((*Mobile)(nil)),
		"EquippedItems":    reflect.ValueOf(scriptEquippedItems),
		"HasItem":          reflect.ValueOf(scriptHasItem),
		"IsEquipped":       reflect.ValueOf(scriptIsEquipped),
		"IgnoreCase":       reflect.ValueOf(scriptIgnoreCase),
		"StartsWith":       reflect.ValueOf(scriptStartsWith),
		"EndsWith":         reflect.ValueOf(scriptEndsWith),
		"Includes":         reflect.ValueOf(scriptIncludes),
		"Lower":            reflect.ValueOf(scriptLower),
		"Upper":            reflect.ValueOf(scriptUpper),
		"Trim":             reflect.ValueOf(scriptTrim),
		"TrimStart":        reflect.ValueOf(scriptTrimStart),
		"TrimEnd":          reflect.ValueOf(scriptTrimEnd),
		"Words":            reflect.ValueOf(scriptWords),
		"Join":             reflect.ValueOf(scriptJoin),
		"Replace":          reflect.ValueOf(scriptReplace),
		"Split":            reflect.ValueOf(scriptSplit),
		// Hotkey event type for function-based hotkeys
		"HotkeyEvent": reflect.ValueOf((*HotkeyEvent)(nil)),
		// Chat trigger flags
		"ChatAny":      reflect.ValueOf(ChatAny),
		"ChatPlayer":   reflect.ValueOf(ChatPlayer),
		"ChatNPC":      reflect.ValueOf(ChatNPC),
		"ChatCreature": reflect.ValueOf(ChatCreature),
		"ChatSelf":     reflect.ValueOf(ChatSelf),
		"ChatOther":    reflect.ValueOf(ChatOther),
	},
}

func exportsForscript(owner string) interp.Exports {
	ex := make(interp.Exports)
	for pkg, symbols := range basescriptExports {
		m := map[string]reflect.Value{}
		for k, v := range symbols {
			m[k] = v
		}
		// Prefer string-based APIs; keep ID variants for power users
		m["Equip"] = reflect.ValueOf(func(name string) { scriptEquipByName(owner, name) })
		m["Unequip"] = reflect.ValueOf(func(name string) { scriptUnequipByName(owner, name) })
		m["EquipPartial"] = reflect.ValueOf(func(name string) { scriptEquipPartial(owner, name) })
		m["UnequipPartial"] = reflect.ValueOf(func(name string) { scriptUnequipPartial(owner, name) })
		m["EquipById"] = reflect.ValueOf(func(id uint16) { scriptEquip(owner, id) })
		m["UnequipById"] = reflect.ValueOf(func(id uint16) { scriptUnequip(owner, id) })
		m["AddHotkey"] = reflect.ValueOf(func(combo, command string) { scriptAddHotkey(owner, combo, command) })
		m["AddHotkeyFn"] = reflect.ValueOf(func(combo string, handler func(HotkeyEvent)) { scriptAddHotkeyFn(owner, combo, handler) })
		m["RemoveHotkey"] = reflect.ValueOf(func(combo string) { scriptRemoveHotkey(owner, combo) })
		m["RegisterCommand"] = reflect.ValueOf(func(name string, handler scriptCommandHandler) {
			scriptRegisterCommand(owner, name, handler)
		})
		m["AddShortcut"] = reflect.ValueOf(func(short, full string) { scriptAddShortcut(owner, short, full) })
		m["AddShortcuts"] = reflect.ValueOf(func(shortcuts map[string]string) { scriptAddShortcuts(owner, shortcuts) })
		// Chat/Console (simple, no slices)
		// Simple DSL aliases
		m["Print"] = reflect.ValueOf(scriptConsole)
		m["Notify"] = reflect.ValueOf(scriptShowNotification)
		m["Cmd"] = reflect.ValueOf(func(text string) { scriptEnqueueCommand(owner, strings.TrimSpace(text)) })
		m["Run"] = reflect.ValueOf(func(text string) { scriptRunCommand(owner, strings.TrimSpace(text)) })
		m["Me"] = reflect.ValueOf(scriptPlayerName)
		m["Has"] = reflect.ValueOf(func(name string) bool { return scriptHasItem(name) })
		m["Save"] = reflect.ValueOf(func(key, value string) { scriptStorageSet(owner, key, value) })
		m["Load"] = reflect.ValueOf(func(key string) string {
			if v, ok := scriptStorageGet(owner, key).(string); ok {
				return v
			}
			return ""
		})
		m["Delete"] = reflect.ValueOf(func(key string) { scriptStorageDelete(owner, key) })
		m["Input"] = reflect.ValueOf(scriptInputText)
		m["SetInput"] = reflect.ValueOf(scriptSetInputText)
		// (Removed explicit Thank/Curse/Share/Unshare helpers to avoid duplicating
		// in-game commands; authors can use Cmd("/thank ...") etc.)
		// No-slice chat/console helpers (one call per phrase)
		m["Chat"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				scriptRegisterChat(owner, "", []string{p}, ChatAny, handler)
			}
		})
		m["PlayerChat"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				scriptRegisterChat(owner, "", []string{p}, ChatPlayer, handler)
			}
		})
		m["NPCChat"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				scriptRegisterChat(owner, "", []string{p}, ChatNPC, handler)
			}
		})
		m["CreatureChat"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				scriptRegisterChat(owner, "", []string{p}, ChatCreature, handler)
			}
		})
		m["SelfChat"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				scriptRegisterChat(owner, "", []string{p}, ChatSelf, handler)
			}
		})
		m["OtherChat"] = reflect.ValueOf(func(name, phrase string, handler func(string)) {
			n := strings.TrimSpace(name)
			p := strings.TrimSpace(phrase)
			if p != "" {
				scriptRegisterChat(owner, n, []string{p}, ChatOther, handler)
			}
		})
		m["ChatFrom"] = reflect.ValueOf(func(name, phrase string, handler func(string)) {
			n := strings.TrimSpace(name)
			p := strings.TrimSpace(phrase)
			if n != "" && p != "" {
				scriptRegisterChat(owner, n, []string{p}, ChatAny, handler)
			}
		})
		m["PlayerChatFrom"] = reflect.ValueOf(func(name, phrase string, handler func(string)) {
			n := strings.TrimSpace(name)
			p := strings.TrimSpace(phrase)
			if n != "" && p != "" {
				scriptRegisterChat(owner, n, []string{p}, ChatPlayer, handler)
			}
		})
		m["OtherChatFrom"] = reflect.ValueOf(func(name, phrase string, handler func(string)) {
			n := strings.TrimSpace(name)
			p := strings.TrimSpace(phrase)
			if n != "" && p != "" {
				scriptRegisterChat(owner, n, []string{p}, ChatOther, handler)
			}
		})
		m["ConsoleMsg"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				scriptRegisterConsole(owner, []string{p}, handler)
			}
		})
		// Sleep for game ticks (blocks current goroutine only)
		m["SleepTicks"] = reflect.ValueOf(func(ticks int) { scriptSleepTicks(owner, ticks) })
		// Simpler alias: Console("text", fn)
		m["Console"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				scriptRegisterConsole(owner, []string{p}, handler)
			}
		})
		// Back-compat registrations matching gt stubs
		m["RegisterTriggers"] = reflect.ValueOf(func(name string, phrases []string, fn func(string)) {
			if fn == nil || len(phrases) == 0 {
				return
			}
			scriptRegisterChat(owner, name, phrases, ChatAny, fn)
		})
		m["RegisterConsoleTriggers"] = reflect.ValueOf(func(phrases []string, fn func()) {
			if fn == nil || len(phrases) == 0 {
				return
			}
			scriptRegisterConsoleTriggers(owner, phrases, fn)
		})
		m["RegisterTrigger"] = reflect.ValueOf(func(name, phrase string, fn func()) {
			p := strings.TrimSpace(phrase)
			if p == "" || fn == nil {
				return
			}
			scriptRegisterChat(owner, name, []string{p}, ChatAny, func(string) { fn() })
		})
		m["RegisterPlayerHandler"] = reflect.ValueOf(func(fn func(Player)) { scriptRegisterPlayerHandler(owner, fn) })
		m["RegisterInputHandler"] = reflect.ValueOf(func(fn func(string) string) { scriptRegisterInputHandler(owner, fn) })
		m["RegisterChatHandler"] = reflect.ValueOf(func(fn func(string)) { scriptRegisterChatHandler(owner, fn) })
		// Simple world overlay drawing (top-left origin, world units)
		m["OverlayClear"] = reflect.ValueOf(func() { scriptOverlayClear(owner) })
		m["OverlayRect"] = reflect.ValueOf(func(x, y, w, h int, r, g, b, a uint8) {
			scriptOverlayRect(owner, x, y, w, h, r, g, b, a)
		})
		m["OverlayText"] = reflect.ValueOf(func(x, y int, txt string, r, g, b, a uint8) {
			scriptOverlayText(owner, x, y, txt, r, g, b, a)
		})
		m["OverlayImage"] = reflect.ValueOf(func(id uint16, x, y int) {
			scriptOverlayImage(owner, id, x, y)
		})
		m["WorldSize"] = reflect.ValueOf(func() (int, int) { return gameAreaSizeX, gameAreaSizeY })
		m["ImageSize"] = reflect.ValueOf(func(id uint16) (int, int) {
			if clImages == nil {
				return 0, 0
			}
			w, h := clImages.Size(uint32(id))
			return w, h
		})
		m["RunCommand"] = reflect.ValueOf(func(cmd string) { scriptRunCommand(owner, cmd) })
		m["EnqueueCommand"] = reflect.ValueOf(func(cmd string) { scriptEnqueueCommand(owner, cmd) })
		m["StorageGet"] = reflect.ValueOf(func(key string) any { return scriptStorageGet(owner, key) })
		m["StorageSet"] = reflect.ValueOf(func(key string, value any) { scriptStorageSet(owner, key, value) })
		m["StorageDelete"] = reflect.ValueOf(func(key string) { scriptStorageDelete(owner, key) })
		m["AddConfig"] = reflect.ValueOf(func(name, typ string) { scriptAddConfig(owner, name, typ) })

		// Timers
		m["After"] = reflect.ValueOf(func(ms int, fn func()) {
			if fn == nil || ms <= 0 {
				return
			}
			t := time.AfterFunc(time.Duration(ms)*time.Millisecond, fn)
			scriptMu.Lock()
			scriptTimers[owner] = append(scriptTimers[owner], t)
			scriptMu.Unlock()
		})
		m["AfterDur"] = reflect.ValueOf(func(d time.Duration, fn func()) {
			if fn == nil || d <= 0 {
				return
			}
			t := time.AfterFunc(d, fn)
			scriptMu.Lock()
			scriptTimers[owner] = append(scriptTimers[owner], t)
			scriptMu.Unlock()
		})
		m["Every"] = reflect.ValueOf(func(ms int, fn func()) {
			if fn == nil || ms <= 0 {
				return
			}
			stop := make(chan struct{})
			scriptMu.Lock()
			scriptTickerStops[owner] = append(scriptTickerStops[owner], stop)
			scriptMu.Unlock()
			d := time.Duration(ms) * time.Millisecond
			go func() {
				ticker := time.NewTicker(d)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						fn()
					case <-stop:
						return
					}
				}
			}()
		})
		m["EveryDur"] = reflect.ValueOf(func(d time.Duration, fn func()) {
			if fn == nil || d <= 0 {
				return
			}
			stop := make(chan struct{})
			scriptMu.Lock()
			scriptTickerStops[owner] = append(scriptTickerStops[owner], stop)
			scriptMu.Unlock()
			go func() {
				ticker := time.NewTicker(d)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						fn()
					case <-stop:
						return
					}
				}
			}()
		})

		// Key binding to a function: creates a hidden command and binds a hotkey to it
		m["Key"] = reflect.ValueOf(func(combo string, handler func()) {
			c := strings.TrimSpace(combo)
			if c == "" || handler == nil {
				return
			}
			cmd := "__hk_" + strings.ReplaceAll(strings.ToLower(c), " ", "_")
			scriptRegisterCommand(owner, cmd, func(args string) { handler() })
			scriptAddHotkey(owner, c, "/"+cmd)
		})
		ex[pkg] = m
	}
	return ex
}

//go:embed scripts
var scriptScripts embed.FS

// userScriptsDir returns the preferred location for user-editable scripts.
// Scripts now live alongside the executable in a top-level "scripts" folder
// instead of under the data directory.
func userScriptsDir() string {
	if isWASM {
		return ""
	}
	exe, err := os.Executable()
	if err != nil {
		return "scripts"
	}
	return filepath.Join(filepath.Dir(exe), "scripts")
}

// scriptSearchDirs returns only the scripts/ folder next to the executable.
func scriptSearchDirs() []string {
	dir := userScriptsDir()
	if dir == "" {
		return nil
	}
	return []string{dir}
}

// ensureScriptsDir creates the scripts directory next to the executable and
// populates it with the embedded scripts if it is missing.
func ensureScriptsDir() {
	if isWASM {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Join(filepath.Dir(exe), "scripts")
	if _, err := os.Stat(dir); err == nil {
		return
	} else if !os.IsNotExist(err) {
		log.Printf("check scripts dir: %v", err)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("create scripts dir: %v", err)
		return
	}
	entries, err := scriptScripts.ReadDir("scripts")
	if err != nil {
		log.Printf("read embedded scripts: %v", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := scriptScripts.ReadFile(path.Join("scripts", e.Name()))
		if err != nil {
			log.Printf("read embedded %s: %v", e.Name(), err)
			continue
		}
		dst := filepath.Join(dir, e.Name())
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			log.Printf("write %s: %v", dst, err)
		}
	}
}

// ensureDefaultScripts creates the user scripts directory and populates it
// with example scripts when it is empty.
func ensureDefaultScripts() {
	if isWASM {
		return
	}
	dir := userScriptsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("create scripts dir: %v", err)
		return
	}
	// Check if directory already has any .go script files
	hasGo := false
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				hasGo = true
				break
			}
		}
	}
	if hasGo {
		return
	}
	// Write example script files
	files := []string{
		"default_shortcuts.go",
		"README.txt",
		"numpad_poser.go",
	}
	for _, src := range files {
		sPath := path.Join("scripts", src)
		data, err := scriptScripts.ReadFile(sPath)
		if err != nil {
			log.Printf("read embedded %s: %v", sPath, err)
			continue
		}
		base := filepath.Base(sPath)
		dst := filepath.Join(dir, base)
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			log.Printf("write %s: %v", dst, err)
			continue
		}
	}
}

// ensurescriptAPI removed: the editor stub now ships in scripts/gt.

var scriptAllowedPkgs = []string{
	"bytes/bytes",
	"encoding/json/json",
	"errors/errors",
	"fmt/fmt",
	"math/big/big",
	"math/math",
	"math/rand/rand",
	"regexp/regexp",
	"sort/sort",
	"strconv/strconv",
	"strings/strings",
	"time/time",
	"unicode/utf8/utf8",
}

const scriptGoroutineLimit = 1024

func init() {
	go scriptGoroutineWatchdog()
}

func scriptGoroutineWatchdog() {
	for {
		if runtime.NumGoroutine() > scriptGoroutineLimit {
			log.Printf("[script] goroutine limit exceeded; stopping all scripts")
			consoleMessage("[script] goroutine limit exceeded; stopping scripts")
			stopAllscripts()
			return
		}
		time.Sleep(time.Millisecond * 100)
	}
}

func restrictedStdlib() interp.Exports {
	restricted := interp.Exports{}
	for _, key := range scriptAllowedPkgs {
		if syms, ok := stdlib.Symbols[key]; ok {
			restricted[key] = syms
		}
	}
	return restricted
}

func scriptConsole(msg string) {
	if gs.scriptOutputDebug {
		consoleMessage(msg)
	}
}

func scriptLogEvent(owner, ev, data string) {
	if !gs.scriptEventDebug {
		return
	}
	line := fmt.Sprintf("[%s] %s: %s", owner, ev, data)
	scriptDebugMu.Lock()
	scriptDebugLines = append(scriptDebugLines, line)
	if len(scriptDebugLines) > 200 {
		scriptDebugLines = scriptDebugLines[len(scriptDebugLines)-200:]
	}
	scriptDebugMu.Unlock()
	refreshscriptDebug()
}

func scriptShowNotification(msg string) {
	showNotification(msg)
}

func scriptIsDisabled(owner string) bool {
	scriptMu.RLock()
	disabled := scriptDisabled[owner]
	scriptMu.RUnlock()
	return disabled
}

func scriptAddHotkey(owner, combo, command string) {
	if scriptIsDisabled(owner) {
		return
	}
	// Default script hotkeys to enabled on first add; users can disable them
	// in the Hotkeys window. Persisted preferences still override this.
	hk := Hotkey{Name: command, Combo: combo, Commands: []HotkeyCommand{{Command: command}}, Script: owner, Disabled: false}
	scriptHotkeyMu.RLock()
	if m := scriptHotkeyEnabled[owner]; m != nil {
		if m[combo] {
			hk.Disabled = false
		}
	}
	scriptHotkeyMu.RUnlock()
	hotkeysMu.Lock()
	for _, existing := range hotkeys {
		if existing.Script == owner && existing.Combo == combo {
			hotkeysMu.Unlock()
			return
		}
	}
	hotkeys = append(hotkeys, hk)
	hotkeysMu.Unlock()
	scriptHotkeyMu.Lock()
	if hk.Disabled {
		if m := scriptHotkeyEnabled[owner]; m != nil {
			delete(m, combo)
			if len(m) == 0 {
				delete(scriptHotkeyEnabled, owner)
			}
		}
	} else {
		m := scriptHotkeyEnabled[owner]
		if m == nil {
			m = map[string]bool{}
			scriptHotkeyEnabled[owner] = m
		}
		m[combo] = true
	}
	scriptHotkeyMu.Unlock()
	refreshHotkeysList()
	saveHotkeys()
	name := scriptDisplayNames[owner]
	if name == "" {
		name = owner
	}
	msg := fmt.Sprintf("[script:%s] hotkey added: %s -> %s", name, combo, command)
	if gs.scriptOutputDebug {
		consoleMessage(msg)
	}
	log.Print(msg)
}

// HotkeyEvent describes a triggered hotkey in a compact form.
// Combo is the full combo string (e.g., "Ctrl-Shift-D", "RightClick").
// Parts is Combo split at '-', and Trigger is usually the last element
// (e.g., "D", "RightClick", "WheelUp").
type HotkeyEvent struct {
	Combo   string
	Parts   []string
	Trigger string
}

var (
	scriptHotkeyFnMu sync.RWMutex
	scriptHotkeyFns  = map[string]map[string]func(HotkeyEvent){}
)

// scriptAddHotkeyFn registers a function-based hotkey for a script.
// The hotkey appears in the "script Hotkeys" list and can be enabled/disabled
// like command-based hotkeys, but when pressed it will call the provided
// handler instead of emitting a slash command.
func scriptAddHotkeyFn(owner, combo string, handler func(HotkeyEvent)) {
	if scriptIsDisabled(owner) || handler == nil {
		return
	}
	// Remember handler
	scriptHotkeyFnMu.Lock()
	m := scriptHotkeyFns[owner]
	if m == nil {
		m = map[string]func(HotkeyEvent){}
		scriptHotkeyFns[owner] = m
	}
	m[combo] = handler
	scriptHotkeyFnMu.Unlock()

	// Ensure a visible toggleable hotkey entry exists for this script+combo.
	// Function-based hotkeys default to enabled on first add.
	hk := Hotkey{Name: "", Combo: combo, Script: owner, Disabled: false}
	scriptHotkeyMu.RLock()
	if m := scriptHotkeyEnabled[owner]; m != nil {
		if m[combo] {
			hk.Disabled = false
		}
	}
	scriptHotkeyMu.RUnlock()
	hotkeysMu.Lock()
	for _, existing := range hotkeys {
		if existing.Script == owner && existing.Combo == combo {
			hotkeysMu.Unlock()
			refreshHotkeysList()
			saveHotkeys()
			return
		}
	}
	hotkeys = append(hotkeys, hk)
	hotkeysMu.Unlock()
	scriptHotkeyMu.Lock()
	if hk.Disabled {
		if m := scriptHotkeyEnabled[owner]; m != nil {
			delete(m, combo)
			if len(m) == 0 {
				delete(scriptHotkeyEnabled, owner)
			}
		}
	} else {
		m := scriptHotkeyEnabled[owner]
		if m == nil {
			m = map[string]bool{}
			scriptHotkeyEnabled[owner] = m
		}
		m[combo] = true
	}
	scriptHotkeyMu.Unlock()
	refreshHotkeysList()
	saveHotkeys()
	name := scriptDisplayNames[owner]
	if name == "" {
		name = owner
	}
	msg := fmt.Sprintf("[script:%s] hotkey added: %s -> <function>", name, combo)
	if gs.scriptOutputDebug {
		consoleMessage(msg)
	}
	log.Print(msg)
}

func scriptGetHotkeyFn(owner, combo string) (func(HotkeyEvent), bool) {
	scriptHotkeyFnMu.RLock()
	defer scriptHotkeyFnMu.RUnlock()
	if m := scriptHotkeyFns[owner]; m != nil {
		if fn := m[combo]; fn != nil {
			return fn, true
		}
	}
	return nil, false
}

// script command registries.
type scriptCommandHandler func(args string)

type triggerHandler struct {
	owner string
	name  string
	flags int
	fn    func(string)
}

type inputHandler struct {
	owner string
	fn    func(string) string
}

// chatHandler holds a script-owned handler for all chat messages.
type chatHandler struct {
	owner string
	fn    func(string)
}

var (
	scriptCommands        = map[string]scriptCommandHandler{}
	scriptMu              sync.RWMutex
	scriptNames           = map[string]bool{}
	scriptDisplayNames    = map[string]string{}
	scriptAuthors         = map[string]string{}
	scriptCategories      = map[string]string{}
	scriptSubCategories   = map[string]string{}
	scriptInvalid         = map[string]bool{}
	scriptDisabled        = map[string]bool{}
	scriptEnabledFor      = map[string]scriptScope{}
	scriptPaths           = map[string]string{}
	scriptTerminators     = map[string]func(){}
	scriptTriggers        = map[string][]triggerHandler{}
	scriptConsoleTriggers = map[string][]triggerHandler{}
	triggerHandlersMu     sync.RWMutex
	// Handlers that receive every chat message (no phrase filtering)
	scriptChatHandlers  []chatHandler
	chatHandlersMu      sync.RWMutex
	scriptInputHandlers []inputHandler
	inputHandlersMu     sync.RWMutex
	scriptCommandOwners = map[string]string{}
	scriptSendHistory   = map[string][]time.Time{}
	scriptModTime       time.Time
	scriptModCheck      time.Time
	// timers per script owner
	scriptTimers      = map[string][]*time.Timer{}
	scriptTickerStops = map[string][]chan struct{}{}
	scriptTickWaiters = map[string][]*tickWaiter{}

	// Per-script world overlay draw operations.
	scriptOverlayOps = map[string][]overlayOp{}
	overlayMu        sync.RWMutex

	scriptDebugLines []string
	scriptDebugMu    sync.Mutex
)

// overlayOp describes a simple draw command for the world overlay.
type overlayOp struct {
	kind       int // 0=rect, 1=text, 2=image
	x, y       int // world coordinates (top-left origin)
	w, h       int // for rect
	r, g, b, a uint8
	text       string // for text
	id         uint16 // for image (CL_Images pict ID)
}

type tickWaiter struct {
	remain int
	done   chan struct{}
}

func scriptSleepTicks(owner string, ticks int) {
	if ticks <= 0 {
		return
	}
	w := &tickWaiter{remain: ticks, done: make(chan struct{}, 1)}
	scriptMu.Lock()
	scriptTickWaiters[owner] = append(scriptTickWaiters[owner], w)
	scriptMu.Unlock()
	<-w.done
}

func scriptAdvanceTick() {
	scriptMu.Lock()
	for owner, list := range scriptTickWaiters {
		n := 0
		for _, w := range list {
			if w == nil {
				continue
			}
			w.remain--
			if w.remain <= 0 {
				select {
				case w.done <- struct{}{}:
				default:
				}
			} else {
				list[n] = w
				n++
			}
		}
		if n == 0 {
			delete(scriptTickWaiters, owner)
		} else {
			scriptTickWaiters[owner] = list[:n]
		}
	}
	scriptMu.Unlock()
}

const (
	minscriptMetaLen = 2
	maxscriptMetaLen = 40
)

func invalidscriptValue(s string) bool {
	l := len(s)
	return l < minscriptMetaLen || l > maxscriptMetaLen
}

// scriptRegisterCommand lets scripts handle a local slash command like
// "/example". The name should be without the leading slash and will be
// matched case-insensitively.
func scriptRegisterCommand(owner, name string, handler scriptCommandHandler) {
	if name == "" || handler == nil {
		return
	}
	if scriptIsDisabled(owner) {
		return
	}
	key := strings.ToLower(strings.TrimPrefix(name, "/"))
	scriptMu.Lock()
	if _, exists := scriptCommands[key]; exists {
		scriptMu.Unlock()
		msg := fmt.Sprintf("[script] command conflict: /%s already registered", key)
		consoleMessage(msg)
		log.Print(msg)
		return
	}
	scriptCommands[key] = handler
	scriptCommandOwners[key] = owner
	scriptMu.Unlock()
	consoleMessage("[script] command registered: /" + key)
	log.Printf("[script] command registered: /%s", key)
}

// scriptRunCommand echoes and enqueues a command for immediate sending.
func scriptRunCommand(owner, cmd string) {
	if scriptIsDisabled(owner) {
		return
	}
	if recordscriptSend(owner) {
		return
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	consoleMessage("> " + cmd)
	enqueueCommand(cmd)
	nextCommand()
}

// scriptEnqueueCommand enqueues a command to be sent on the next tick without echoing.
func scriptEnqueueCommand(owner, cmd string) {
	if scriptIsDisabled(owner) {
		return
	}
	if recordscriptSend(owner) {
		return
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	enqueueCommand(cmd)
}

func loadscriptSource(owner, name, path string, src []byte, restricted interp.Exports) {
	scriptRemoveConfig(owner)
	i := interp.New(interp.Options{})
	if len(restricted) > 0 {
		i.Use(restricted)
	}
	i.Use(exportsForscript(owner))
	scriptMu.Lock()
	scriptDisabled[owner] = false
	scriptMu.Unlock()
	// Strip build tags like //go:build which are for the Go toolchain only.
	src = stripGoBuildDirectives(src)
	if _, err := i.Eval(string(src)); err != nil {
		log.Printf("script %s: %v", path, err)
		consoleMessage("[script] load error for " + path + ": " + err.Error())
		disablescript(owner, "load error")
		return
	}
	if v, err := i.Eval("Terminate"); err == nil {
		if fn, ok := v.Interface().(func()); ok {
			scriptMu.Lock()
			scriptTerminators[owner] = fn
			scriptMu.Unlock()
		}
	}
	if v, err := i.Eval("Init"); err == nil {
		if fn, ok := v.Interface().(func()); ok {
			go fn()
		}
	}
	log.Printf("loaded script %s", path)
	consoleMessage("[script] loaded: " + name)
}

// stripGoBuildDirectives removes leading build constraints (//go:build, // +build)
// which are meaningful to the Go toolchain but can confuse the interpreter.
func stripGoBuildDirectives(src []byte) []byte {
	lines := strings.Split(string(src), "\n")
	out := make([]string, 0, len(lines))
	i := 0
	// Skip initial build constraint lines and following blank lines until package clause
	for i < len(lines) {
		l := strings.TrimSpace(lines[i])
		if strings.HasPrefix(l, "package ") {
			break
		}
		if strings.HasPrefix(l, "//go:build") || strings.HasPrefix(l, "// +build") || l == "" {
			i++
			continue
		}
		// Any other pre-package content: keep it
		break
	}
	if i > 0 {
		out = append(out, lines[i:]...)
		return []byte(strings.Join(out, "\n"))
	}
	return src
}

func enablescript(owner string) {
	scriptMu.RLock()
	path := scriptPaths[owner]
	name := scriptDisplayNames[owner]
	scriptMu.RUnlock()
	if path == "" {
		return
	}
	src, err := os.ReadFile(path)
	if err != nil {
		log.Printf("read script %s: %v", path, err)
		consoleMessage("[script] read error for " + path + ": " + err.Error())
		return
	}
	loadscriptSource(owner, name, path, src, restrictedStdlib())
	settingsDirty = true
	saveSettings()
	refreshscriptsWindow()
}

func recordscriptSend(owner string) bool {
	if !gs.ScriptSpamKill {
		return false
	}
	now := time.Now()
	cutoff := now.Add(-5 * time.Second)
	scriptMu.Lock()
	times := scriptSendHistory[owner]
	n := 0
	for _, t := range times {
		if t.After(cutoff) {
			times[n] = t
			n++
		}
	}
	times = times[:n]
	times = append(times, now)
	scriptSendHistory[owner] = times
	count := len(times)
	scriptMu.Unlock()
	if count > 30 {
		disablescript(owner, "sent too many lines")
		return true
	}
	return false
}

func disablescript(owner, reason string) {
	scriptMu.Lock()
	scriptDisabled[owner] = true
	if reason != "disabled for this character" && reason != "reloaded" {
		delete(scriptEnabledFor, owner)
	}
	term := scriptTerminators[owner]
	delete(scriptTerminators, owner)
	scriptMu.Unlock()
	if term != nil {
		go term()
	}
	for _, hk := range scriptHotkeys(owner) {
		scriptRemoveHotkey(owner, hk.Combo)
	}
	scriptRemoveShortcuts(owner)
	scriptRemoveConfig(owner)
	if scriptConfigWin != nil && scriptConfigOwner == owner {
		scriptConfigWin.Close()
		scriptConfigWin = nil
		scriptConfigOwner = ""
	}
	inputHandlersMu.Lock()
	for i := len(scriptInputHandlers) - 1; i >= 0; i-- {
		if scriptInputHandlers[i].owner == owner {
			scriptInputHandlers = append(scriptInputHandlers[:i], scriptInputHandlers[i+1:]...)
		}
	}
	inputHandlersMu.Unlock()
	triggerHandlersMu.Lock()
	for phrase, hs := range scriptTriggers {
		n := 0
		for _, h := range hs {
			if h.owner != owner {
				hs[n] = h
				n++
			}
		}
		if n == 0 {
			delete(scriptTriggers, phrase)
		} else {
			scriptTriggers[phrase] = hs[:n]
		}
	}
	for phrase, hs := range scriptConsoleTriggers {
		n := 0
		for _, h := range hs {
			if h.owner != owner {
				hs[n] = h
				n++
			}
		}
		if n == 0 {
			delete(scriptConsoleTriggers, phrase)
		} else {
			scriptConsoleTriggers[phrase] = hs[:n]
		}
	}
	triggerHandlersMu.Unlock()
	// Remove function hotkeys
	scriptHotkeyFnMu.Lock()
	delete(scriptHotkeyFns, owner)
	scriptHotkeyFnMu.Unlock()
	refreshTriggersList()
	playerHandlersMu.Lock()
	for i := len(scriptPlayerHandlers) - 1; i >= 0; i-- {
		if scriptPlayerHandlers[i].owner == owner {
			scriptPlayerHandlers = append(scriptPlayerHandlers[:i], scriptPlayerHandlers[i+1:]...)
		}
	}
	playerHandlersMu.Unlock()
	// Remove all-chat handlers for this script
	chatHandlersMu.Lock()
	for i := len(scriptChatHandlers) - 1; i >= 0; i-- {
		if scriptChatHandlers[i].owner == owner {
			scriptChatHandlers = append(scriptChatHandlers[:i], scriptChatHandlers[i+1:]...)
		}
	}
	chatHandlersMu.Unlock()
	// Clear overlay ops
	overlayMu.Lock()
	delete(scriptOverlayOps, owner)
	overlayMu.Unlock()
	// Stop any timers/tickers and tick waiters for this script
	scriptMu.Lock()
	if list := scriptTimers[owner]; len(list) > 0 {
		for _, t := range list {
			if t != nil {
				t.Stop()
			}
		}
		delete(scriptTimers, owner)
	}
	if stops := scriptTickerStops[owner]; len(stops) > 0 {
		for _, ch := range stops {
			if ch != nil {
				close(ch)
			}
		}
		delete(scriptTickerStops, owner)
	}
	if waits := scriptTickWaiters[owner]; len(waits) > 0 {
		for _, w := range waits {
			if w != nil {
				select {
				case w.done <- struct{}{}:
				default:
				}
			}
		}
		delete(scriptTickWaiters, owner)
	}
	scriptMu.Unlock()
	scriptMu.Lock()
	for cmd, o := range scriptCommandOwners {
		if o == owner {
			delete(scriptCommands, cmd)
			delete(scriptCommandOwners, cmd)
		}
	}
	delete(scriptSendHistory, owner)
	disp := scriptDisplayNames[owner]
	scriptMu.Unlock()
	if disp == "" {
		disp = owner
	}
	consoleMessage("[script:" + disp + "] stopped: " + reason)
	settingsDirty = true
	saveSettings()
	refreshscriptsWindow()
}

func stopAllscripts() {
	scriptMu.RLock()
	owners := make([]string, 0, len(scriptDisplayNames))
	for o := range scriptDisplayNames {
		if !scriptDisabled[o] {
			owners = append(owners, o)
		}
	}
	scriptMu.RUnlock()
	for _, o := range owners {
		disablescript(o, "stopped by user")
	}
	if len(owners) > 0 {
		commandQueue = nil
		pendingCommand = ""
		consoleMessage("[script] all scripts stopped")
	}
}

func applyEnabledScripts() {
	scriptMu.RLock()
	owners := make([]string, 0, len(scriptDisplayNames))
	for o := range scriptDisplayNames {
		owners = append(owners, o)
	}
	scriptMu.RUnlock()
	for _, o := range owners {
		scriptMu.RLock()
		scope := scriptEnabledFor[o]
		disabled := scriptDisabled[o]
		invalid := scriptInvalid[o]
		scriptMu.RUnlock()
		if invalid {
			scriptMu.Lock()
			scriptDisabled[o] = true
			scriptMu.Unlock()
			continue
		}
		// Enable when set to all, or when the scope includes the active
		// character. If not logged in, fall back to LastCharacter.
		effChar := playerName
		if effChar == "" {
			effChar = gs.LastCharacter
		}
		shouldEnable := scope.enablesFor(effChar)
		if disabled && shouldEnable {
			enablescript(o)
		} else if !disabled && !shouldEnable {
			disablescript(o, "disabled for this character")
		} else {
			scriptMu.Lock()
			scriptDisabled[o] = !shouldEnable
			scriptMu.Unlock()
		}
	}
}

func setscriptEnabled(owner string, char, all bool) {
	scriptMu.Lock()
	if scriptInvalid[owner] {
		scriptMu.Unlock()
		return
	}
	s := scriptEnabledFor[owner]
	if all {
		s.All = true
		s.Chars = nil
	} else if char {
		effChar := playerName
		if effChar == "" {
			effChar = gs.LastCharacter
		}
		if effChar != "" {
			s.All = false
			s.addChar(effChar)
		}
	} else {
		effChar := playerName
		if effChar == "" {
			effChar = gs.LastCharacter
		}
		if effChar != "" {
			s.removeChar(effChar)
		} else {
			s = scriptScope{}
		}
	}
	if s.empty() {
		delete(scriptEnabledFor, owner)
	} else {
		scriptEnabledFor[owner] = s
	}
	scriptMu.Unlock()
	applyEnabledScripts()
	saveSettings()
	refreshscriptsWindow()
}

// clearscriptScope removes all enablement for a script (no all, no characters)
// and refreshes apply/save/UI. Used by the UI when unchecking the "All" box
// to explicitly stop a script regardless of any per-character flags.
func clearscriptScope(owner string) {
	scriptMu.Lock()
	delete(scriptEnabledFor, owner)
	scriptMu.Unlock()
	// Stop scheduled timers/tickers for this script
	if list := scriptTimers[owner]; len(list) > 0 {
		for _, t := range list {
			if t != nil {
				t.Stop()
			}
		}
		delete(scriptTimers, owner)
	}
	if stops := scriptTickerStops[owner]; len(stops) > 0 {
		for _, ch := range stops {
			if ch != nil {
				close(ch)
			}
		}
		delete(scriptTickerStops, owner)
	}
	applyEnabledScripts()
	saveSettings()
	refreshscriptsWindow()
}

func scriptPlayerName() string {
	return playerName
}

func scriptPlayers() []Player {
	ps := getPlayers()
	out := make([]Player, len(ps))
	copy(out, ps)
	return out
}

func scriptInventory() []InventoryItem {
	return getInventory()
}

type Stats struct {
	HP, HPMax           int
	SP, SPMax           int
	Balance, BalanceMax int
}

func scriptInputText() string {
	inputMu.Lock()
	txt := string(inputText)
	inputMu.Unlock()
	return txt
}

func scriptSetInputText(text string) {
	inputMu.Lock()
	inputText = []rune(text)
	inputActive = true
	inputPos = len(inputText)
	inputMu.Unlock()
}

func scriptEquip(owner string, id uint16) {
	if recordscriptSend(owner) {
		return
	}
	items := getInventory()
	idx := -1
	for _, it := range items {
		if it.ID != id {
			continue
		}
		if it.Equipped {
			name := it.Name
			if name == "" {
				name = fmt.Sprintf("%d", id)
			}
			consoleMessage(name + " already equipped, skipping")
			return
		}
		if idx < 0 {
			idx = it.IDIndex
		}
	}
	if idx < 0 {
		return
	}
	queueEquipCommand(id, idx)
	equipInventoryItem(id, idx, true)
}

// scriptEquipByName equips the first inventory item whose name matches the
// provided name (case-insensitive). If the item is already equipped, it skips.
func scriptEquipByName(owner, name string) {
	if recordscriptSend(owner) {
		return
	}
	targetName := strings.ToLower(strings.TrimSpace(name))
	if targetName == "" {
		return
	}
	items := getInventory()
	var id uint16
	idx := -1
	for _, it := range items {
		if strings.ToLower(it.Name) != targetName {
			continue
		}
		// If any matching item is already equipped, skip as redundant.
		if it.Equipped {
			n := it.Name
			if n == "" {
				n = targetName
			}
			consoleMessage(n + " already equipped, skipping")
			return
		}
		// Prefer the first match; use its ID and server-provided index.
		id = it.ID
		if idx < 0 {
			idx = it.IDIndex
		}
		// Do not break; if there are multiple matches, the first branch sets id/idx
		// and we continue to see if an equipped instance exists to early-out.
	}
	if idx < 0 {
		return
	}
	queueEquipCommand(id, idx)
	equipInventoryItem(id, idx, true)
}

func scriptUnequip(owner string, id uint16) {
	if recordscriptSend(owner) {
		return
	}
	items := getInventory()
	equipped := false
	for _, it := range items {
		if it.ID == id && it.Equipped {
			equipped = true
			break
		}
	}
	if !equipped {
		return
	}
	pendingCommand = fmt.Sprintf("/unequip %d", id)
	equipInventoryItem(id, -1, false)
}

// scriptEquipPartial equips the first item whose name contains the pattern
// (case-insensitive). If a matching item is already equipped, it skips.
func scriptEquipPartial(owner, pattern string) {
	if recordscriptSend(owner) {
		return
	}
	p := strings.ToLower(strings.TrimSpace(pattern))
	if p == "" {
		return
	}
	items := getInventory()
	var id uint16
	idx := -1
	// If any matching item is already equipped, skip as redundant.
	for _, it := range items {
		if strings.Contains(strings.ToLower(it.Name), p) && it.Equipped {
			consoleMessage(it.Name + " already equipped, skipping")
			return
		}
	}
	for _, it := range items {
		if strings.Contains(strings.ToLower(it.Name), p) {
			id = it.ID
			idx = it.IDIndex
			break
		}
	}
	if idx < 0 {
		return
	}
	queueEquipCommand(id, idx)
	equipInventoryItem(id, idx, true)
}

// scriptUnequipPartial unequips any equipped item whose name contains the
// provided pattern (case-insensitive).
func scriptUnequipPartial(owner, pattern string) {
	if recordscriptSend(owner) {
		return
	}
	p := strings.ToLower(strings.TrimSpace(pattern))
	if p == "" {
		return
	}
	items := getInventory()
	for _, it := range items {
		if it.Equipped && strings.Contains(strings.ToLower(it.Name), p) {
			pendingCommand = fmt.Sprintf("/unequip %d", it.ID)
			equipInventoryItem(it.ID, -1, false)
			return
		}
	}
}

// scriptUnequipByName unequips an item by name (case-insensitive). If multiple
// items share the name, it unequips any equipped instance.
func scriptUnequipByName(owner, name string) {
	if recordscriptSend(owner) {
		return
	}
	targetName := strings.ToLower(strings.TrimSpace(name))
	if targetName == "" {
		return
	}
	items := getInventory()
	var id uint16
	equipped := false
	for _, it := range items {
		if strings.ToLower(it.Name) != targetName {
			continue
		}
		if it.Equipped {
			id = it.ID
			equipped = true
			break
		}
		// Remember an ID even if not equipped yet; we still require equipped=true
		// to proceed, matching previous Unequip behavior.
		if id == 0 {
			id = it.ID
		}
	}
	if !equipped {
		return
	}
	pendingCommand = fmt.Sprintf("/unequip %d", id)
	equipInventoryItem(id, -1, false)
}

func scriptRegisterInputHandler(owner string, fn func(string) string) {
	if scriptIsDisabled(owner) || fn == nil {
		return
	}
	inputHandlersMu.Lock()
	scriptInputHandlers = append(scriptInputHandlers, inputHandler{owner: owner, fn: fn})
	inputHandlersMu.Unlock()
}

// Chat trigger kinds for filtering messages by source.
const (
	ChatAny      = 1 << iota // match any chat message
	ChatPlayer               // message from a known player (not NPC)
	ChatNPC                  // message from a known NPC
	ChatCreature             // message from an unknown/non-player speaker
	ChatSelf                 // message from yourself
	ChatOther                // message not from yourself
)

// scriptRegisterChat registers a chat trigger with optional name and kind flags.
func scriptRegisterChat(owner, name string, phrases []string, flags int, fn func(string)) {
	if scriptIsDisabled(owner) || fn == nil {
		return
	}
	triggerHandlersMu.Lock()
	name = strings.ToLower(name)
	for _, p := range phrases {
		if p == "" {
			continue
		}
		p = strings.ToLower(p)
		scriptTriggers[p] = append(scriptTriggers[p], triggerHandler{owner: owner, name: name, flags: flags, fn: fn})
	}
	triggerHandlersMu.Unlock()
	refreshTriggersList()
}

// Back-compat wrapper for older API without flags.
func scriptRegisterTriggers(owner, name string, phrases []string, fn func()) {
	if fn == nil {
		return
	}
	scriptRegisterChat(owner, name, phrases, ChatAny, func(string) { fn() })
}

// New console registration with message parameter
func scriptRegisterConsole(owner string, phrases []string, fn func(string)) {
	if scriptIsDisabled(owner) || fn == nil {
		return
	}
	triggerHandlersMu.Lock()
	for _, p := range phrases {
		if p == "" {
			continue
		}
		p = strings.ToLower(p)
		scriptConsoleTriggers[p] = append(scriptConsoleTriggers[p], triggerHandler{owner: owner, fn: fn})
	}
	triggerHandlersMu.Unlock()
	refreshTriggersList()
}

// Back-compat: old console registration without msg parameter
func scriptRegisterConsoleTriggers(owner string, phrases []string, fn func()) {
	if fn == nil {
		return
	}
	scriptRegisterConsole(owner, phrases, func(string) { fn() })
}

// scriptAutoReply sends a command when a chat message contains trigger.
func scriptAutoReply(owner, trigger, command string) {
	if scriptIsDisabled(owner) || trigger == "" || command == "" {
		return
	}
	scriptRegisterTriggers(owner, "", []string{trigger}, func() {
		scriptEnqueueCommand(owner, command)
	})
}

func scriptRegisterTrigger(owner string, phrase string, fn func(string)) {
	if scriptIsDisabled(owner) || fn == nil {
		return
	}
	if len(phrase) < 2 {
		return
	}
	triggerHandlersMu.Lock()
	phrase = strings.ToLower(phrase)
	scriptTriggers[phrase] = append(scriptTriggers[phrase], triggerHandler{owner: owner, fn: fn})
	triggerHandlersMu.Unlock()
}

func scriptRegisterPlayerHandler(owner string, fn func(Player)) {
	if scriptIsDisabled(owner) || fn == nil {
		return
	}
	playerHandlersMu.Lock()
	scriptPlayerHandlers = append(scriptPlayerHandlers, playerHandler{owner: owner, fn: fn})
	playerHandlersMu.Unlock()
}

// scriptRegisterChatHandler registers a callback invoked for every chat message.
func scriptRegisterChatHandler(owner string, fn func(string)) {
	if scriptIsDisabled(owner) || fn == nil {
		return
	}
	chatHandlersMu.Lock()
	scriptChatHandlers = append(scriptChatHandlers, chatHandler{owner: owner, fn: fn})
	chatHandlersMu.Unlock()
}

func runInputHandlers(txt string) string {
	inputHandlersMu.RLock()
	handlers := append([]inputHandler{}, scriptInputHandlers...)
	inputHandlersMu.RUnlock()
	for _, h := range handlers {
		if h.fn != nil {
			scriptLogEvent(h.owner, "InputHandler", txt)
			txt = h.fn(txt)
		}
	}
	return txt
}

func runChatTriggers(msg string) {
	triggerHandlersMu.RLock()
	// Determine message flags and speaker for filtering.
	speaker := chatSpeaker(msg)
	msgFlags := ChatAny
	if strings.EqualFold(speaker, playerName) && playerName != "" {
		msgFlags |= ChatSelf
	} else {
		msgFlags |= ChatOther
	}
	if speaker != "" {
		classified := false
		playersMu.RLock()
		p, ok := players[speaker]
		playersMu.RUnlock()
		if ok && p != nil && p.IsNPC {
			msgFlags |= ChatNPC
			classified = true
		} else {
			if isNPCDescriptor(speaker) {
				msgFlags |= ChatNPC
				classified = true
				if ok && p != nil && !p.IsNPC {
					playersMu.Lock()
					if np, ok := players[speaker]; ok && np != nil {
						np.IsNPC = true
						playersDirty = true
					}
					playersMu.Unlock()
				}
			} else if ok {
				msgFlags |= ChatPlayer
				classified = true
			} else if strings.EqualFold(speaker, playerName) && playerName != "" {
				msgFlags |= ChatPlayer
				classified = true
			}
		}
		if !classified {
			msgFlags |= ChatCreature
		}
	} else {
		msgFlags |= ChatCreature
	}
	for phrase, hs := range scriptTriggers {
		if strings.Contains(strings.ToLower(msg), phrase) {
			for _, h := range hs {
				if h.name != "" && h.name != strings.ToLower(speaker) {
					continue
				}
				f := h.flags
				if f == 0 {
					f = ChatAny
				}
				if (f & msgFlags) != 0 {
					owner := h.owner
					fn := h.fn
					scriptLogEvent(owner, "ChatTrigger", fmt.Sprintf("%q %q", phrase, msg))
					go fn(msg)
				}
			}
		}
	}
	triggerHandlersMu.RUnlock()

	// Dispatch all-chat handlers (no phrase filtering).
	chatHandlersMu.RLock()
	handlers := append([]chatHandler{}, scriptChatHandlers...)
	chatHandlersMu.RUnlock()
	for _, h := range handlers {
		if h.fn != nil {
			scriptLogEvent(h.owner, "ChatHandler", msg)
			go h.fn(msg)
		}
	}
}

func isNPCDescriptor(name string) bool {
	if name == "" {
		return false
	}
	stateMu.Lock()
	for _, d := range state.descriptors {
		if d.Name == name {
			isNPC := d.Type == kDescNPC
			stateMu.Unlock()
			return isNPC
		}
	}
	stateMu.Unlock()
	return false
}

func runConsoleTriggers(msg string) {
	triggerHandlersMu.RLock()
	msgLower := strings.ToLower(msg)
	for phrase, hs := range scriptConsoleTriggers {
		if strings.Contains(msgLower, phrase) {
			for _, h := range hs {
				scriptLogEvent(h.owner, "ConsoleTrigger", fmt.Sprintf("%q %q", phrase, msg))
				fn := h.fn
				go fn(msg)
			}
		}
	}
	triggerHandlersMu.RUnlock()
}

func scriptPlaySound(ids []uint16) {
	playSound(ids)
}

// ---- Overlay helpers (called by script exports) ----
func scriptOverlayClear(owner string) {
	overlayMu.Lock()
	delete(scriptOverlayOps, owner)
	overlayMu.Unlock()
}

func scriptOverlayRect(owner string, x, y, w, h int, r, g, b, a uint8) {
	if w <= 0 || h <= 0 {
		return
	}
	overlayMu.Lock()
	scriptOverlayOps[owner] = append(scriptOverlayOps[owner], overlayOp{kind: 0, x: x, y: y, w: w, h: h, r: r, g: g, b: b, a: a})
	overlayMu.Unlock()
}

func scriptOverlayText(owner string, x, y int, txt string, r, g, b, a uint8) {
	if txt == "" {
		return
	}
	overlayMu.Lock()
	scriptOverlayOps[owner] = append(scriptOverlayOps[owner], overlayOp{kind: 1, x: x, y: y, text: txt, r: r, g: g, b: b, a: a})
	overlayMu.Unlock()
}

func scriptOverlayImage(owner string, id uint16, x, y int) {
	if id == 0xffff || id == 0 {
		return
	}
	overlayMu.Lock()
	scriptOverlayOps[owner] = append(scriptOverlayOps[owner], overlayOp{kind: 2, x: x, y: y, id: id, a: 255, r: 255, g: 255, b: 255})
	overlayMu.Unlock()
}

func refreshscriptMod() {
	if isWASM {
		return
	}
	latest := time.Time{}
	for _, dir := range scriptSearchDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			if info, err := e.Info(); err == nil {
				if info.ModTime().After(latest) {
					latest = info.ModTime()
				}
			}
		}
	}
	scriptModTime = latest
}

type scriptInfo struct {
	name        string
	author      string
	category    string
	subCategory string
	path        string
	src         []byte
	invalid     bool
	apiVer      int
}

func scanscripts(scriptDirs []string, dup func(name, path string)) map[string]scriptInfo {
	nameRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+scriptName\s*=\s*"([^"]+)"`)
	authorRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+scriptAuthor\s*=\s*"([^"]+)"`)
	categoryRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+scriptCategory\s*=\s*"([^"]+)"`)
	subCategoryRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+scriptSubCategory\s*=\s*"([^"]+)"`)
	apiVerRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+scriptAPIVersion\s*=\s*([0-9]+)\s*$`)
	scripts := map[string]scriptInfo{}
	seenNames := map[string]bool{}
	for _, dir := range scriptDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Printf("read script dir %s: %v", dir, err)
			}
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			src, err := os.ReadFile(path)
			if err != nil {
				log.Printf("read script %s: %v", path, err)
				continue
			}
			nameMatch := nameRE.FindSubmatch(src)
			base := strings.TrimSuffix(e.Name(), ".go")
			name := base
			if len(nameMatch) >= 2 {
				name = strings.TrimSpace(string(nameMatch[1]))
			}
			catMatch := categoryRE.FindSubmatch(src)
			category := ""
			if len(catMatch) >= 2 {
				category = strings.TrimSpace(string(catMatch[1]))
			}
			subMatch := subCategoryRE.FindSubmatch(src)
			subCategory := ""
			if len(subMatch) >= 2 {
				subCategory = strings.TrimSpace(string(subMatch[1]))
			}
			author := ""
			if match := authorRE.FindSubmatch(src); len(match) >= 2 {
				author = strings.TrimSpace(string(match[1]))
			}
			invalid := false
			apiVer := 0
			if len(nameMatch) < 2 || name == "" || invalidscriptValue(name) {
				if len(nameMatch) < 2 || name == "" {
					consoleMessage("[script] missing name: " + path)
					name = base
				} else {
					consoleMessage("[script] invalid name: " + path)
				}
				invalid = true
			}
			if author == "" || invalidscriptValue(author) {
				if author == "" {
					consoleMessage("[script] missing author: " + path)
				} else {
					consoleMessage("[script] invalid author: " + path)
				}
				invalid = true
			}
			if category == "" || invalidscriptValue(category) {
				if category == "" {
					consoleMessage("[script] missing category: " + path)
				} else {
					consoleMessage("[script] invalid category: " + path)
				}
				invalid = true
			}
			if m := apiVerRE.FindSubmatch(src); len(m) >= 2 {
				if n, err := strconv.Atoi(strings.TrimSpace(string(m[1]))); err == nil {
					apiVer = n
				}
			}
			lower := strings.ToLower(name)
			if seenNames[lower] {
				if dup != nil {
					dup(name, path)
				}
				continue
			}
			seenNames[lower] = true
			owner := name + "_" + base
			scripts[owner] = scriptInfo{
				name:        name,
				author:      author,
				category:    category,
				subCategory: subCategory,
				path:        path,
				src:         src,
				invalid:     invalid,
				apiVer:      apiVer,
			}
		}
	}
	return scripts
}

func rescanscripts() {
	if isWASM {
		return
	}
	scanned := scanscripts(scriptSearchDirs(), nil)

	scriptMu.RLock()
	oldDisabled := make(map[string]bool, len(scriptDisabled))
	for o, d := range scriptDisabled {
		oldDisabled[o] = d
	}
	oldOwners := make(map[string]struct{}, len(scriptDisplayNames))
	for o := range scriptDisplayNames {
		oldOwners[o] = struct{}{}
	}
	scriptMu.RUnlock()

	for o := range oldOwners {
		if _, ok := scanned[o]; !ok {
			disablescript(o, "removed")
		}
	}

	scriptMu.Lock()
	scriptDisplayNames = make(map[string]string, len(scanned))
	scriptPaths = make(map[string]string, len(scanned))
	scriptAuthors = make(map[string]string, len(scanned))
	scriptCategories = make(map[string]string, len(scanned))
	scriptSubCategories = make(map[string]string, len(scanned))
	scriptInvalid = make(map[string]bool, len(scanned))
	scriptDisabled = make(map[string]bool, len(scanned))
	newEnabled := map[string]scriptScope{}
	for o, info := range scanned {
		scriptDisplayNames[o] = info.name
		scriptPaths[o] = info.path
		scriptAuthors[o] = info.author
		scriptCategories[o] = info.category
		scriptSubCategories[o] = info.subCategory
		// Require a matching script API version
		invalid := info.invalid || info.apiVer != scriptAPICurrentVersion
		scriptInvalid[o] = invalid
		if invalid {
			scriptDisabled[o] = true
			continue
		}
		if en, ok := scriptEnabledFor[o]; ok {
			newEnabled[o] = en
		} else if gs.Enabledscripts != nil {
			if val, ok := gs.Enabledscripts[o]; ok {
				newEnabled[o] = scopeFromSettingValue(val)
			}
		}
		effChar := playerName
		if effChar == "" {
			effChar = gs.LastCharacter
		}
		scriptDisabled[o] = !newEnabled[o].enablesFor(effChar)
	}
	scriptEnabledFor = newEnabled
	scriptNames = make(map[string]bool, len(scanned))
	for _, info := range scanned {
		scriptNames[strings.ToLower(info.name)] = true
	}
	scriptMu.Unlock()

	applyEnabledScripts()
	refreshscriptsWindow()
	settingsDirty = true
}

func checkForScriptEdit() {
	if isWASM {
		return
	}
	if time.Since(scriptModCheck) < 500*time.Millisecond {
		return
	}
	scriptModCheck = time.Now()
	old := scriptModTime
	refreshscriptMod()
	if scriptModTime.After(old) {
		rescanscripts()
	}
}

func loadScripts() {
	if isWASM {
		return
	}
	ensureScriptsDir()
	ensureDefaultScripts()
	scanned := scanscripts(scriptSearchDirs(), func(name, path string) {
		log.Printf("script %s duplicate name %s", path, name)
		consoleMessage("[script] duplicate name: " + name)
	})

	scriptNames = make(map[string]bool, len(scanned))
	for o, info := range scanned {
		scriptNames[strings.ToLower(info.name)] = true
		s, ok := scriptEnabledFor[o]
		if !ok && gs.Enabledscripts != nil {
			if val, ok2 := gs.Enabledscripts[o]; ok2 {
				s = scopeFromSettingValue(val)
			}
		}
		effChar := playerName
		if effChar == "" {
			effChar = gs.LastCharacter
		}
		invalid := info.invalid || info.apiVer != scriptAPICurrentVersion
		disabled := invalid || !s.enablesFor(effChar)
		scriptMu.Lock()
		scriptDisplayNames[o] = info.name
		scriptCategories[o] = info.category
		scriptSubCategories[o] = info.subCategory
		scriptPaths[o] = info.path
		if !s.empty() {
			scriptEnabledFor[o] = s
		}
		scriptAuthors[o] = info.author
		scriptInvalid[o] = invalid
		scriptDisabled[o] = disabled
		scriptMu.Unlock()
		if !disabled {
			loadscriptSource(o, info.name, info.path, info.src, restrictedStdlib())
		}
	}
	hotkeysMu.Lock()
	for i := range hotkeys {
		if hotkeys[i].Script != "" {
			hotkeys[i].Disabled = true
		}
	}
	hotkeysMu.Unlock()
	refreshHotkeysList()
	refreshscriptsWindow()
	refreshscriptMod()
}

// scopeFromSettingValue converts a settings value into a scriptScope.
// Accepted values:
// - string("all"): All=true
// - string(name): include that character
// - []string: include all listed characters
// - []any (from JSON): include all listed string characters
// - bool(true): include LastCharacter if present
func scopeFromSettingValue(v any) scriptScope {
	s := scriptScope{}
	switch val := v.(type) {
	case string:
		if val == "all" {
			s.All = true
		} else if val != "" {
			s.addChar(val)
		}
	case []string:
		for _, n := range val {
			if n != "" {
				s.addChar(n)
			}
		}
	case []any:
		for _, e := range val {
			if str, ok := e.(string); ok && str != "" {
				s.addChar(str)
			}
		}
	case bool:
		if val && gs.LastCharacter != "" {
			s.addChar(gs.LastCharacter)
		}
	}
	return s
}
