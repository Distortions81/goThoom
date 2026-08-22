// Code generated for editor support.
// This file provides stubs for the "gt" package so editors can type-check
// scripts without the full client. Implementations are no-ops.

package gt

import "time"

// Version and client info
var clVersion int

// Basic output
func Print(msg string)            {}
func ShowNotification(msg string) {}
func Notify(msg string)           {}

// Commands
type scriptCommandHandler func(args string)

func RegisterCommand(name string, handler scriptCommandHandler) {}
func Run(cmd string)                                            {}
func Cmd(cmd string)                                            {}
func RunCommand(cmd string)                                     {}
func EnqueueCommand(cmd string)                                 {}

// Hotkeys
type HotkeyEvent struct {
	Combo   string
	Parts   []string
	Trigger string
}

func AddHotkey(combo, command string)                     {}
func AddHotkeyFn(combo string, handler func(HotkeyEvent)) {}
func RemoveHotkey(combo string)                           {}
func Key(combo string, handler func())                    {}

// Shortcuts
func AddShortcut(short, full string)   {}
func AddShortcuts(m map[string]string) {}

// Config entries
func AddConfig(name, typ string) {}

// Storage (per-script persistent key/value)
func StorageGet(key string) any        { return nil }
func StorageSet(key string, value any) {}
func StorageDelete(key string)         {}

// Convenience string-only helpers
func Save(key, value string) {}
func Load(key string) string { return "" }
func Delete(key string)      {}

// Input box helpers
func InputText() string        { return "" }
func SetInputText(text string) {}

// Aliases
func Input() string        { return InputText() }
func SetInput(text string) { SetInputText(text) }

// Player and world info
type Player struct{ Name string }

func PlayerName() string { return "" }
func Me() string         { return PlayerName() }
func Players() []Player  { return nil }

// Inventory
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

func Inventory() []InventoryItem     { return nil }
func EquippedItems() []InventoryItem { return nil }
func HasItem(name string) bool       { return false }
func IsEquipped(name string) bool    { return false }
func Has(name string) bool           { return HasItem(name) }

// Equip equips an item by name (case-insensitive).
func Equip(name string) {}

// Unequip unequips an item by name (case-insensitive).
func Unequip(name string) {}

// EquipPartial equips the first item whose name contains the substring (case-insensitive).
func EquipPartial(name string) {}

// UnequipPartial unequips an equipped item whose name contains the substring (case-insensitive).
func UnequipPartial(name string) {}

// EquipById equips an item by numeric ID.
func EquipById(id uint16) {}

// UnequipById unequips an item by numeric ID.
func UnequipById(id uint16) {}

// Images and world overlay
func WorldSize() (int, int)                              { return 0, 0 }
func ImageSize(id uint16) (int, int)                     { return 0, 0 }
func OverlayClear()                                      {}
func OverlayRect(x, y, w, h int, r, g, b, a uint8)       {}
func OverlayText(x, y int, txt string, r, g, b, a uint8) {}
func OverlayImage(id uint16, x, y int)                   {}

// Mouse and keyboard
func KeyJustPressed(name string) bool   { return false }
func MouseJustPressed(name string) bool { return false }
func MouseWheel() (float64, float64)    { return 0, 0 }

// Last world click
type Mobile struct {
	Index  uint8
	Name   string
	H, V   int16
	PictID uint16
	Colors uint8
}
type ClickInfo struct {
	X, Y     int16
	OnMobile bool
	OnPlayer bool
	Mobile   Mobile
	Button   int // placeholder; real value is an ebiten.MouseButton
	Ctrl     bool
	Alt      bool
	Shift    bool
}

func LastClick() ClickInfo { return ClickInfo{} }

// Chat trigger kinds
const (
	ChatAny = 1 << iota
	ChatPlayer
	ChatNPC
	ChatCreature
	ChatSelf
	ChatOther
)

// Chat and console triggers
func Chat(phrase string, handler func(string))                        {}
func PlayerChat(phrase string, handler func(string))                  {}
func NPCChat(phrase string, handler func(string))                     {}
func CreatureChat(phrase string, handler func(string))                {}
func SelfChat(phrase string, handler func(string))                    {}
func OtherChat(name, phrase string, handler func(string))             {}
func ChatFrom(name, phrase string, handler func(string))              {}
func PlayerChatFrom(name, phrase string, handler func(string))        {}
func OtherChatFrom(name, phrase string, handler func(string))         {}
func ConsoleMsg(phrase string, handler func(string))                  {}
func Console(phrase string, handler func(string))                     {}
func RegisterTriggers(name string, phrases []string, fn func(string)) {}
func RegisterConsoleTriggers(phrases []string, fn func())             {}
func RegisterTrigger(name, phrase string, fn func())                  {}
func RegisterPlayerHandler(fn func(Player))                           {}
func RegisterInputHandler(fn func(string) string)                     {}
func RegisterChatHandler(fn func(string))                             {}

// Time helpers
func SleepTicks(ticks int)                {}
func After(ms int, fn func())             {}
func AfterDur(d time.Duration, fn func()) {}
func Every(ms int, fn func())             {}
func EveryDur(d time.Duration, fn func()) {}

// Sound
func PlaySound(ids []uint16) {}

// String helpers
func IgnoreCase(a, b string) bool            { return false }
func StartsWith(s, prefix string) bool       { return false }
func EndsWith(s, suffix string) bool         { return false }
func Includes(s, substr string) bool         { return false }
func Lower(s string) string                  { return s }
func Upper(s string) string                  { return s }
func Trim(s string) string                   { return s }
func TrimStart(s, prefix string) string      { return s }
func TrimEnd(s, suffix string) string        { return s }
func Words(s string) []string                { return nil }
func Join(parts []string, sep string) string { return "" }
func Replace(s, old, new string) string      { return s }
func Split(s, sep string) []string           { return nil }
