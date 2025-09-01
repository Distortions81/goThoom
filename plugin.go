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

const pluginAPICurrentVersion = 1

type pluginScope struct {
	All   bool
	Chars map[string]bool
}

func (s pluginScope) enablesFor(effChar string) bool {
	if s.All {
		return true
	}
	if effChar == "" || s.Chars == nil {
		return false
	}
	return s.Chars[effChar]
}

func (s *pluginScope) addChar(name string) {
	if name == "" {
		return
	}
	if s.Chars == nil {
		s.Chars = map[string]bool{}
	}
	s.Chars[name] = true
}

func (s *pluginScope) removeChar(name string) {
	if s.Chars == nil || name == "" {
		return
	}
	delete(s.Chars, name)
}

func (s pluginScope) empty() bool { return !s.All && (s.Chars == nil || len(s.Chars) == 0) }

// Expose the plugin API under both a short and a module-qualified path so
// Yaegi can resolve imports regardless of how the script refers to it.
var basePluginExports = interp.Exports{
	// Short path used by simple plugin scripts: import "gt"
	// Yaegi expects keys as "importPath/pkgName".
	"gt/gt": {
		"Console":          reflect.ValueOf(pluginConsole),
		"ShowNotification": reflect.ValueOf(pluginShowNotification),
		"ClientVersion":    reflect.ValueOf(&clientVersion).Elem(),
		"PlayerName":       reflect.ValueOf(pluginPlayerName),
		"Players":          reflect.ValueOf(pluginPlayers),
		"Inventory":        reflect.ValueOf(pluginInventory),
		"InventoryItem":    reflect.ValueOf((*InventoryItem)(nil)),
		"PlaySound":        reflect.ValueOf(pluginPlaySound),
		"InputText":        reflect.ValueOf(pluginInputText),
		"SetInputText":     reflect.ValueOf(pluginSetInputText),
		"KeyJustPressed":   reflect.ValueOf(pluginKeyJustPressed),
		"MouseJustPressed": reflect.ValueOf(pluginMouseJustPressed),
		"MouseWheel":       reflect.ValueOf(pluginMouseWheel),
		"ClickInfo":        reflect.ValueOf((*ClickInfo)(nil)),
		"Mobile":           reflect.ValueOf((*Mobile)(nil)),
		"EquippedItems":    reflect.ValueOf(pluginEquippedItems),
		"HasItem":          reflect.ValueOf(pluginHasItem),
		"IgnoreCase":       reflect.ValueOf(pluginIgnoreCase),
		"StartsWith":       reflect.ValueOf(pluginStartsWith),
		"EndsWith":         reflect.ValueOf(pluginEndsWith),
		"Includes":         reflect.ValueOf(pluginIncludes),
		"Lower":            reflect.ValueOf(pluginLower),
		"Upper":            reflect.ValueOf(pluginUpper),
		"Trim":             reflect.ValueOf(pluginTrim),
		"TrimStart":        reflect.ValueOf(pluginTrimStart),
		"TrimEnd":          reflect.ValueOf(pluginTrimEnd),
		"Words":            reflect.ValueOf(pluginWords),
		"Join":             reflect.ValueOf(pluginJoin),
		"Replace":          reflect.ValueOf(pluginReplace),
		"Split":            reflect.ValueOf(pluginSplit),
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

func exportsForPlugin(owner string) interp.Exports {
	ex := make(interp.Exports)
	for pkg, symbols := range basePluginExports {
		m := map[string]reflect.Value{}
		for k, v := range symbols {
			m[k] = v
		}
		m["Equip"] = reflect.ValueOf(func(id uint16) { pluginEquip(owner, id) })
		m["Unequip"] = reflect.ValueOf(func(id uint16) { pluginUnequip(owner, id) })
		m["AddHotkey"] = reflect.ValueOf(func(combo, command string) { pluginAddHotkey(owner, combo, command) })
		m["AddHotkeyFn"] = reflect.ValueOf(func(combo string, handler func(HotkeyEvent)) { pluginAddHotkeyFn(owner, combo, handler) })
		m["RemoveHotkey"] = reflect.ValueOf(func(combo string) { pluginRemoveHotkey(owner, combo) })
		m["RegisterCommand"] = reflect.ValueOf(func(name string, handler PluginCommandHandler) {
			pluginRegisterCommand(owner, name, handler)
		})
		m["AddShortcut"] = reflect.ValueOf(func(short, full string) { pluginAddShortcut(owner, short, full) })
		m["AddShortcuts"] = reflect.ValueOf(func(shortcuts map[string]string) { pluginAddShortcuts(owner, shortcuts) })
		// Chat/Console (simple, no slices)
		// Simple DSL aliases
		m["Print"] = reflect.ValueOf(pluginConsole)
		m["Notify"] = reflect.ValueOf(pluginShowNotification)
		m["Cmd"] = reflect.ValueOf(func(text string) { pluginEnqueueCommand(owner, strings.TrimSpace(text)) })
		m["Run"] = reflect.ValueOf(func(text string) { pluginRunCommand(owner, strings.TrimSpace(text)) })
		m["Me"] = reflect.ValueOf(pluginPlayerName)
		m["Has"] = reflect.ValueOf(func(name string) bool { return pluginHasItem(name) })
		m["Save"] = reflect.ValueOf(func(key, value string) { pluginStorageSet(owner, key, value) })
		m["Load"] = reflect.ValueOf(func(key string) string {
			if v, ok := pluginStorageGet(owner, key).(string); ok {
				return v
			}
			return ""
		})
		m["Delete"] = reflect.ValueOf(func(key string) { pluginStorageDelete(owner, key) })
		m["Input"] = reflect.ValueOf(pluginInputText)
		m["SetInput"] = reflect.ValueOf(pluginSetInputText)
		// (Removed explicit Thank/Curse/Share/Unshare helpers to avoid duplicating
		// in-game commands; authors can use Cmd("/thank ...") etc.)
		// No-slice chat/console helpers (one call per phrase)
		m["Chat"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				pluginRegisterChat(owner, "", []string{p}, ChatAny, handler)
			}
		})
		m["PlayerChat"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				pluginRegisterChat(owner, "", []string{p}, ChatPlayer, handler)
			}
		})
		m["NPCChat"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				pluginRegisterChat(owner, "", []string{p}, ChatNPC, handler)
			}
		})
		m["CreatureChat"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				pluginRegisterChat(owner, "", []string{p}, ChatCreature, handler)
			}
		})
		m["SelfChat"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				pluginRegisterChat(owner, "", []string{p}, ChatSelf, handler)
			}
		})
		m["OtherChat"] = reflect.ValueOf(func(name, phrase string, handler func(string)) {
			n := strings.TrimSpace(name)
			p := strings.TrimSpace(phrase)
			if p != "" {
				pluginRegisterChat(owner, n, []string{p}, ChatOther, handler)
			}
		})
		m["ChatFrom"] = reflect.ValueOf(func(name, phrase string, handler func(string)) {
			n := strings.TrimSpace(name)
			p := strings.TrimSpace(phrase)
			if n != "" && p != "" {
				pluginRegisterChat(owner, n, []string{p}, ChatAny, handler)
			}
		})
		m["PlayerChatFrom"] = reflect.ValueOf(func(name, phrase string, handler func(string)) {
			n := strings.TrimSpace(name)
			p := strings.TrimSpace(phrase)
			if n != "" && p != "" {
				pluginRegisterChat(owner, n, []string{p}, ChatPlayer, handler)
			}
		})
		m["OtherChatFrom"] = reflect.ValueOf(func(name, phrase string, handler func(string)) {
			n := strings.TrimSpace(name)
			p := strings.TrimSpace(phrase)
			if n != "" && p != "" {
				pluginRegisterChat(owner, n, []string{p}, ChatOther, handler)
			}
		})
		m["ConsoleMsg"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				pluginRegisterConsole(owner, []string{p}, handler)
			}
		})
		// Sleep for game ticks (blocks current goroutine only)
		m["SleepTicks"] = reflect.ValueOf(func(ticks int) { pluginSleepTicks(owner, ticks) })
		// Simpler alias: Console("text", fn)
		m["Console"] = reflect.ValueOf(func(phrase string, handler func(string)) {
			p := strings.TrimSpace(phrase)
			if p != "" {
				pluginRegisterConsole(owner, []string{p}, handler)
			}
		})
		m["RegisterInputHandler"] = reflect.ValueOf(func(fn func(string) string) { pluginRegisterInputHandler(owner, fn) })
		m["RunCommand"] = reflect.ValueOf(func(cmd string) { pluginRunCommand(owner, cmd) })
		m["EnqueueCommand"] = reflect.ValueOf(func(cmd string) { pluginEnqueueCommand(owner, cmd) })
		m["StorageGet"] = reflect.ValueOf(func(key string) any { return pluginStorageGet(owner, key) })
		m["StorageSet"] = reflect.ValueOf(func(key string, value any) { pluginStorageSet(owner, key, value) })
		m["StorageDelete"] = reflect.ValueOf(func(key string) { pluginStorageDelete(owner, key) })

		// Timers
		m["After"] = reflect.ValueOf(func(ms int, fn func()) {
			if fn == nil || ms <= 0 {
				return
			}
			t := time.AfterFunc(time.Duration(ms)*time.Millisecond, fn)
			pluginMu.Lock()
			pluginTimers[owner] = append(pluginTimers[owner], t)
			pluginMu.Unlock()
		})
		m["AfterDur"] = reflect.ValueOf(func(d time.Duration, fn func()) {
			if fn == nil || d <= 0 {
				return
			}
			t := time.AfterFunc(d, fn)
			pluginMu.Lock()
			pluginTimers[owner] = append(pluginTimers[owner], t)
			pluginMu.Unlock()
		})
		m["Every"] = reflect.ValueOf(func(ms int, fn func()) {
			if fn == nil || ms <= 0 {
				return
			}
			stop := make(chan struct{})
			pluginMu.Lock()
			pluginTickerStops[owner] = append(pluginTickerStops[owner], stop)
			pluginMu.Unlock()
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
			pluginMu.Lock()
			pluginTickerStops[owner] = append(pluginTickerStops[owner], stop)
			pluginMu.Unlock()
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
			pluginRegisterCommand(owner, cmd, func(args string) { handler() })
			pluginAddHotkey(owner, c, "/"+cmd)
		})
		ex[pkg] = m
	}
	return ex
}

//go:embed scripts
var pluginScripts embed.FS

// userScriptsDir returns the preferred location for user-editable scripts.
// Scripts now live alongside the executable in a top-level "scripts" folder
// instead of under the data directory.
func userScriptsDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "scripts"
	}
	return filepath.Join(filepath.Dir(exe), "scripts")
}

// ensureScriptsDir creates the scripts directory next to the executable and
// populates it with the embedded scripts if it is missing.
func ensureScriptsDir() {
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
	entries, err := pluginScripts.ReadDir("scripts")
	if err != nil {
		log.Printf("read embedded scripts: %v", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := pluginScripts.ReadFile(path.Join("scripts", e.Name()))
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
	// Write example plugin files
	files := []string{
		"default_shortcuts.go",
		"README.txt",
		"numpad_poser.go",
	}
	for _, src := range files {
		sPath := path.Join("scripts", src)
		data, err := pluginScripts.ReadFile(sPath)
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

var pluginAllowedPkgs = []string{
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

const pluginGoroutineLimit = 256

func init() {
	go pluginGoroutineWatchdog()
}

func pluginGoroutineWatchdog() {
	for {
		if runtime.NumGoroutine() > pluginGoroutineLimit {
			log.Printf("[plugin] goroutine limit exceeded; stopping all plugins")
			consoleMessage("[plugin] goroutine limit exceeded; stopping plugins")
			stopAllPlugins()
			return
		}
		time.Sleep(time.Millisecond * 100)
	}
}

func restrictedStdlib() interp.Exports {
	restricted := interp.Exports{}
	for _, key := range pluginAllowedPkgs {
		if syms, ok := stdlib.Symbols[key]; ok {
			restricted[key] = syms
		}
	}
	return restricted
}

func pluginConsole(msg string) {
	if gs.pluginOutputDebug {
		consoleMessage(msg)
	}
}

func pluginShowNotification(msg string) {
	showNotification(msg)
}

func pluginIsDisabled(owner string) bool {
	pluginMu.RLock()
	disabled := pluginDisabled[owner]
	pluginMu.RUnlock()
	return disabled
}

func pluginAddHotkey(owner, combo, command string) {
	if pluginIsDisabled(owner) {
		return
	}
	hk := Hotkey{Name: command, Combo: combo, Commands: []HotkeyCommand{{Command: command}}, Plugin: owner, Disabled: true}
	pluginHotkeyMu.RLock()
	if m := pluginHotkeyEnabled[owner]; m != nil {
		if m[combo] {
			hk.Disabled = false
		}
	}
	pluginHotkeyMu.RUnlock()
	hotkeysMu.Lock()
	for _, existing := range hotkeys {
		if existing.Plugin == owner && existing.Combo == combo {
			hotkeysMu.Unlock()
			return
		}
	}
	hotkeys = append(hotkeys, hk)
	hotkeysMu.Unlock()
	pluginHotkeyMu.Lock()
	if hk.Disabled {
		if m := pluginHotkeyEnabled[owner]; m != nil {
			delete(m, combo)
			if len(m) == 0 {
				delete(pluginHotkeyEnabled, owner)
			}
		}
	} else {
		m := pluginHotkeyEnabled[owner]
		if m == nil {
			m = map[string]bool{}
			pluginHotkeyEnabled[owner] = m
		}
		m[combo] = true
	}
	pluginHotkeyMu.Unlock()
	refreshHotkeysList()
	saveHotkeys()
	name := pluginDisplayNames[owner]
	if name == "" {
		name = owner
	}
	msg := fmt.Sprintf("[plugin:%s] hotkey added: %s -> %s", name, combo, command)
	if gs.pluginOutputDebug {
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
	pluginHotkeyFnMu sync.RWMutex
	pluginHotkeyFns  = map[string]map[string]func(HotkeyEvent){}
)

// pluginAddHotkeyFn registers a function-based hotkey for a plugin.
// The hotkey appears in the "Plugin Hotkeys" list and can be enabled/disabled
// like command-based hotkeys, but when pressed it will call the provided
// handler instead of emitting a slash command.
func pluginAddHotkeyFn(owner, combo string, handler func(HotkeyEvent)) {
	if pluginIsDisabled(owner) || handler == nil {
		return
	}
	// Remember handler
	pluginHotkeyFnMu.Lock()
	m := pluginHotkeyFns[owner]
	if m == nil {
		m = map[string]func(HotkeyEvent){}
		pluginHotkeyFns[owner] = m
	}
	m[combo] = handler
	pluginHotkeyFnMu.Unlock()

	// Ensure a visible toggleable hotkey entry exists for this plugin+combo.
	hk := Hotkey{Name: "", Combo: combo, Plugin: owner, Disabled: true}
	pluginHotkeyMu.RLock()
	if m := pluginHotkeyEnabled[owner]; m != nil {
		if m[combo] {
			hk.Disabled = false
		}
	}
	pluginHotkeyMu.RUnlock()
	hotkeysMu.Lock()
	for _, existing := range hotkeys {
		if existing.Plugin == owner && existing.Combo == combo {
			hotkeysMu.Unlock()
			refreshHotkeysList()
			saveHotkeys()
			return
		}
	}
	hotkeys = append(hotkeys, hk)
	hotkeysMu.Unlock()
	pluginHotkeyMu.Lock()
	if hk.Disabled {
		if m := pluginHotkeyEnabled[owner]; m != nil {
			delete(m, combo)
			if len(m) == 0 {
				delete(pluginHotkeyEnabled, owner)
			}
		}
	} else {
		m := pluginHotkeyEnabled[owner]
		if m == nil {
			m = map[string]bool{}
			pluginHotkeyEnabled[owner] = m
		}
		m[combo] = true
	}
	pluginHotkeyMu.Unlock()
	refreshHotkeysList()
	saveHotkeys()
	name := pluginDisplayNames[owner]
	if name == "" {
		name = owner
	}
	msg := fmt.Sprintf("[plugin:%s] hotkey added: %s -> <function>", name, combo)
	if gs.pluginOutputDebug {
		consoleMessage(msg)
	}
	log.Print(msg)
}

func pluginGetHotkeyFn(owner, combo string) (func(HotkeyEvent), bool) {
	pluginHotkeyFnMu.RLock()
	defer pluginHotkeyFnMu.RUnlock()
	if m := pluginHotkeyFns[owner]; m != nil {
		if fn := m[combo]; fn != nil {
			return fn, true
		}
	}
	return nil, false
}

// Plugin command registries.
type PluginCommandHandler func(args string)

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

var (
	pluginCommands        = map[string]PluginCommandHandler{}
	pluginMu              sync.RWMutex
	pluginNames           = map[string]bool{}
	pluginDisplayNames    = map[string]string{}
	pluginAuthors         = map[string]string{}
	pluginCategories      = map[string]string{}
	pluginSubCategories   = map[string]string{}
	pluginInvalid         = map[string]bool{}
	pluginDisabled        = map[string]bool{}
	pluginEnabledFor      = map[string]pluginScope{}
	pluginPaths           = map[string]string{}
	pluginTerminators     = map[string]func(){}
	pluginTriggers        = map[string][]triggerHandler{}
	pluginConsoleTriggers = map[string][]triggerHandler{}
	triggerHandlersMu     sync.RWMutex
	pluginInputHandlers   []inputHandler
	inputHandlersMu       sync.RWMutex
	pluginCommandOwners   = map[string]string{}
	pluginSendHistory     = map[string][]time.Time{}
	pluginModTime         time.Time
	pluginModCheck        time.Time
	// timers per plugin owner
	pluginTimers      = map[string][]*time.Timer{}
	pluginTickerStops = map[string][]chan struct{}{}
	pluginTickWaiters = map[string][]*tickWaiter{}
)

type tickWaiter struct {
	remain int
	done   chan struct{}
}

func pluginSleepTicks(owner string, ticks int) {
	if ticks <= 0 {
		return
	}
	w := &tickWaiter{remain: ticks, done: make(chan struct{}, 1)}
	pluginMu.Lock()
	pluginTickWaiters[owner] = append(pluginTickWaiters[owner], w)
	pluginMu.Unlock()
	<-w.done
}

func pluginAdvanceTick() {
	pluginMu.Lock()
	for owner, list := range pluginTickWaiters {
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
			delete(pluginTickWaiters, owner)
		} else {
			pluginTickWaiters[owner] = list[:n]
		}
	}
	pluginMu.Unlock()
}

const (
	minPluginMetaLen = 2
	maxPluginMetaLen = 40
)

func invalidPluginValue(s string) bool {
	l := len(s)
	return l < minPluginMetaLen || l > maxPluginMetaLen
}

// pluginRegisterCommand lets plugins handle a local slash command like
// "/example". The name should be without the leading slash and will be
// matched case-insensitively.
func pluginRegisterCommand(owner, name string, handler PluginCommandHandler) {
	if name == "" || handler == nil {
		return
	}
	if pluginIsDisabled(owner) {
		return
	}
	key := strings.ToLower(strings.TrimPrefix(name, "/"))
	pluginMu.Lock()
	if _, exists := pluginCommands[key]; exists {
		pluginMu.Unlock()
		msg := fmt.Sprintf("[plugin] command conflict: /%s already registered", key)
		consoleMessage(msg)
		log.Print(msg)
		return
	}
	pluginCommands[key] = handler
	pluginCommandOwners[key] = owner
	pluginMu.Unlock()
	consoleMessage("[plugin] command registered: /" + key)
	log.Printf("[plugin] command registered: /%s", key)
}

// pluginRunCommand echoes and enqueues a command for immediate sending.
func pluginRunCommand(owner, cmd string) {
	if pluginIsDisabled(owner) {
		return
	}
	if recordPluginSend(owner) {
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

// pluginEnqueueCommand enqueues a command to be sent on the next tick without echoing.
func pluginEnqueueCommand(owner, cmd string) {
	if pluginIsDisabled(owner) {
		return
	}
	if recordPluginSend(owner) {
		return
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	enqueueCommand(cmd)
}

func loadPluginSource(owner, name, path string, src []byte, restricted interp.Exports) {
	i := interp.New(interp.Options{})
	if len(restricted) > 0 {
		i.Use(restricted)
	}
	i.Use(exportsForPlugin(owner))
	pluginMu.Lock()
	pluginDisabled[owner] = false
	pluginMu.Unlock()
	if _, err := i.Eval(string(src)); err != nil {
		log.Printf("plugin %s: %v", path, err)
		consoleMessage("[plugin] load error for " + path + ": " + err.Error())
		disablePlugin(owner, "load error")
		return
	}
	if v, err := i.Eval("Terminate"); err == nil {
		if fn, ok := v.Interface().(func()); ok {
			pluginMu.Lock()
			pluginTerminators[owner] = fn
			pluginMu.Unlock()
		}
	}
	if v, err := i.Eval("Init"); err == nil {
		if fn, ok := v.Interface().(func()); ok {
			go fn()
		}
	}
	log.Printf("loaded plugin %s", path)
	consoleMessage("[plugin] loaded: " + name)
}

func enablePlugin(owner string) {
	pluginMu.RLock()
	path := pluginPaths[owner]
	name := pluginDisplayNames[owner]
	pluginMu.RUnlock()
	if path == "" {
		return
	}
	src, err := os.ReadFile(path)
	if err != nil {
		log.Printf("read plugin %s: %v", path, err)
		consoleMessage("[plugin] read error for " + path + ": " + err.Error())
		return
	}
	loadPluginSource(owner, name, path, src, restrictedStdlib())
	settingsDirty = true
	saveSettings()
	refreshPluginsWindow()
}

func recordPluginSend(owner string) bool {
	if !gs.PluginSpamKill {
		return false
	}
	now := time.Now()
	cutoff := now.Add(-5 * time.Second)
	pluginMu.Lock()
	times := pluginSendHistory[owner]
	n := 0
	for _, t := range times {
		if t.After(cutoff) {
			times[n] = t
			n++
		}
	}
	times = times[:n]
	times = append(times, now)
	pluginSendHistory[owner] = times
	count := len(times)
	pluginMu.Unlock()
	if count > 30 {
		disablePlugin(owner, "sent too many lines")
		return true
	}
	return false
}

func disablePlugin(owner, reason string) {
	pluginMu.Lock()
	pluginDisabled[owner] = true
	if reason != "disabled for this character" && reason != "reloaded" {
		delete(pluginEnabledFor, owner)
	}
	term := pluginTerminators[owner]
	delete(pluginTerminators, owner)
	pluginMu.Unlock()
	if term != nil {
		go term()
	}
	for _, hk := range pluginHotkeys(owner) {
		pluginRemoveHotkey(owner, hk.Combo)
	}
	pluginRemoveShortcuts(owner)
	inputHandlersMu.Lock()
	for i := len(pluginInputHandlers) - 1; i >= 0; i-- {
		if pluginInputHandlers[i].owner == owner {
			pluginInputHandlers = append(pluginInputHandlers[:i], pluginInputHandlers[i+1:]...)
		}
	}
	inputHandlersMu.Unlock()
	triggerHandlersMu.Lock()
	for phrase, hs := range pluginTriggers {
		n := 0
		for _, h := range hs {
			if h.owner != owner {
				hs[n] = h
				n++
			}
		}
		if n == 0 {
			delete(pluginTriggers, phrase)
		} else {
			pluginTriggers[phrase] = hs[:n]
		}
	}
	for phrase, hs := range pluginConsoleTriggers {
		n := 0
		for _, h := range hs {
			if h.owner != owner {
				hs[n] = h
				n++
			}
		}
		if n == 0 {
			delete(pluginConsoleTriggers, phrase)
		} else {
			pluginConsoleTriggers[phrase] = hs[:n]
		}
	}
	triggerHandlersMu.Unlock()
	// Remove function hotkeys
	pluginHotkeyFnMu.Lock()
	delete(pluginHotkeyFns, owner)
	pluginHotkeyFnMu.Unlock()
	refreshTriggersList()
	playerHandlersMu.Lock()
	for i := len(pluginPlayerHandlers) - 1; i >= 0; i-- {
		if pluginPlayerHandlers[i].owner == owner {
			pluginPlayerHandlers = append(pluginPlayerHandlers[:i], pluginPlayerHandlers[i+1:]...)
		}
	}
	playerHandlersMu.Unlock()
	// Stop any timers/tickers and tick waiters for this plugin
	pluginMu.Lock()
	if list := pluginTimers[owner]; len(list) > 0 {
		for _, t := range list {
			if t != nil {
				t.Stop()
			}
		}
		delete(pluginTimers, owner)
	}
	if stops := pluginTickerStops[owner]; len(stops) > 0 {
		for _, ch := range stops {
			if ch != nil {
				close(ch)
			}
		}
		delete(pluginTickerStops, owner)
	}
	if waits := pluginTickWaiters[owner]; len(waits) > 0 {
		for _, w := range waits {
			if w != nil {
				select {
				case w.done <- struct{}{}:
				default:
				}
			}
		}
		delete(pluginTickWaiters, owner)
	}
	pluginMu.Unlock()
	pluginMu.Lock()
	for cmd, o := range pluginCommandOwners {
		if o == owner {
			delete(pluginCommands, cmd)
			delete(pluginCommandOwners, cmd)
		}
	}
	delete(pluginSendHistory, owner)
	disp := pluginDisplayNames[owner]
	pluginMu.Unlock()
	if disp == "" {
		disp = owner
	}
	consoleMessage("[plugin:" + disp + "] stopped: " + reason)
	settingsDirty = true
	saveSettings()
	refreshPluginsWindow()
}

func stopAllPlugins() {
	pluginMu.RLock()
	owners := make([]string, 0, len(pluginDisplayNames))
	for o := range pluginDisplayNames {
		if !pluginDisabled[o] {
			owners = append(owners, o)
		}
	}
	pluginMu.RUnlock()
	for _, o := range owners {
		disablePlugin(o, "stopped by user")
	}
	if len(owners) > 0 {
		commandQueue = nil
		pendingCommand = ""
		consoleMessage("[plugin] all plugins stopped")
	}
}

func applyEnabledPlugins() {
	pluginMu.RLock()
	owners := make([]string, 0, len(pluginDisplayNames))
	for o := range pluginDisplayNames {
		owners = append(owners, o)
	}
	pluginMu.RUnlock()
	for _, o := range owners {
		pluginMu.RLock()
		scope := pluginEnabledFor[o]
		disabled := pluginDisabled[o]
		invalid := pluginInvalid[o]
		pluginMu.RUnlock()
		if invalid {
			pluginMu.Lock()
			pluginDisabled[o] = true
			pluginMu.Unlock()
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
			enablePlugin(o)
		} else if !disabled && !shouldEnable {
			disablePlugin(o, "disabled for this character")
		} else {
			pluginMu.Lock()
			pluginDisabled[o] = !shouldEnable
			pluginMu.Unlock()
		}
	}
}

func setPluginEnabled(owner string, char, all bool) {
	pluginMu.Lock()
	if pluginInvalid[owner] {
		pluginMu.Unlock()
		return
	}
	s := pluginEnabledFor[owner]
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
			s = pluginScope{}
		}
	}
	if s.empty() {
		delete(pluginEnabledFor, owner)
	} else {
		pluginEnabledFor[owner] = s
	}
	pluginMu.Unlock()
	applyEnabledPlugins()
	saveSettings()
	refreshPluginsWindow()
}

// clearPluginScope removes all enablement for a plugin (no all, no characters)
// and refreshes apply/save/UI. Used by the UI when unchecking the "All" box
// to explicitly stop a plugin regardless of any per-character flags.
func clearPluginScope(owner string) {
	pluginMu.Lock()
	delete(pluginEnabledFor, owner)
	pluginMu.Unlock()
	// Stop scheduled timers/tickers for this plugin
	if list := pluginTimers[owner]; len(list) > 0 {
		for _, t := range list {
			if t != nil {
				t.Stop()
			}
		}
		delete(pluginTimers, owner)
	}
	if stops := pluginTickerStops[owner]; len(stops) > 0 {
		for _, ch := range stops {
			if ch != nil {
				close(ch)
			}
		}
		delete(pluginTickerStops, owner)
	}
	applyEnabledPlugins()
	saveSettings()
	refreshPluginsWindow()
}

func pluginPlayerName() string {
	return playerName
}

func pluginPlayers() []Player {
	ps := getPlayers()
	out := make([]Player, len(ps))
	copy(out, ps)
	return out
}

func pluginInventory() []InventoryItem {
	return getInventory()
}

func pluginToggleEquip(owner string, id uint16) {
	if recordPluginSend(owner) {
		return
	}
	toggleInventoryEquip(id)
}

type Stats struct {
	HP, HPMax           int
	SP, SPMax           int
	Balance, BalanceMax int
}

func pluginPlayerStats() Stats {
	stateMu.Lock()
	s := Stats{
		HP:         state.hp,
		HPMax:      state.hpMax,
		SP:         state.sp,
		SPMax:      state.spMax,
		Balance:    state.balance,
		BalanceMax: state.balanceMax,
	}
	stateMu.Unlock()
	return s
}

func pluginInputText() string {
	inputMu.Lock()
	txt := string(inputText)
	inputMu.Unlock()
	return txt
}

func pluginSetInputText(text string) {
	inputMu.Lock()
	inputText = []rune(text)
	inputActive = true
	inputPos = len(inputText)
	inputMu.Unlock()
}

func pluginEquip(owner string, id uint16) {
	if recordPluginSend(owner) {
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

func pluginUnequip(owner string, id uint16) {
	if recordPluginSend(owner) {
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

func pluginRegisterInputHandler(owner string, fn func(string) string) {
	if pluginIsDisabled(owner) || fn == nil {
		return
	}
	inputHandlersMu.Lock()
	pluginInputHandlers = append(pluginInputHandlers, inputHandler{owner: owner, fn: fn})
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

// pluginRegisterChat registers a chat trigger with optional name and kind flags.
func pluginRegisterChat(owner, name string, phrases []string, flags int, fn func(string)) {
	if pluginIsDisabled(owner) || fn == nil {
		return
	}
	triggerHandlersMu.Lock()
	name = strings.ToLower(name)
	for _, p := range phrases {
		if p == "" {
			continue
		}
		p = strings.ToLower(p)
		pluginTriggers[p] = append(pluginTriggers[p], triggerHandler{owner: owner, name: name, flags: flags, fn: fn})
	}
	triggerHandlersMu.Unlock()
	refreshTriggersList()
}

// Back-compat wrapper for older API without flags.
func pluginRegisterTriggers(owner, name string, phrases []string, fn func()) {
	if fn == nil {
		return
	}
	pluginRegisterChat(owner, name, phrases, ChatAny, func(string) { fn() })
}

// New console registration with message parameter
func pluginRegisterConsole(owner string, phrases []string, fn func(string)) {
	if pluginIsDisabled(owner) || fn == nil {
		return
	}
	triggerHandlersMu.Lock()
	for _, p := range phrases {
		if p == "" {
			continue
		}
		p = strings.ToLower(p)
		pluginConsoleTriggers[p] = append(pluginConsoleTriggers[p], triggerHandler{owner: owner, fn: fn})
	}
	triggerHandlersMu.Unlock()
	refreshTriggersList()
}

// Back-compat: old console registration without msg parameter
func pluginRegisterConsoleTriggers(owner string, phrases []string, fn func()) {
	if fn == nil {
		return
	}
	pluginRegisterConsole(owner, phrases, func(string) { fn() })
}

// pluginAutoReply sends a command when a chat message contains trigger.
func pluginAutoReply(owner, trigger, command string) {
	if pluginIsDisabled(owner) || trigger == "" || command == "" {
		return
	}
	pluginRegisterTriggers(owner, "", []string{trigger}, func() {
		pluginEnqueueCommand(owner, command)
	})
}

func pluginRegisterTrigger(owner string, phrase string, fn func(string)) {
	if pluginIsDisabled(owner) || fn == nil {
		return
	}
	if len(phrase) < 2 {
		return
	}
	triggerHandlersMu.Lock()
	phrase = strings.ToLower(phrase)
	pluginTriggers[phrase] = append(pluginTriggers[phrase], triggerHandler{owner: owner, fn: fn})
	triggerHandlersMu.Unlock()
}

func pluginRegisterPlayerHandler(owner string, fn func(Player)) {
	if pluginIsDisabled(owner) || fn == nil {
		return
	}
	playerHandlersMu.Lock()
	pluginPlayerHandlers = append(pluginPlayerHandlers, playerHandler{owner: owner, fn: fn})
	playerHandlersMu.Unlock()
}

func runInputHandlers(txt string) string {
	inputHandlersMu.RLock()
	handlers := make([]func(string) string, len(pluginInputHandlers))
	for i, h := range pluginInputHandlers {
		handlers[i] = h.fn
	}
	inputHandlersMu.RUnlock()
	for _, h := range handlers {
		if h != nil {
			txt = h(txt)
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
		playersMu.RLock()
		if p, ok := players[speaker]; ok {
			if p.IsNPC {
				msgFlags |= ChatNPC
			} else {
				msgFlags |= ChatPlayer
			}
		} else {
			msgFlags |= ChatCreature
		}
		playersMu.RUnlock()
	} else {
		msgFlags |= ChatCreature
	}
	for phrase, hs := range pluginTriggers {
		if strings.Contains(strings.ToLower(msg), phrase) {
			for _, h := range hs {
				// Name filter matches literal name if set.
				if h.name != "" && h.name != strings.ToLower(speaker) {
					continue
				}
				// Flag filter: default to ChatAny, then require intersection.
				f := h.flags
				if f == 0 {
					f = ChatAny
				}
				match := (f & msgFlags) != 0
				if match {
					fn := h.fn
					go fn(msg)
				}
			}
		}
	}
	triggerHandlersMu.RUnlock()
}

func runConsoleTriggers(msg string) {
	triggerHandlersMu.RLock()
	msgLower := strings.ToLower(msg)
	for phrase, hs := range pluginConsoleTriggers {
		if strings.Contains(msgLower, phrase) {
			for _, h := range hs {
				fn := h.fn
				go fn(msg)
			}
		}
	}
	triggerHandlersMu.RUnlock()
}

func pluginPlaySound(ids []uint16) {
	playSound(ids)
}

func pluginCommandsFor(owner string) []string {
	pluginMu.RLock()
	defer pluginMu.RUnlock()
	var list []string
	for cmd, o := range pluginCommandOwners {
		if o == owner {
			list = append(list, cmd)
		}
	}
	return list
}

func pluginSource(owner string) string {
	pluginMu.RLock()
	path := pluginPaths[owner]
	pluginMu.RUnlock()
	if path == "" {
		return "plugin source not found"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("error reading %s: %v", path, err)
	}
	return string(data)
}

func refreshPluginMod() {
	dirs := []string{userScriptsDir(), "scripts"}
	latest := time.Time{}
	for _, dir := range dirs {
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
	pluginModTime = latest
}

type pluginInfo struct {
	name        string
	author      string
	category    string
	subCategory string
	path        string
	src         []byte
	invalid     bool
	apiVer      int
}

func scanPlugins(pluginDirs []string, dup func(name, path string)) map[string]pluginInfo {
	nameRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+PluginName\s*=\s*"([^"]+)"`)
	authorRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+PluginAuthor\s*=\s*"([^"]+)"`)
	categoryRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+PluginCategory\s*=\s*"([^"]+)"`)
	subCategoryRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+PluginSubCategory\s*=\s*"([^"]+)"`)
	apiVerRE := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+PluginAPIVersion\s*=\s*([0-9]+)\s*$`)
	plugins := map[string]pluginInfo{}
	seenNames := map[string]bool{}
	for _, dir := range pluginDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Printf("read plugin dir %s: %v", dir, err)
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
				log.Printf("read plugin %s: %v", path, err)
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
			if len(nameMatch) < 2 || name == "" || invalidPluginValue(name) {
				if len(nameMatch) < 2 || name == "" {
					consoleMessage("[plugin] missing name: " + path)
					name = base
				} else {
					consoleMessage("[plugin] invalid name: " + path)
				}
				invalid = true
			}
			if author == "" || invalidPluginValue(author) {
				if author == "" {
					consoleMessage("[plugin] missing author: " + path)
				} else {
					consoleMessage("[plugin] invalid author: " + path)
				}
				invalid = true
			}
			if category == "" || invalidPluginValue(category) {
				if category == "" {
					consoleMessage("[plugin] missing category: " + path)
				} else {
					consoleMessage("[plugin] invalid category: " + path)
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
			plugins[owner] = pluginInfo{
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
	return plugins
}

func rescanPlugins() {
	pluginDirs := []string{userScriptsDir(), "scripts"}
	scanned := scanPlugins(pluginDirs, nil)

	pluginMu.RLock()
	oldDisabled := make(map[string]bool, len(pluginDisabled))
	for o, d := range pluginDisabled {
		oldDisabled[o] = d
	}
	oldOwners := make(map[string]struct{}, len(pluginDisplayNames))
	for o := range pluginDisplayNames {
		oldOwners[o] = struct{}{}
	}
	pluginMu.RUnlock()

	for o := range oldOwners {
		if _, ok := scanned[o]; !ok {
			disablePlugin(o, "removed")
		}
	}

	pluginMu.Lock()
	pluginDisplayNames = make(map[string]string, len(scanned))
	pluginPaths = make(map[string]string, len(scanned))
	pluginAuthors = make(map[string]string, len(scanned))
	pluginCategories = make(map[string]string, len(scanned))
	pluginSubCategories = make(map[string]string, len(scanned))
	pluginInvalid = make(map[string]bool, len(scanned))
	pluginDisabled = make(map[string]bool, len(scanned))
	newEnabled := map[string]pluginScope{}
	for o, info := range scanned {
		pluginDisplayNames[o] = info.name
		pluginPaths[o] = info.path
		pluginAuthors[o] = info.author
		pluginCategories[o] = info.category
		pluginSubCategories[o] = info.subCategory
		// Require a matching plugin API version
		invalid := info.invalid || info.apiVer != pluginAPICurrentVersion
		pluginInvalid[o] = invalid
		if invalid {
			pluginDisabled[o] = true
			continue
		}
		if en, ok := pluginEnabledFor[o]; ok {
			newEnabled[o] = en
		} else if gs.EnabledPlugins != nil {
			if val, ok := gs.EnabledPlugins[o]; ok {
				newEnabled[o] = scopeFromSettingValue(val)
			}
		}
		effChar := playerName
		if effChar == "" {
			effChar = gs.LastCharacter
		}
		pluginDisabled[o] = !newEnabled[o].enablesFor(effChar)
	}
	pluginEnabledFor = newEnabled
	pluginNames = make(map[string]bool, len(scanned))
	for _, info := range scanned {
		pluginNames[strings.ToLower(info.name)] = true
	}
	pluginMu.Unlock()

	applyEnabledPlugins()
	refreshPluginsWindow()
	settingsDirty = true
}

func checkPluginMods() {
	if time.Since(pluginModCheck) < 500*time.Millisecond {
		return
	}
	pluginModCheck = time.Now()
	old := pluginModTime
	refreshPluginMod()
	if pluginModTime.After(old) {
		rescanPlugins()
	}
}

func loadPlugins() {
	ensureScriptsDir()
	ensureDefaultScripts()

	pluginDirs := []string{userScriptsDir(), "scripts"}
	scanned := scanPlugins(pluginDirs, func(name, path string) {
		log.Printf("plugin %s duplicate name %s", path, name)
		consoleMessage("[plugin] duplicate name: " + name)
	})

	pluginNames = make(map[string]bool, len(scanned))
	for o, info := range scanned {
		pluginNames[strings.ToLower(info.name)] = true
		s, ok := pluginEnabledFor[o]
		if !ok && gs.EnabledPlugins != nil {
			if val, ok2 := gs.EnabledPlugins[o]; ok2 {
				s = scopeFromSettingValue(val)
			}
		}
		effChar := playerName
		if effChar == "" {
			effChar = gs.LastCharacter
		}
		invalid := info.invalid || info.apiVer != pluginAPICurrentVersion
		disabled := invalid || !s.enablesFor(effChar)
		pluginMu.Lock()
		pluginDisplayNames[o] = info.name
		pluginCategories[o] = info.category
		pluginSubCategories[o] = info.subCategory
		pluginPaths[o] = info.path
		if !s.empty() {
			pluginEnabledFor[o] = s
		}
		pluginAuthors[o] = info.author
		pluginInvalid[o] = invalid
		pluginDisabled[o] = disabled
		pluginMu.Unlock()
		if !disabled {
			loadPluginSource(o, info.name, info.path, info.src, restrictedStdlib())
		}
	}
	hotkeysMu.Lock()
	for i := range hotkeys {
		if hotkeys[i].Plugin != "" {
			hotkeys[i].Disabled = true
		}
	}
	hotkeysMu.Unlock()
	refreshHotkeysList()
	refreshPluginsWindow()
	refreshPluginMod()
}

// scopeFromSettingValue converts a settings value into a pluginScope.
// Accepted values:
// - string("all"): All=true
// - string(name): include that character
// - []string: include all listed characters
// - []any (from JSON): include all listed string characters
// - bool(true): include LastCharacter if present
func scopeFromSettingValue(v any) pluginScope {
	s := pluginScope{}
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
