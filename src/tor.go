package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/sha3"
)

var (
	torAddress               string
	torControlConn           net.Conn
	serverNonceRe            = regexp.MustCompile(" SERVERNONCE=([0-9A-Fa-f]+)")
	serviceIDRe              = regexp.MustCompile("250-ServiceID=([2-7A-Za-z]+)")
	torControlPortRe         = regexp.MustCompile("PORT=(.+)")
	torControlServerNonce    []byte
	torCmd                   *exec.Cmd
	torConnected             chan bool
	torGetControlServerNonce chan bool
)

const (
	TOR_HMAC_SECRET = "Tor safe cookie authentication controller-to-server hash"
)

func getHiddenServiceV3PrivKey() string {
	var privKey []byte
	content, err := os.ReadFile(configData.Tor.HiddenServicePrivateKeyPath)
	// If hidden_service_private_key_path is invalid or can't be read,
	if err != nil || len(content) < 96 {
		var pubKey []byte
		pubKey, privKey = generateHiddenServiceV3PubPrivKey()
		createHiddenServiceV3PrivKeyFile(privKey)
		torAddress = encodeHiddenServicePublicKey(pubKey)
		// Generate tls cert for hidden service
	} else {
		privKey = content[32:96]
	}
	return base64.StdEncoding.EncodeToString(privKey)
}

func createHiddenServiceV3PrivKeyFile(privKey []byte) {
	keyFileContent := append([]byte("== ed25519v1-secret: type0 ==\x00\x00\x00"),
		append(privKey, 0x0a)...)
	err := os.WriteFile(configData.Tor.HiddenServicePrivateKeyPath,
		keyFileContent, 0600)
	if err != nil {
		return
	}
}

// Generate a Tor Hidden Service v3 Private Key using the method
// mentioned in section A.2. of rend-spec-v3.txt at
// https://gitweb.torproject.org/torspec.git/tree/rend-spec-v3.txt
func generateHiddenServiceV3PubPrivKey() (pubKey, privKey []byte) {
	var err error
	pubKey, privKey, err = ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}
	privKeyHash := sha512.Sum512(privKey[:32])
	privKeyHash[0] &= 248
	privKeyHash[31] &= 63
	privKeyHash[31] |= 64

	return pubKey, privKeyHash[:]
}

func getTorControlPort() (port string) {
	for i := 0; i < 5; i++ {
		fileExists(configData.Tor.ControlPortFilePath)
		time.Sleep(100 * time.Millisecond)
	}
	port = "127.0.0.1:9051"
	controlPortFileContent, err := os.ReadFile(configData.Tor.ControlPortFilePath)
	handleErr(err, "Unable to open tor control port file "+
		configData.Tor.ControlPortFilePath)
	torControlPortMatch := torControlPortRe.FindStringSubmatch(
		string(controlPortFileContent))
	if len(torControlPortMatch) > 1 {
		port = torControlPortMatch[1]
	}
	return
}

func connectToTor() (err error) {
	torControlPort := getTorControlPort()
	torControlConn, err = net.Dial("tcp", torControlPort)
	handleErr(err, "Unable to connect to Tor control port")
	go readTorConn()
	torAuth()
	return
}

func readTorConn() {
	//torControlConnOut := ""
	defer torControlConn.Close()
	for {
		buf := make([]byte, 1024)
		n, err := torControlConn.Read(buf)
		if err != nil || n == 0 {
			break
		}
		switch {
		case bytes.Contains(buf[:n], []byte(" SERVERNONCE=")):
			torControlServerNonceMatch := serverNonceRe.FindStringSubmatch(string(buf[:n]))
			if len(torControlServerNonceMatch) > 1 {
				torControlServerNonce, _ = hex.DecodeString(torControlServerNonceMatch[1])
				torGetControlServerNonce <- true
			}
		case bytes.Contains(buf[:n], []byte("250-ServiceID=")):
			torServiceIDMatch := serviceIDRe.FindStringSubmatch(string(buf[:n]))
			if len(torServiceIDMatch) > 1 {
				torAddress = torServiceIDMatch[1] + ".onion"
				torConnected <- true
				return
			}
		}
	}
}

func torAuth() {
	// Cookie hash, Client nonce, Server nonce
	clientNonce := make([]byte, 32)
	rand.Read(clientNonce)
	torGetControlServerNonce = make(chan bool)
	_, err := torControlConn.Write([]byte("AUTHCHALLENGE SAFECOOKIE " + hex.EncodeToString(clientNonce) + "\n"))
	if err != nil {
		torControlConn.Close()
	}
	<-torGetControlServerNonce
	cookieString, err := os.ReadFile(configData.Tor.ControlAuthCookiePath)
	if err != nil {
		return
	}
	authHmac := hmac.New(sha256.New, []byte(TOR_HMAC_SECRET))
	authHmac.Write(cookieString)
	authHmac.Write(clientNonce)
	authHmac.Write(torControlServerNonce)
	torControlConn.Write([]byte("AUTHENTICATE " + hex.EncodeToString(authHmac.Sum(nil)) + "\n"))
	privKey := getHiddenServiceV3PrivKey()
	addOnionString := ("ADD_ONION ED25519-V3:" + privKey +
		" Flags=DiscardPK,Detach Port=" +
		strconv.Itoa(configData.Gemini.Tor.VirtualPort) + "," +
		configData.Gemini.ListeningLocation + " Port=" +
		strconv.Itoa(configData.HTTP.Tor.VirtualPort) + "," +
		configData.HTTP.ListeningLocation + "\n")
	torControlConn.Write([]byte(addOnionString))
}

func encodeHiddenServicePublicKey(pubKey []byte) string {
	checksum := sha3.Sum256(append([]byte(".onion checksum"), append(pubKey,
		0x03)...))
	address := strings.ToLower(base32.StdEncoding.EncodeToString(append(pubKey,
		append(checksum[:2], 0x03)...))) + ".onion"
	return address
}

/*
	err = StartTor()
	if err == nil {
		get ControlPort from configData.ControlPortFilePath

		ControlPortFilePath         string `yaml:"control_port_file_path"`
		ControlAuthCookiePath       string `yaml:"control_auth_cookie_path"`
	} else {
		handle err
	}
*/

// Start Tor
func startTor() {
	torCmd = exec.Command("tor", "-f", configData.Tor.TorrcPath)
	handleErr(torCmd.Start(), "Unable to start tor.  Is tor installed on "+
		"your system?")
}

func killTor() {
	if torCmd != nil {
		if torCmd.Process != nil {
			torCmd.Process.Kill()
		}
	}
}
