package main

import (
	"net"
	"strconv"
	"strings"
)

const editServerListOption = "Edit Server List..."

var builtInServerAddresses = []string{
	defaultServerHostName + ":5010",
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

func isBuiltInServerAddress(address string) bool {
	for _, builtIn := range builtInServerAddresses {
		if sameServerAddress(address, builtIn) {
			return true
		}
	}
	return false
}

func serverAddresses() []string {
	addresses := make([]string, 0, len(builtInServerAddresses)+len(gs.ServerAddresses)+1)
	appendAddress := func(address string) {
		address, ok := normalizeServerAddress(address)
		if !ok {
			return
		}
		for _, existing := range addresses {
			if sameServerAddress(existing, address) {
				return
			}
		}
		addresses = append(addresses, address)
	}
	for _, address := range builtInServerAddresses {
		appendAddress(address)
	}
	for _, address := range gs.ServerAddresses {
		appendAddress(address)
	}
	appendAddress(gs.ServerAddress)
	return addresses
}

func normalizeServerListSettings() {
	addresses := serverAddresses()
	var custom []string
	for _, address := range addresses {
		if !isBuiltInServerAddress(address) {
			custom = append(custom, address)
		}
	}
	gs.ServerAddresses = custom
	if address, ok := normalizeServerAddress(gs.ServerAddress); ok {
		gs.ServerAddress = address
	} else {
		gs.ServerAddress = gsdef.ServerAddress
	}
}

func addServerAddress(address string) bool {
	address, ok := normalizeServerAddress(address)
	if !ok {
		return false
	}
	for _, existing := range serverAddresses() {
		if sameServerAddress(existing, address) {
			return true
		}
	}
	gs.ServerAddresses = append(gs.ServerAddresses, address)
	return true
}

func removeServerAddress(address string) bool {
	if isBuiltInServerAddress(address) {
		return false
	}
	for i, existing := range gs.ServerAddresses {
		if sameServerAddress(existing, address) {
			gs.ServerAddresses = append(gs.ServerAddresses[:i], gs.ServerAddresses[i+1:]...)
			if sameServerAddress(gs.ServerAddress, address) {
				gs.ServerAddress = gsdef.ServerAddress
			}
			return true
		}
	}
	return false
}
