// Package gt provides an editor-only stub of the goThoom script API.
//
// Scripts are plain .go files interpreted at runtime via Yaegi. At
// compile-time your editor and static analysis tools still need symbols
// to type-check imports like `import "gt"`. This package defines those
// symbols with no-op implementations and brief docs so you can browse
// available functions in your IDE. The real implementations are injected
// into the Yaegi interpreter by the client at runtime.
//
// Notes
//   - These stubs are never called by the compiled client binary.
//   - Keep names and signatures in sync with the runtime export table in
//     plugin.go (search for exportsForPlugin).
//   - This package is intentionally tiny: it has zero dependencies and
//     returns zero values.
package gt

import "time"

// ClientVersion mirrors the client version value exported to plugins.
var ClientVersion int

// Console writes a message to the in-client console.
func Console(msg string) {}

// Print writes a message to the in-client console (alias).
func Print(msg string) {}

// ShowNotification displays a notification bubble.
func ShowNotification(msg string) {}

// Notify displays a notification bubble (alias).
func Notify(msg string) {}

// AddHotkey binds a key combo to a slash command.
func AddHotkey(combo, command string) {}

// HotkeyCommand mirrors the command bound to a hotkey.
type HotkeyCommand struct {
	Command string
}

// Hotkey represents a single key binding and its metadata.
type Hotkey struct {
	Name     string
	Combo    string
	Commands []HotkeyCommand
	Plugin   string
	Disabled bool
}

// HotkeyEvent describes which key(s) or mouse button triggered a plugin hotkey.
// Combo is the full recorded combo string (e.g., "Ctrl-Shift-D" or "RightClick").
// Parts are the components split by '-', and Trigger is usually the last part.
type HotkeyEvent struct {
	Combo   string
	Parts   []string
	Trigger string
}

// Hotkeys returns the plugin's registered hotkeys.
func Hotkeys() []Hotkey { return nil }

// RemoveHotkey removes a plugin-owned hotkey by combo.
func RemoveHotkey(combo string) {}

// RegisterCommand handles a local slash command like "/example".
func RegisterCommand(command string, handler func(args string)) {}

// RunCommand queues a command to send immediately to the server.
func RunCommand(cmd string) {}

// EnqueueCommand queues a command for the next tick without echoing.
func EnqueueCommand(cmd string) {}

// AddHotkeyFn binds a key combo to a function handler.
// The handler receives which key(s)/button triggered it via HotkeyEvent.
func AddHotkeyFn(combo string, handler func(HotkeyEvent)) {}

// IgnoreCase reports whether a and b are equal ignoring capitalization.
func IgnoreCase(a, b string) bool { return false }

// StartsWith reports whether text begins with prefix.
func StartsWith(text, prefix string) bool { return false }

// EndsWith reports whether text ends with suffix.
func EndsWith(text, suffix string) bool { return false }

// Includes reports whether text contains substr.
func Includes(text, substr string) bool { return false }

// Lower returns text in lower case.
func Lower(text string) string { return "" }

// Upper returns text in upper case.
func Upper(text string) string { return "" }

// Trim removes spaces at the start and end of text.
func Trim(text string) string { return "" }

// TrimStart removes prefix from text if present.
func TrimStart(text, prefix string) string { return "" }

// TrimEnd removes suffix from text if present.
func TrimEnd(text, suffix string) string { return "" }

// Words splits text into fields separated by spaces.
func Words(text string) []string { return nil }

// Join concatenates parts with sep between elements.
func Join(parts []string, sep string) string { return "" }

// Replace returns a copy of s with the first n non-overlapping instances of
// old replaced by new. If n < 0, all instances are replaced.
func Replace(s, old, new string, n int) string { return "" }

// Split slices s into all substrings separated by sep and returns a slice.
func Split(s, sep string) []string { return nil }

// AddShortcut replaces a short prefix with a full command in the chat box.
// When you type the short text at the start of the input bar it is expanded
// to the full command (with arguments preserved after a space).
func AddShortcut(short, full string) {}

// AddShortcuts registers multiple shortcuts at once.
func AddShortcuts(shortcuts map[string]string) {}

// PlayerName returns the current player's name.
func PlayerName() string { return "" }

// Player mirrors the player's state exposed to plugins.
type Player struct {
	Name       string
	Race       string
	Gender     string
	Class      string
	PictID     uint16
	Colors     []byte
	IsNPC      bool
	Sharee     bool
	Sharing    bool
	Friend     bool
	Blocked    bool
	Ignored    bool
	Dead       bool
	FellWhere  string
	FellTime   time.Time
	KillerName string
	Bard       bool
	SameClan   bool
	Seen       bool
	LastSeen   time.Time
	Offline    bool
}

// Players returns the list of known players.
func Players() []Player { return nil }

// RegisterTriggers registers a callback for chat messages containing any phrase
// from the specified player name. An empty name matches any speaker.
// The handler receives the full message text.
func RegisterTriggers(name string, phrases []string, fn func(msg string)) {}

// Chat helpers (one phrase per call)
func Chat(phrase string, fn func(msg string))                 {}
func PlayerChat(phrase string, fn func(msg string))           {}
func NPCChat(phrase string, fn func(msg string))              {}
func CreatureChat(phrase string, fn func(msg string))         {}
func SelfChat(phrase string, fn func(msg string))             {}
func OtherChat(name, phrase string, fn func(msg string))      {}
func ChatFrom(name, phrase string, fn func(msg string))       {}
func PlayerChatFrom(name, phrase string, fn func(msg string)) {}
func OtherChatFrom(name, phrase string, fn func(msg string))  {}

// Timers: by milliseconds or by duration
func After(ms int, fn func())             {}
func Every(ms int, fn func())             {}
func AfterDur(d time.Duration, fn func()) {}
func EveryDur(d time.Duration, fn func()) {}

// Key binds a key combo to a handler function. The combo string should use
// names like "Ctrl-Shift-D" or "RightClick"; see the example README for a
// complete list.
func Key(combo string, handler func()) {}

// SleepTicks blocks the current handler goroutine for a number of server frames.
func SleepTicks(ticks int) {}

// ConsoleMsg is an alias of Console.
func ConsoleMsg(phrase string, fn func(msg string)) {}

// Minimal DSL helpers
func Cmd(text string)        {}
func Run(text string)        {}
func Me() string             { return "" }
func Has(name string) bool   { return false }
func Toggle(id uint16)       {}
func Save(key, value string) {}
func Load(key string) string { return "" }
func Delete(key string)      {}
func Input() string          { return "" }
func SetInput(text string)   {}

// (Intentionally no helpers that duplicate in-game slash commands
// like Thank/Curse/Share/Unshare; use Cmd("/...") in scripts.)

// RegisterConsoleTriggers registers a callback for console messages containing
// any phrase.
func RegisterConsoleTriggers(phrases []string, fn func()) {}

// RegisterTriggers registers a callback for messages containing any phrase.
func RegisterTrigger(name string, phrase string, fn func()) {}

// RegisterInputHandler registers a callback to modify input text before sending.
func RegisterInputHandler(fn func(text string) string) {}

// RegisterPlayerHandler registers a callback for player info updates.
func RegisterPlayerHandler(fn func(Player)) {}

// InventoryItem mirrors the client's inventory item structure.
type InventoryItem struct {
	ID       uint16
	Name     string
	Base     string
	Extra    string
	Equipped bool
	Index    int
	IDIndex  int
	Quantity int
}

// Inventory returns the player's inventory.
func Inventory() []InventoryItem { return nil }

// InputText returns the current text in the input bar.
func InputText() string { return "" }

// SetInputText replaces the text in the input bar.
func SetInputText(text string) {}

// Equip equips the specified item by ID if it isn't already equipped.
func Equip(id uint16) {}

// Unequip removes the specified item by ID if it is currently equipped.
func Unequip(id uint16) {}

// PlaySound plays the sounds referenced by the provided IDs.
func PlaySound(ids []uint16) {}

// KeyJustPressed reports whether the given key was pressed this frame.
func KeyJustPressed(name string) bool { return false }

// MouseJustPressed reports whether the given mouse button was pressed this frame.
func MouseJustPressed(name string) bool { return false }

// MouseWheel returns the scroll wheel delta since the last frame.
func MouseWheel() (float64, float64) { return 0, 0 }

// Mobile contains basic info about a clicked mobile.
type Mobile struct {
	Index  uint8
	Name   string
	H, V   int16
	PictID uint16
	Colors uint8
}

// ClickInfo describes the last click in the game world.
type ClickInfo struct {
	X, Y     int16
	OnMobile bool
	Mobile   Mobile
}

// LastClick returns information about the last left-click in the world.
func LastClick() ClickInfo { return ClickInfo{} }

// EquippedItems returns the items currently equipped.
func EquippedItems() []InventoryItem { return nil }

// HasItem reports whether an inventory item with the given name exists.
func HasItem(name string) bool { return false }

// StorageGet retrieves a value previously stored with StorageSet.
func StorageGet(key string) any { return nil }

// Chat trigger flags
const (
	ChatAny      = 1 << iota // match any chat message
	ChatPlayer               // message from a known player (not NPC)
	ChatNPC                  // message from a known NPC
	ChatCreature             // message from an unknown/non-player speaker
	ChatSelf                 // message from yourself
	ChatOther                // message not from yourself
)

// StorageSet stores a value associated with key for the plugin.
func StorageSet(key string, value any) {}

// StorageDelete removes a stored value for key.
func StorageDelete(key string) {}

// RegisterChatHandler registers a callback for any chat message.
// The handler receives the full message text.
func RegisterChatHandler(fn func(msg string)) {}
