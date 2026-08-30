package main

import (
	"sort"
	"strings"
	"unicode"
)

// serverCommandNames is the set of server commands documented by the client.
// Local, script, and legacy-macro commands are added dynamically below.
var serverCommandNames = []string{
	"action", "affiliations", "anoncurse", "anonthank", "bag", "boot", "bug",
	"buy", "curse", "depart", "drop", "equip", "examine", "give", "help",
	"info", "karma", "money", "name", "narrate", "news", "options", "ponder",
	"pose", "pray", "pull", "push", "report", "sell", "share", "show", "sky",
	"sleep", "speak", "status", "thank", "think", "thinkclan", "thinkgroup",
	"thinkto", "tip", "unequip", "unshare", "use", "useitem", "whisper", "who",
	"whoclan", "yell",
}

var localCommandNames = []string{"palette", "play", "setting", "testhooks"}

type inputCompletionCandidates struct {
	commands []string
	items    []string
	chat     []string
}

// inputCompletionSuffix returns the gray text that can be appended at cursor.
// Completion is intentionally limited to a cursor at the end of the line so
// the prediction is always an unambiguous visual suffix.
func inputCompletionSuffix(text string, cursor int, candidates inputCompletionCandidates) string {
	runes := []rune(text)
	if cursor != len(runes) || cursor <= 0 {
		return ""
	}

	firstEnd := len(runes)
	for i, r := range runes {
		if unicode.IsSpace(r) {
			firstEnd = i
			break
		}
	}
	first := string(runes[:firstEnd])
	if firstEnd == len(runes) {
		if strings.HasPrefix(first, "/") {
			return completionCandidateSuffix(first, candidates.commands)
		}
		// Legacy expression macros need not begin with a slash. Command
		// candidates take priority over player and inventory names here.
		if suffix := completionCandidateSuffix(first, candidates.commands); suffix != "" {
			return suffix
		}
		return completionAtWordBoundary(string(runes), candidates.chat)
	}

	if exactCandidate(first, candidates.commands) {
		return completionAtWordBoundary(string(runes[firstEnd+1:]), candidates.items)
	}
	return completionAtWordBoundary(string(runes), candidates.chat)
}

func completionAtWordBoundary(text string, candidates []string) string {
	runes := []rune(text)
	bestPrefix := ""
	bestSuffix := ""
	starts := []int{0}
	for i, r := range runes {
		if unicode.IsSpace(r) && i+1 < len(runes) {
			starts = append(starts, i+1)
		}
	}
	for _, start := range starts {
		prefix := string(runes[start:])
		if strings.TrimSpace(prefix) == "" || len([]rune(prefix)) < len([]rune(bestPrefix)) {
			continue
		}
		if suffix := completionCandidateSuffix(prefix, candidates); suffix != "" {
			bestPrefix = prefix
			bestSuffix = suffix
		}
	}
	return bestSuffix
}

func completionCandidateSuffix(prefix string, candidates []string) string {
	if prefix == "" {
		return ""
	}
	ordered := sortedUniqueCandidates(candidates)
	for _, candidate := range ordered {
		if strings.EqualFold(candidate, prefix) {
			return ""
		}
	}
	lower := strings.ToLower(prefix)
	for _, candidate := range ordered {
		candidateRunes := []rune(candidate)
		prefixRunes := []rune(prefix)
		if !strings.HasPrefix(strings.ToLower(candidate), lower) || len(candidateRunes) <= len(prefixRunes) {
			continue
		}
		return string(candidateRunes[len(prefixRunes):])
	}
	return ""
}

func exactCandidate(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if strings.EqualFold(value, candidate) {
			return true
		}
	}
	return false
}

func sortedUniqueCandidates(values []string) []string {
	byFolded := make(map[string]string, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if existing, ok := byFolded[key]; !ok || value < existing {
			byFolded[key] = value
		}
	}
	result := make([]string, 0, len(byFolded))
	for _, value := range byFolded {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i]) < strings.ToLower(result[j])
	})
	return result
}

func currentInputCompletionCandidates() inputCompletionCandidates {
	commands := make([]string, 0, len(serverCommandNames)+len(localCommandNames))
	for _, name := range serverCommandNames {
		commands = append(commands, "/"+name)
	}
	for _, name := range localCommandNames {
		commands = append(commands, "/"+name)
	}

	scriptMu.RLock()
	for name, owner := range scriptCommandOwners {
		if !scriptDisabled[owner] {
			commands = append(commands, "/"+name)
		}
	}
	scriptMu.RUnlock()

	legacyMacrosMu.RLock()
	for _, declaration := range legacyMacrosProgram.Macros {
		if declaration.Kind == legacyMacroExpression {
			commands = append(commands, declaration.Trigger)
		}
	}
	legacyMacrosMu.RUnlock()

	items := getInventory()
	itemNames := make([]string, 0, len(items))
	for _, item := range items {
		if item.Name != "" {
			itemNames = append(itemNames, item.Name)
		}
	}

	chat := append([]string(nil), itemNames...)
	for _, player := range getPlayers() {
		if player.Name != "" && !player.IsNPC {
			chat = append(chat, player.Name)
		}
	}
	return inputCompletionCandidates{commands: commands, items: itemNames, chat: chat}
}

func currentInputCompletionSuffix(text string, cursor int) string {
	return inputCompletionSuffix(text, cursor, currentInputCompletionCandidates())
}
