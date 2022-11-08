package main

import (
	"regexp"
	"strings"
)

var (
	locationPathRe = regexp.MustCompile("^(?:(?i)(tcp|udp|unix):)?(.*)")
)

// Determine if locationPath is a tcp, udp, or unix socket location and
// return the protocol type (tcp, udp, or unix) and the location
// If there is no protocol, default to tcp.
func parseLocation(locationPath string) (protocol, location string) {
	matches := locationPathRe.FindStringSubmatch(locationPath)
	if len(matches) > 1 {
		protocol = strings.ToLower(matches[1])
		if len(matches) > 2 {
			location = matches[2]
		}
	}
	if protocol == "" {
		protocol = "tcp"
	}
	return
}
