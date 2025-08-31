// Package gt provides a tiny, editor-only stub for the
// interpreted plugin API exposed by the client at runtime via Yaegi.
//
// This package exists solely to satisfy linters and IDEs for files in
// the plugins/ directory (e.g., healer.go) that do `import "gt"`.
// The real implementations are injected into the Yaegi interpreter at
// runtime; these no-op stubs are never called by the compiled client.
package gt

import "time"

// ClientVersion mirrors the client version value exported to plugins.
var ClientVersion int

// Logf is a no-op printf-style logger for editor/linter happiness.
func Logf(format string, args ...interface{}) {}

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

// AddMacro replaces a short prefix with a full command in the chat box.
func AddMacro(short, full string) {}

// AddMacros registers multiple macros at once.
func AddMacros(macros map[string]string) {}

// AutoReply sends a command when a chat message begins with trigger.
func AutoReply(trigger, command string) {}

// PlayerName returns the current player's name.
func PlayerName() string { return "" }

// Player mirrors the player's state exposed to plugins.
type Player struct {
	Name       string
	Race       string
	Gender     string
	Class      string
	Clan       string
	PictID     uint16
	Colors     []byte
	IsNPC      bool
	Sharee     bool
	Sharing    bool
	GMLevel    int
	Friend     bool
	Blocked    bool
	Ignored    bool
	Dead       bool
	FellWhere  string
	FellTime   time.Time
	KillerName string
	Bard       bool
	SameClan   bool
	BeWho      bool
	LastSeen   time.Time
	Offline    bool
}

// Players returns the list of known players.
func Players() []Player { return nil }

// RegisterTriggers registers a callback for chat messages containing any phrase
// from the specified player name. An empty name matches any speaker.
// The handler receives the full message text.
func RegisterTriggers(name string, phrases []string, fn func(msg string)) {}

// RegisterChat registers a chat trigger with kind flags (ChatAny, ChatPlayer, ChatNPC, ChatCreature).
// Use an empty name to match any speaker; handler receives the full message.
func RegisterChat(name string, phrases []string, flags int, fn func(msg string)) {}

// Simpler chat helpers for non-programmers
func RegisterChatAny(name string, phrases []string, fn func(msg string)) {}
func RegisterChatPlayers(name string, phrases []string, fn func(msg string)) {}
func RegisterChatNPCs(name string, phrases []string, fn func(msg string)) {}
func RegisterChatCreatures(name string, phrases []string, fn func(msg string)) {}
func RegisterChatSelf(phrases []string, fn func(msg string)) {}
func RegisterChatOthers(name string, phrases []string, fn func(msg string)) {}

// No-slice convenience (one phrase per call)
func Chat(phrase string, fn func(msg string)) {}
func PlayerChat(phrase string, fn func(msg string)) {}
func NPCChat(phrase string, fn func(msg string)) {}
func CreatureChat(phrase string, fn func(msg string)) {}
func SelfChat(phrase string, fn func(msg string)) {}
func OtherChat(name, phrase string, fn func(msg string)) {}
func ChatFrom(name, phrase string, fn func(msg string)) {}
func PlayerChatFrom(name, phrase string, fn func(msg string)) {}
func OtherChatFrom(name, phrase string, fn func(msg string)) {}

// RegisterConsole registers a console trigger; handler receives the full message text.
func RegisterConsole(phrases []string, fn func(msg string)) {}
// ConsoleMsg registers a single console phrase (no slices needed).
func ConsoleMsg(phrase string, fn func(msg string)) {}
// Console registers a single console phrase (alias of ConsoleMsg).
func Console(phrase string, fn func(msg string)) {}

// Minimal DSL helpers
func Cmd(text string) {}
func Run(text string) {}
func Me() string { return "" }
func Has(name string) bool { return false }
func Toggle(id uint16) {}
func Save(key, value string) {}
func Load(key string) string { return "" }
func Delete(key string) {}
func Input() string { return "" }
func SetInput(text string) {}
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

// ToggleEquip toggles the equipped state of an item by ID.
func ToggleEquip(id uint16) {}

// InputText returns the current text in the input bar.
func InputText() string { return "" }

// SetInputText replaces the text in the input bar.
func SetInputText(text string) {}

// Stats mirrors the player's HP, SP, and balance values.
type Stats struct {
	HP, HPMax           int
	SP, SPMax           int
	Balance, BalanceMax int
}

// PlayerStats returns the player's current stat values.
func PlayerStats() Stats { return Stats{} }

// Equip equips the specified item by ID if it isn't already equipped.
func Equip(id uint16) {}

// Unequip removes the specified item by ID if it is currently equipped.
func Unequip(id uint16) {}

// PlaySound plays the sounds referenced by the provided IDs.
func PlaySound(ids []uint16) {}

// KeyPressed reports whether the given key is currently pressed.
func KeyPressed(name string) bool { return false }

// KeyJustPressed reports whether the given key was pressed this frame.
func KeyJustPressed(name string) bool { return false }

// MousePressed reports whether the given mouse button is pressed.
func MousePressed(name string) bool { return false }

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

// FrameNumber returns the current frame counter.
func FrameNumber() int { return 0 }

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
