package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var (
	//go:embed http/layout.html
	defaultHTTPHTMLLayoutContent []byte
	//go:embed tor/torrc
	defaultTorrcFileContent []byte
)

// Get user input from stdin (newline terminated)
func getUserInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	handleErr(err, "Unable to read user input from terminal")
	return (strings.TrimSuffix(input, "\n"))
}

// Get user text input from stdin otherwise defaultVal if the input is blank
func getUserInputText(prompt, defaultVal string) string {
	t := getUserInput(prompt)
	if t == "" {
		return defaultVal
	}
	return t
}

// Get user bool (y/n) input from stdin otherwise defaultVal if the input is
// blank
func getUserInputYN(prompt string, defaultVal bool) bool {
	for {
		b := strings.ToLower(getUserInput(prompt))
		switch {
		case b == "y":
			return true
		case b == "n":
			return false
		case b == "":
			return defaultVal
		default:
			fmt.Println("Value must be \"Y\" or \"N\".  Try again.")
		}
	}
}

// Get user int input from stdin between min and max that is not in
// forbiddenValues otherwise defaultVal if input is blank
func getUserInputInt(prompt string, min, max, defaultVal int,
	forbiddenValues []int) int {
	for {
		iStr := getUserInput(prompt)
		if iStr == "" {
			iStr = strconv.Itoa(defaultVal)
		}
		i, err := strconv.Atoi(iStr)
		if err == nil && i >= min && i <= max {
			forbiddenValue := -1
			for _, fi := range forbiddenValues {
				if i == fi {
					forbiddenValue = i
					break
				}
			}
			if forbiddenValue < 0 {
				return i
			}
			fmt.Printf("Value cannot be \"%d\".  Try again.\n", forbiddenValue)
		} else {
			fmt.Printf("Value must be an integer between (and including) \"%d\" and \"%d\".  Try again.\n", min, max)
		}
	}
}

// Generate initial configData values that aren't asked for by user input
// prompts
func initConfigData() {
	configData = Config{
		BergelmirVersion: VERSION,
		Tor: ConfigTor{
			ControlAuthCookiePath:       "tor/control_auth_cookie",
			ControlPortFilePath:         "tor/control_port",
			HiddenServicePrivateKeyPath: "tor/hs_ed25519_secret_key",
			TorrcPath:                   "tor/torrc",
		},
		Gemini: ConfigGemini{
			DataPath:          "gemini/",
			ListeningLocation: "127.0.0.1:1965",
			TLS: ConfigGeminiTLS{
				CertPath: "tls/cert.pem",
				KeyPath:  "tls/cert.key",
			},
			Tor: ConfigGeminiTor{
				VirtualPort: GEMINI_DEFAULT_PORT,
			},
		},
		HTTP: ConfigHTTP{
			DataPath:          "http/",
			LayoutHTMLPath:    "http/layout.html",
			ListeningLocation: "127.0.0.1:8080",
			Tor: ConfigHTTPTor{
				VirtualPort: HTTP_DEFAULT_PORT,
			},
		},
	}
}

// Ask user for user input to populate configData and save configData to
// config.yaml
func initBergelmirProject() {
	if fileExists(CONFIG_FILE_PATH) {
		if !getUserInputYN(CONFIG_FILE_PATH+
			" already exists.\nCreate new config file anyways?\n"+
			"This will overwrite "+CONFIG_FILE_PATH+" [y/N]: ", false) {
			fmt.Println("Exiting...")
			os.Exit(0)
		}
		fmt.Println()
	}
	initConfigData()
	//generateConfigFile()
	fmt.Println("Initializing Bergelmir Project")
	// Ask if tor should be enabled
	configData.Tor.Enabled = getUserInputYN(
		"Enable Tor \".onion\" address? [y/N]: ", false)
	// Ask which port the gemini capsule is listening on
	geminiPort := getUserInputInt("Gemini capsule listening port [ 1965 ]: ",
		1, 65535, GEMINI_DEFAULT_PORT, []int{})
	configData.Gemini.ListeningLocation = strconv.Itoa(geminiPort)
	// Ask if gemini capsule is listening on localhost
	if getUserInputYN(
		"Limit Gemini capsule reachability to localhost [y/N]: ", false) {
		configData.Gemini.ListeningLocation = "127.0.0.1:" +
			configData.Gemini.ListeningLocation
	} else {
		configData.Gemini.ListeningLocation = "0.0.0.0:" +
			configData.Gemini.ListeningLocation
	}
	// Ask for domain names
	configData.Gemini.DomainNames = strings.Fields(
		getUserInput("Domain names of Gemini capsule (space seperated):\n"))
	// Ask for gemini capsule file path
	configData.Gemini.DataPath = getUserInputText(
		"Path for Gemini capsule files [ gemini/ ]: ", "gemini/")
	if configData.Tor.Enabled {
		// Ask which port the tor hidden service for the gemini capsule is
		// listening on
		configData.Gemini.Tor.VirtualPort = getUserInputInt(
			"Tor listening port for Gemini capsule [ 1965 ]: ",
			1, 65535, GEMINI_DEFAULT_PORT, []int{})
	}
	// Ask if RSS feed should be enabled
	configData.RSS.Enabled = getUserInputYN(
		"Enable RSS feed at /rss and /feed URLs? [Y/n]", true)
	if configData.RSS.Enabled {
		configData.RSS.FeedSourceGeminiPath = getUserInputText(
			"Gemini source path for RSS feed [ /blog ]: ", "blog")
	}
	// Ask if http server should be enabled
	configData.HTTP.Enabled = getUserInputYN(
		"Enable HTTP server? [Y/n]: ", true)
	if configData.HTTP.Enabled {
		// Ask which port the http server is listening on
		configData.HTTP.ListeningLocation = strconv.Itoa(
			getUserInputInt("HTTP server listening port [ 8080 ]: ",
				1, 65535, 8080, []int{geminiPort}))
		// Ask if http server is listening on localhost
		if getUserInputYN(
			"Limit HTTP server reachability to localhost [y/N]: ", false) {
			configData.HTTP.ListeningLocation = "127.0.0.1:" +
				configData.HTTP.ListeningLocation
		} else {
			configData.HTTP.ListeningLocation = "0.0.0.0:" +
				configData.HTTP.ListeningLocation
		}
		// Ask for http server file path
		configData.HTTP.DataPath = getUserInputText(
			"Path for HTTP server files [ http/ ]: ", "http/")
		// Set default HTML layout file path
		configData.HTTP.LayoutHTMLPath = strings.TrimSuffix(
			configData.HTTP.DataPath, "/") + "/layout.html"
		// Ask for default html page title
		configData.HTTP.DefaultPageTitle = getUserInput(
			"Default HTML page title: ")
		if configData.Tor.Enabled {
			// Ask which port the tor hidden service for the http server is
			// listening on
			configData.HTTP.Tor.VirtualPort = getUserInputInt(
				"Tor listening port for HTTP Server [ 80 ]: ",
				1, 65535, HTTP_DEFAULT_PORT, []int{configData.Gemini.Tor.VirtualPort})
		}
	}

	// Create torrc path directory
	createFileDirectory(configData.Tor.TorrcPath)
	if !fileExists(configData.Tor.TorrcPath) {
		// Write default torrc file to torrc path if it does not already exist
		torrcData := bytes.ReplaceAll(defaultTorrcFileContent,
			[]byte("%COOKIE_AUTH_FILE%"),
			[]byte(configData.Tor.ControlAuthCookiePath))
		torrcData = bytes.ReplaceAll(torrcData, []byte("%CONTROL_PORT_FILE%"),
			[]byte(configData.Tor.ControlPortFilePath))
		writeFile(configData.Tor.TorrcPath, torrcData)
	}
	// Create tor control port path directory
	createFileDirectory(configData.Tor.ControlPortFilePath)
	// Create tor control auth cookie path directory
	createFileDirectory(configData.Tor.ControlAuthCookiePath)
	// Create tor hidden service private key path directory
	createFileDirectory(configData.Tor.HiddenServicePrivateKeyPath)
	// Create Gemini capsule file path directory
	createFileDirectory(configData.Gemini.DataPath + "/")
	if !fileExists(configData.Gemini.DataPath+"/index.gmi") &&
		!fileExists(configData.Gemini.DataPath+"/index.gemini") {
		writeFile(configData.Gemini.DataPath+"/index.gmi",
			[]byte("# Default Bergelmir Gemini Page\n\n"+
				"You should edit this page."))
	}
	// Create HTTP server file path directory
	createFileDirectory(configData.HTTP.DataPath + "/")
	if !fileExists(configData.HTTP.LayoutHTMLPath) {
		// Write default HTML Layout content to Layout HTML Path if it does
		// not already exist
		writeFile(configData.HTTP.LayoutHTMLPath, defaultHTTPHTMLLayoutContent)
	}
	// Create TLS cert path directory
	createFileDirectory(configData.Gemini.TLS.CertPath)
	// Create TLS key path directory
	createFileDirectory(configData.Gemini.TLS.KeyPath)
	generateConfigFile()
}
