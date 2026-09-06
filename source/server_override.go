package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// serverAddressOverride belongs to this process, never to saved settings.
var serverAddressOverride string

func setServerAddressOverride(value string) error {
	value = strings.TrimSpace(value)
	hostname, port, err := net.SplitHostPort(value)
	if err != nil || strings.TrimSpace(hostname) == "" || strings.ContainsAny(hostname, " \t\r\n/") {
		return fmt.Errorf("server must be host:port (for example 127.0.0.1:5010 or [::1]:5010)")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535")
	}
	serverAddressOverride = net.JoinHostPort(hostname, strconv.Itoa(number))
	return nil
}
