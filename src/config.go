package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

const (
	CONFIG_FILE_PATH = "config.yaml"
)

var (
	configData Config
)

type Config struct {
	BergelmirVersion string       `yaml:"bergelmir_version"`
	RSS              ConfigRSS    `yaml:"rss"`
	Tor              ConfigTor    `yaml:"tor"`
	Gemini           ConfigGemini `yaml:"gemini"`
	HTTP             ConfigHTTP   `yaml:"http"`
}

type ConfigRSS struct {
	Enabled              bool   `yaml:"enabled"`
	FeedSourceGeminiPath string `yaml:"feed_source_gemini_path"`
}

type ConfigTor struct {
	Enabled                     bool   `yaml:"enabled"`
	ControlPortFilePath         string `yaml:"control_port_file_path"`
	ControlAuthCookiePath       string `yaml:"control_auth_cookie_path"`
	HiddenServicePrivateKeyPath string `yaml:"hidden_service_private_key_path"`
	TorrcPath                   string `yaml:"torrc_path"`
}

type ConfigGemini struct {
	DomainNames       []string        `yaml:"domain_names"`
	DataPath          string          `yaml:"data_path"`
	ListeningLocation string          `yaml:"listening_location"`
	TLS               ConfigGeminiTLS `yaml:"tls"`
	Tor               ConfigGeminiTor `yaml:"tor"`
}

type ConfigGeminiTLS struct {
	KeyPath  string `yaml:"key_path"`
	CertPath string `yaml:"cert_path"`
}

type ConfigGeminiTor struct {
	VirtualPort int `yaml:"virtual_port"`
}

type ConfigHTTP struct {
	Enabled           bool          `yaml:"enabled"`
	ListeningLocation string        `yaml:"listening_location"`
	DataPath          string        `yaml:"data_path"`
	LayoutHTMLPath    string        `yaml:"layout_html_path"`
	DefaultPageTitle  string        `yaml:"default_page_title"`
	Tor               ConfigHTTPTor `yaml:"tor"`
}

type ConfigHTTPTor struct {
	VirtualPort int `yaml:"virtual_port"`
}

// Open config file and parse contents into configData
func parseConfigData() {
	if !fileExists(CONFIG_FILE_PATH) {
		fmt.Printf("Unable to find config file %s\n", CONFIG_FILE_PATH)
		fmt.Printf("Use `%s init` to create one.\n", bergelmirCmd)
		fmt.Println("Exiting...")
		os.Exit(1)
		// Create config file if file does not exist
		generateConfigFile()
		return
	}
	c, err := os.Open(CONFIG_FILE_PATH)
	handleErr(err, "Unable to open config file at "+CONFIG_FILE_PATH)
	defer c.Close()
	configData = Config{}
	handleErr(yaml.NewDecoder(c).Decode(&configData),
		"Unable to parse config file "+CONFIG_FILE_PATH)
}

// Write configData to config file
func generateConfigFile() {
	c, err := os.Create(CONFIG_FILE_PATH)
	defer c.Close()
	handleErr(err, "Unable to create config file at "+CONFIG_FILE_PATH)
	handleErr(yaml.NewEncoder(c).Encode(&configData),
		"Unable to write config file "+CONFIG_FILE_PATH)
}
