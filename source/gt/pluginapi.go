// Code generated for editor support.
// This file provides stubs for the "gt" package so editors can type-check
// scripts without the full client. Implementations are no-ops.

package gt

import "time"

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

const (
	ScopeGlobal    = "global"
	ScopeCharacter = "character"
)

type BoolOption struct {
	Key, Label, Help, Scope string
	Default                 bool
	Validate                func(bool) bool
	OnChange                func(bool)
}

type IntegerOption struct {
	Key, Label, Help, Scope string
	Default, Min, Max, Step int
	Validate                func(int) bool
	OnChange                func(int)
}

type DecimalOption struct {
	Key, Label, Help, Scope string
	Default, Min, Max, Step float64
	Validate                func(float64) bool
	OnChange                func(float64)
}

type TextOption struct {
	Key, Label, Help, Scope string
	Default                 string
	Validate                func(string) bool
	OnChange                func(string)
}

type ChoiceOption struct {
	Key, Label, Help, Scope string
	Default                 string
	Choices                 []string
	OnChange                func(string)
}

type KeyBindingOption struct {
	Key, Label, Help, Scope string
	Default                 string
	OnChange                func(string)
}

type ItemOption struct {
	Key, Label, Help, Scope string
	Default                 string
	OnChange                func(string)
}

func Bool(option BoolOption) bool               { return option.Default }
func Integer(option IntegerOption) int          { return option.Default }
func Decimal(option DecimalOption) float64      { return option.Default }
func Text(option TextOption) string             { return option.Default }
func Choice(option ChoiceOption) string         { return option.Default }
func KeyBinding(option KeyBindingOption) string { return option.Default }
func ItemSelector(option ItemOption) string     { return option.Default }

// Storage is private to the script that calls it.
func Store(key string, value any)                               {}
func LoadString(key, fallback string) string                    { return fallback }
func LoadBool(key string, fallback bool) bool                   { return fallback }
func LoadInteger(key string, fallback int) int                  { return fallback }
func LoadDecimal(key string, fallback float64) float64          { return fallback }
func LoadStrings(key string, fallback []string) []string        { return fallback }
func LoadJSON(key string, target any) bool                      { return false }
func DeleteStored(key string)                                   {}
func MigrateStorage(version int, migrate func(fromVersion int)) {}

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

type Character struct {
	Name       string
	Health     int
	HealthMax  int
	Spirit     int
	SpiritMax  int
	Balance    int
	BalanceMax int
	Location   string
	Equipment  []Item
}

func Self() Character   { return Character{} }
func Players() []Player { return nil }

// Inventory
type Item struct {
	InstanceID uint64
	ID         uint16
	Name       string
	Base       string
	Extra      string
	Equipped   bool
	Index      int
	IDIndex    int
	Quantity   int
	Slot       string
}

const (
	SlotForehead  = "forehead"
	SlotNeck      = "neck"
	SlotShoulder  = "shoulder"
	SlotArms      = "arms"
	SlotGloves    = "gloves"
	SlotFinger    = "finger"
	SlotCoat      = "coat"
	SlotCloak     = "cloak"
	SlotTorso     = "torso"
	SlotWaist     = "waist"
	SlotLegs      = "legs"
	SlotFeet      = "feet"
	SlotRightHand = "right-hand"
	SlotLeftHand  = "left-hand"
	SlotBothHands = "both-hands"
	SlotHead      = "head"
)

func Inventory() []Item                      { return nil }
func EquippedItems() []Item                  { return nil }
func FindItemExact(name string) (Item, bool) { return Item{}, false }
func FindItem(name string) (Item, bool)      { return Item{}, false }
func FindItems(name string) []Item           { return nil }
func SearchItems(text string) []Item         { return nil }
func Equipped(slot string) (Item, bool)      { return Item{}, false }
func HasItem(name string) bool               { return false }
func IsEquipped(name string) bool            { return false }
func Has(name string) bool                   { return HasItem(name) }

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
	Player bool
}

type Click struct {
	X, Y     int16
	OnMobile bool
	OnPlayer bool
	Mobile   Mobile
	Button   string
	Ctrl     bool
	Alt      bool
	Shift    bool
	Meta     bool
}

func LastClick() Click { return Click{} }
func Hover() Click     { return Click{} }

func SelectedPlayer() (Player, bool) { return Player{}, false }
func SelectedItem() (Item, bool)     { return Item{}, false }

type World struct {
	Width      int
	Height     int
	Location   string
	Generation uint64
	Mobiles    []Mobile
}

func CurrentWorld() World { return World{} }

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

type ServerMessage struct {
	Message    string
	Type       string
	Sequence   uint64
	ReceivedAt time.Time
}

func OnServerMessage(filter ServerMessageFilter, handler func(ServerMessage)) Subscription {
	return Subscription{}
}

func LatestServerMessage() (ServerMessage, bool) { return ServerMessage{}, false }

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
	Inventory       []Item
	Equipment       []Item
	SelectedPlayer  string
	SelectedItem    Item
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
