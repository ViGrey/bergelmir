package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

const (
	VERSION = "0.0.1"
)

func init() {
	getFlags()
	if flags.init {
		initBergelmirProject()
		os.Exit(0)
	} else {
		parseConfigData()
	}
}

func main() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	fmt.Println("Starting Bergelmir")
	if configData.Tor.Enabled {
		fmt.Println("- Starting Tor")
		torConnected = make(chan bool)
		startTor()
		connectToTor()
		<-torConnected
		fmt.Println("- Tor started")
		fmt.Printf("- Tor onion address is %s\n", torAddress)
	}
	fmt.Printf("- Starting Gemini capsule at gemini://%s\n", configData.Gemini.ListeningLocation)
	go startGeminiServer()
	// Show the Gemini capsule .onion address if tor is enabled
	if configData.Tor.Enabled {
		fmt.Printf("- Gemini capsule is accessible over tor at gemini://%s", torAddress)
		// Only show port if not default Gemini port
		if configData.Gemini.Tor.VirtualPort != GEMINI_DEFAULT_PORT {
			fmt.Printf(":%d", configData.Gemini.Tor.VirtualPort)
		}
		fmt.Print("\n")
	}
	if configData.HTTP.Enabled {
		fmt.Printf("- Starting HTTP server at http://%s\n", configData.HTTP.ListeningLocation)
		// Start the HTTP server
		go startHTTPServer()
		// Show the HTTP server .onion address if tor is enabled
		if configData.Tor.Enabled {
			fmt.Printf("- HTTP server is accessible over tor at http://%s", torAddress)
			// Only show port if not default HTTP port
			if configData.HTTP.Tor.VirtualPort != HTTP_DEFAULT_PORT {
				fmt.Printf(":%d", configData.HTTP.Tor.VirtualPort)
			}
			fmt.Print("\n")
		}
	}
	//generateNewTLSCertAndKey()
	<-c
}
