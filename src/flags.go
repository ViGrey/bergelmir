package main

import (
	"os"
	"strings"
)

var (
	flags        cmdFlags
	bergelmirCmd = os.Args[0]
)

type cmdFlags struct {
	init bool
}

func getFlags() {
	f := os.Args[1:]
	for _, flag := range f {
		switch strings.ToLower(flag) {
		case "init":
			flags.init = true
		}
	}
}
