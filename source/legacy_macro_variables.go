package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// legacyMacroExecutionContext holds values captured when a macro starts.
// Input and click integration fill it in later; functions already receive the
// current input text, matching the reference client's execution context.
type legacyMacroExecutionContext struct {
	Text          string
	TextSelection string

	ClickName      string
	ClickButton    int
	ClickChord     int
	HasClickName   bool
	HasClickButton bool
	HasClickChord  bool
}

func legacyMacroDefaultExecutionContext() legacyMacroExecutionContext {
	inputMu.Lock()
	text := string(inputText)
	inputMu.Unlock()
	return legacyMacroExecutionContext{
		Text:          text,
		TextSelection: legacyMacroFrameInputSelection(),
	}
}

// legacyMacroInputSelection snapshots the highlighted typing-box text. The
// displayed input is soft-wrapped, so remove only those visual line breaks.
func legacyMacroInputSelection() string {
	if !inputActive || inputFlow == nil || len(inputFlow.Contents) == 0 {
		return ""
	}
	return strings.ReplaceAll(inputFlow.Contents[0].SelectedText(), "\n", "")
}

func (context legacyMacroExecutionContext) initialVariables() map[string]string {
	values := map[string]string{
		"@text":    context.Text,
		"@textsel": context.TextSelection,
	}
	if context.HasClickName {
		values["@click.name"] = context.ClickName
		values["@click.simple_name"] = legacyMacroSimplePlayerName(context.ClickName)
	}
	if context.HasClickButton {
		values["@click.button"] = strconv.Itoa(context.ClickButton)
	}
	if context.HasClickChord {
		values["@click.chord"] = strconv.Itoa(context.ClickChord)
	}
	return values
}

func legacyMacroSimplePlayerName(name string) string {
	var simple strings.Builder
	simple.Grow(len(name))
	for _, char := range name {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			simple.WriteRune(char)
		}
	}
	return simple.String()
}

var legacyMacroTextLog struct {
	sync.RWMutex
	text string
}

func legacyMacroSetTextLog(text string) {
	legacyMacroSetDisplayedTextLog(text, false)
}

func legacyMacroSetDisplayedTextLog(text string, timestamps bool) {
	if timestamps {
		text = legacyMacroTimestampedTextLog(text, time.Now())
	}
	legacyMacroTextLog.Lock()
	legacyMacroTextLog.text = text
	legacyMacroTextLog.Unlock()
}

func legacyMacroTimestampedTextLog(text string, now time.Time) string {
	hour := now.Hour()
	ampm := byte('a')
	if hour >= 12 {
		ampm = 'p'
	}
	hour %= 12
	if hour == 0 {
		hour = 12
	}
	return fmt.Sprintf("%d/%d/%.2d %d:%.2d:%.2d%c %s",
		int(now.Month()), now.Day(), now.Year()%100,
		hour, now.Minute(), now.Second(), ampm, text)
}

func legacyMacroTextLogValue() string {
	legacyMacroTextLog.RLock()
	defer legacyMacroTextLog.RUnlock()
	return legacyMacroTextLog.text
}

// legacyMacroCurrentGameVariable implements the variables that the reference
// client resolves directly from game state. Other @env values remain normal
// macro variables so existing set/setglobal files keep their semantics.
func legacyMacroCurrentGameVariable(name string) (string, bool) {
	switch strings.ToLower(name) {
	case "@env.textlog":
		return legacyMacroTextLogValue(), true
	case "@my.name":
		return playerName, true
	case "@my.simple_name":
		return legacyMacroSimplePlayerName(playerName), true
	case "@selplayer.name":
		return selectedPlayerName, true
	case "@selplayer.simple_name":
		return legacyMacroSimplePlayerName(selectedPlayerName), true
	case "@my.shares_in":
		return legacyMacroShares(false), true
	case "@my.shares_out":
		return legacyMacroShares(true), true
	case "@my.selected_item":
		return legacyMacroSelectedItemName(), true
	}

	if slot, ok := legacyMacroItemSlots[strings.ToLower(name)]; ok {
		return legacyMacroEquippedItemName(slot), true
	}
	return "", false
}

var legacyMacroItemSlots = map[string]int{
	"@my.forehead_item":  kItemSlotForehead,
	"@my.neck_item":      kItemSlotNeck,
	"@my.shoulder_item":  kItemSlotShoulder,
	"@my.shoulders_item": kItemSlotShoulder,
	"@my.arms_item":      kItemSlotArms,
	"@my.gloves_item":    kItemSlotGloves,
	"@my.finger_item":    kItemSlotFinger,
	"@my.coat_item":      kItemSlotCoat,
	"@my.cloak_item":     kItemSlotCloak,
	"@my.torso_item":     kItemSlotTorso,
	"@my.waist_item":     kItemSlotWaist,
	"@my.legs_item":      kItemSlotLegs,
	"@my.feet_item":      kItemSlotFeet,
	"@my.right_item":     kItemSlotRightHand,
	"@my.left_item":      kItemSlotLeftHand,
	"@my.hands_item":     kItemSlotBothHands,
	"@my.head_item":      kItemSlotHead,
}

func legacyMacroEquippedItemName(slot int) string {
	if clImages == nil {
		return "Nothing"
	}
	for _, item := range getInventory() {
		if item.Equipped && clImages.ItemSlot(uint32(item.ID)) == slot {
			return item.Name
		}
	}
	return "Nothing"
}

func legacyMacroSelectedItemName() string {
	if selectedInvID == 0 {
		return "Nothing"
	}
	for _, item := range getInventory() {
		if item.ID == selectedInvID && item.IDIndex == selectedInvIdx {
			return item.Name
		}
	}
	return "Nothing"
}

func legacyMacroShares(outbound bool) string {
	playersMu.RLock()
	names := make([]string, 0)
	for _, player := range players {
		if player == nil || strings.EqualFold(player.Name, playerName) {
			continue
		}
		if (outbound && player.Sharee) || (!outbound && player.Sharing) {
			names = append(names, legacyMacroSimplePlayerName(player.Name))
		}
	}
	playersMu.RUnlock()
	sort.Strings(names)
	return strings.Join(names, " ")
}

// legacyMacroReadVariableName maps the aliases accepted by GetVariable in
// the reference client. The old environment aliases are also accepted by
// set/setglobal through legacyMacroWritableVariableName below.
func legacyMacroReadVariableName(name string) string {
	name = strings.ToLower(name)
	switch name {
	case "@name":
		return "@my.name"
	case "@splayer":
		return "@selplayer.simple_name"
	case "@rplayer":
		return "@selplayer.name"
	case "@rhanditem":
		return "@my.right_item"
	case "@lhanditem":
		return "@my.left_item"
	case "@my.leftitem":
		return "@my.left_item"
	case "@my.rightitem":
		return "@my.right_item"
	case "@echo":
		return "@env.echo"
	case "@debug":
		return "@env.debug"
	case "@interruptclick":
		return "@env.click_interrupts"
	case "@interruptkey":
		return "@env.key_interrupts"
	case "@clicksplayer":
		return "@click.simple_name"
	case "@clickplayer":
		return "@click.simple_name"
	case "@clickrplayer":
		return "@click.name"
	case "@wordcount":
		return "@text.num_words"
	case "@text.word_count":
		return "@text.num_words"
	}

	if len(name) >= len("@word[") && strings.HasPrefix(name, "@word[") {
		return "@text." + name[1:]
	}
	return name
}

func legacyMacroWritableVariableName(name string) string {
	name = strings.ToLower(name)
	switch name {
	case "@echo":
		return "@env.echo"
	case "@debug":
		return "@env.debug"
	case "@interruptclick":
		return "@env.click_interrupts"
	case "@interruptkey":
		return "@env.key_interrupts"
	default:
		return name
	}
}

func legacyMacroSplitTextTrailers(name string) (string, string, bool) {
	lower := strings.ToLower(name)
	for index := 0; index < len(name); index++ {
		if name[index] != '.' {
			continue
		}
		trailer := lower[index:]
		if strings.HasPrefix(trailer, ".word[") || strings.HasPrefix(trailer, ".letter[") ||
			strings.HasPrefix(trailer, ".num_words") || strings.HasPrefix(trailer, ".num_letters") {
			return name[:index], name[index:], true
		}
	}
	return "", "", false
}

func legacyMacroApplyTextTrailers(value, trailers string) string {
	for trailers != "" {
		lower := strings.ToLower(trailers)
		switch {
		case strings.HasPrefix(lower, ".word["):
			end := strings.IndexByte(trailers, ']')
			if end < len(".word[") {
				return value
			}
			index, ok := legacyMacroStringToInt(strings.TrimSpace(trailers[len(".word["):end]))
			if !ok {
				return value
			}
			words := strings.Fields(value)
			if index < 0 || index >= len(words) {
				value = ""
			} else {
				value = words[index]
			}
			trailers = trailers[end+1:]
		case strings.HasPrefix(lower, ".letter["):
			end := strings.IndexByte(trailers, ']')
			if end < len(".letter[") {
				return value
			}
			index, ok := legacyMacroStringToInt(strings.TrimSpace(trailers[len(".letter["):end]))
			if !ok {
				return value
			}
			letters := []rune(value)
			if index < 0 || index >= len(letters) {
				value = ""
			} else {
				value = string(letters[index])
			}
			trailers = trailers[end+1:]
		case strings.HasPrefix(lower, ".num_words"):
			value = strconv.Itoa(len(strings.Fields(value)))
			trailers = trailers[len(".num_words"):]
		case strings.HasPrefix(lower, ".num_letters"):
			value = strconv.Itoa(len([]rune(value)))
			trailers = trailers[len(".num_letters"):]
		default:
			return value
		}
	}
	return value
}
