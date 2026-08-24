// Code generated for editor support.
// This file provides stubs for the "gt" package so editors can type-check
// scripts without the full client. Implementations are no-ops.

package gt

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// Version and client info
var CLVersion int

// Basic output
func Print(msg string)            {}
func ShowNotification(msg string) {}
func Notify(msg string)           {}

// Commands
type scriptCommandHandler func(args string)

type Subscription struct{}

func (Subscription) Remove()      {}
func (Subscription) Active() bool { return false }

type Timer struct{}

func (Timer) Stop()        {}
func (Timer) Active() bool { return false }

func RegisterCommand(name string, handler scriptCommandHandler)   {}
func Command(name string, handler func(args string)) Subscription { return Subscription{} }
func Run(cmd string)                                              {}
func Cmd(cmd string)                                              {}
func RunCommand(cmd string)                                       {}
func EnqueueCommand(cmd string)                                   {}
func Send(cmd string)                                             {}

// Hotkeys
type InputEvent struct {
	Chord      string
	Key        string
	Button     string
	Modifiers  []string
	Ctrl       bool
	Alt        bool
	Shift      bool
	Meta       bool
	ScreenX    int
	ScreenY    int
	WorldX     int16
	WorldY     int16
	OnMobile   bool
	Mobile     Mobile
	PlayerName string
	SimpleName string
}

func (InputEvent) Consume()        {}
func (InputEvent) Pass()           {}
func (InputEvent) Continues() bool { return true }

func AddHotkey(combo, command string)                          {}
func Bind(combo string, handler func(InputEvent)) Subscription { return Subscription{} }
func RemoveHotkey(combo string)                                {}
func Key(combo string, handler func())                         {}

// Shortcuts
func AddShortcut(short, full string)   {}
func AddShortcuts(m map[string]string) {}

// AddConfig registers a configurable value. The optional arguments are a
// default value and a one-argument change callback. It returns the persisted
// value, or the default when no saved value exists.
func AddConfig(name, typ string, args ...any) any { return nil }

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
type Player struct {
	Name         string
	Race         string
	Gender       string
	Class        string
	PictID       uint16
	Colors       []byte
	IsNPC        bool
	Sharee       bool
	Sharing      bool
	Friend       bool
	FriendLabel  int
	LocalLabel   int
	GlobalLabel  int
	Blocked      bool
	Ignored      bool
	Dead         bool
	FellWhere    string
	FellTime     time.Time
	KillerName   string
	Bard         bool
	SameClan     bool
	Seen         bool
	LastSeen     time.Time
	LastOnScreen time.Time
	Offline      bool
}

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

func WithEquipment(name string, task func()) {}

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
	Button   ebiten.MouseButton
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

type ChatFilter struct {
	Contains string
	Speaker  string
	Kinds    int
}

type ChatEvent struct {
	Speaker string
	Message string
	Raw     string
	Kinds   int
}

func OnChat(filter ChatFilter, handler func(ChatEvent)) Subscription { return Subscription{} }

type ServerMessageFilter struct {
	Contains string
	Type     string
}

type ServerMessageEvent struct {
	Message string
	Type    string
}

func OnServerMessage(filter ServerMessageFilter, handler func(ServerMessageEvent)) Subscription {
	return Subscription{}
}

type LifecycleEvent struct {
	Type              string
	Character         string
	PreviousCharacter string
	Reason            string
}

func OnLogin(handler func(LifecycleEvent)) Subscription           { return Subscription{} }
func OnLogout(handler func(LifecycleEvent)) Subscription          { return Subscription{} }
func OnCharacterChange(handler func(LifecycleEvent)) Subscription { return Subscription{} }
func OnStop(handler func(LifecycleEvent)) Subscription            { return Subscription{} }

const (
	ChangeInventory      = "inventory"
	ChangeEquipment      = "equipment"
	ChangeSelectedPlayer = "selected-player"
	ChangeSelectedItem   = "selected-item"
	ChangeVitals         = "vitals"
	ChangeWorld          = "world"
	ChangeLocation       = "location"
)

type ChangeEvent struct {
	Type            string
	Inventory       []InventoryItem
	Equipment       []InventoryItem
	SelectedPlayer  string
	SelectedItem    InventoryItem
	HasSelectedItem bool
	Health          int
	HealthMax       int
	Spirit          int
	SpiritMax       int
	Balance         int
	BalanceMax      int
	Location        string
	WorldGeneration uint64
}

func OnChange(kind string, handler func(ChangeEvent)) Subscription { return Subscription{} }

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
func SleepTicks(ticks int)        {}
func WaitTicks(ticks int)         {}
func Wait(duration time.Duration) {}
func WaitForInventory(name string, present bool, timeout time.Duration) bool {
	return false
}
func WaitForEquipment(name string, equipped bool, timeout time.Duration) bool {
	return false
}
func After(ms int, fn func())                        {}
func AfterDur(d time.Duration, fn func())            {}
func Every(ms int, fn func())                        {}
func EveryDur(d time.Duration, fn func())            {}
func Repeat(interval time.Duration, fn func()) Timer { return Timer{} }

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
