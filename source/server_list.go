package main

import (
	"net"
	"strconv"
	"strings"
)

const editServerListOption = "Edit Server List..."

var builtInServerAddresses = []string{
	defaultServerHostName + ":5010",
	"gothoom.m45sci.xyz:5010",
}

func normalizeServerAddress(value string) (string, bool) {
	hostname, port, err := net.SplitHostPort(strings.TrimSpace(value))
	if err != nil || strings.TrimSpace(hostname) == "" || strings.ContainsAny(hostname, " \t\r\n/") {
		return "", false
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return "", false
	}
	return net.JoinHostPort(hostname, strconv.Itoa(number)), true
}

func sameServerAddress(a, b string) bool {
	a, okA := normalizeServerAddress(a)
	b, okB := normalizeServerAddress(b)
	return okA && okB && strings.EqualFold(a, b)
}

func serverAddresses() []string {
	return append([]string(nil), gs.ServerAddresses...)
}

// Server slots are one-based positions in the saved server list. Characters
// refer to the slot so editing an address does not change their association.
func serverSlotForAddress(address string) int {
	for index, candidate := range serverAddresses() {
		if sameServerAddress(candidate, address) {
			return index + 1
		}
	}
	return 0
}

func selectedServerSlot() int {
	if slot := serverSlotForAddress(gs.ServerAddress); slot > 0 {
		return slot
	}
	return 1
}

func normalizeServerListSettings() {
	addresses := serverAddresses()
	if len(addresses) == 0 {
		addresses = append([]string(nil), builtInServerAddresses...)
	}
	for index, address := range addresses {
		if normalized, ok := normalizeServerAddress(address); ok {
			addresses[index] = normalized
		} else {
			addresses[index] = strings.TrimSpace(address)
		}
	}
	if address, ok := normalizeServerAddress(gs.ServerAddress); ok {
		gs.ServerAddress = address
	} else {
		gs.ServerAddress = addresses[0]
	}
	found := false
	for _, address := range addresses {
		found = found || sameServerAddress(address, gs.ServerAddress)
	}
	if !found {
		addresses = append(addresses, gs.ServerAddress)
	}
	gs.ServerAddresses = addresses
}

func addServerAddress(address string) bool {
	address, ok := normalizeServerAddress(address)
	if !ok {
		return false
	}
	for _, existing := range gs.ServerAddresses {
		if sameServerAddress(existing, address) {
			return false
		}
	}
	gs.ServerAddresses = append(gs.ServerAddresses, address)
	return true
}

func editServerSlot(slot int, replacement string) bool {
	replacement, ok := normalizeServerAddress(replacement)
	if !ok || slot < 1 || slot > len(gs.ServerAddresses) {
		return false
	}
	index := slot - 1
	for i, address := range gs.ServerAddresses {
		if i == index {
			continue
		}
		if sameServerAddress(address, replacement) {
			return false
		}
	}
	selected := selectedServerSlot() == slot
	updated := append([]string(nil), gs.ServerAddresses...)
	updated[index] = replacement
	gs.ServerAddresses = updated
	if selected {
		gs.ServerAddress = replacement
	}
	return true
}
