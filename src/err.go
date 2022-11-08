package main

import "log"

func handleErr(err error, msg string) {
	if err != nil {
		killTor()
		log.Fatalln(msg + "\nExiting...")
	}
}
