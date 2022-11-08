package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

const (
	GEMINI_DEFAULT_PORT                = 1965
	STATUS_INPUT                       = 10
	STATUS_SENSITIVE_INPUT             = 11
	STATUS_SUCCESS                     = 20
	STATUS_REDIRECT_TEMPORARY          = 30
	STATUS_REDIRECT_PERMANENT          = 31
	STATUS_TEMPORARY_FAILURE           = 40
	STATUS_SERVER_UNAVAILABLE          = 41
	STATUS_CGI_ERROR                   = 42
	STATUS_PROXY_ERROR                 = 43
	STATUS_SLOW_DOWN                   = 44
	STATUS_PERMANENT_FAILURE           = 50
	STATUS_NOT_FOUND                   = 51
	STATUS_GONE                        = 52
	STATUS_PROXY_REQUEST_REFUSED       = 53
	STATUS_BAD_REQUEST                 = 59
	STATUS_CLIENT_CERTIFICATE_REQUIRED = 60
	STATUS_CERTIFICATE_NOT_AUTHORISED  = 61
	STATUS_CERTIFICATE_NOT_VALID       = 62
)

var (
	geminiHostList = []string{}
)

// Write Gemini Response Header to client (3.1 of specification.gmi)
func sendGeminiResponseHeader(conn net.Conn, status int, meta string) error {
	_, err := conn.Write([]byte(strconv.Itoa(status) + " " + meta + "\r\n"))
	return err
}

// Write Gemini Response Body to client (3.3 of specification.gmi)
func sendGeminiResponseBody(conn net.Conn, dataWriter io.Reader) {
	io.Copy(conn, dataWriter)
}

// Handle Gemini request
func handleGeminiRequest(conn net.Conn, addr string) {
	if !utf8.ValidString(addr) {
		// Invalid Gemini Request for at least one of the following reasons:
		// * Not UTF-8 encoded
		sendGeminiResponseHeader(conn, STATUS_BAD_REQUEST, "Invalid Request")
		return
	}
	urlCRLFIndex := strings.Index(addr, "\r\n")
	if urlCRLFIndex < 0 || urlCRLFIndex != len(addr)-2 {
		// Invalid Gemini Request for at least one of the following reasons:
		// * No <CR><LF> in request
		// * Last 2 bytes of request were not the first (and only) instance of <CR><LF>
		return
	}
	if urlCRLFIndex > 1024 || urlCRLFIndex != len(addr)-2 {
		// Invalid Gemini Request for at least one of the following reasons:
		// * More than 1024 byte long request before <CR><LF>
		sendGeminiResponseHeader(conn, STATUS_BAD_REQUEST, "Invalid Request")
		return
	}
	addr = addr[:len(addr)-2]
	urlBOMIndex := strings.Index(addr, "\uFEFF")
	if urlBOMIndex == 0 {
		// Invalid Gemini Request for at least one of the following reasons:
		// * Request started with UTF-8 encoded byte order mark (U+FEFF)
		sendGeminiResponseHeader(conn, STATUS_BAD_REQUEST, "Invalid Request")
		return
	}
	u, err := url.Parse(addr)
	if err != nil {
		// Invalid Gemini Request for at least one of the following reasons:
		// * Invalid URL structure
		sendGeminiResponseHeader(conn, STATUS_BAD_REQUEST, "Invalid Request")
		return
	}
	if u.Hostname() == "" || u.Scheme == "" {
		// Invalid Gemini Request for at least one of the following reasons:
		// * No URL hostname
		// * No URL scheme
		sendGeminiResponseHeader(conn, STATUS_BAD_REQUEST, "Invalid Request")
		return
	}
	urlRelPath, err := filepath.Rel(u.Hostname(), u.Hostname()+u.Path)
	if err != nil || strings.Index(urlRelPath, "..") == 0 {
		// Invalid Gemini Request for at least one of the following reasons:
		// * Path is not within root
		sendGeminiResponseHeader(conn, STATUS_BAD_REQUEST, "Invalid Request")
		return
	}
	u = u.ResolveReference(u)
	if strings.ToLower(u.Scheme) != "gemini" {
		// Reject Gemini Request for at least one of the following reasons:
		// * URL scheme is not gemini
		sendGeminiResponseHeader(conn, STATUS_PROXY_REQUEST_REFUSED, "Invalid Scheme")
		return
	}
	serverPort, validHost := getGeminiHostPortValid(u.Hostname())
	if !validHost {
		// Reject Gemini Request for at least one of the following reasons:
		// * Hostname is not valid
		sendGeminiResponseHeader(conn, STATUS_PROXY_REQUEST_REFUSED, "Invalid Host")
	} else {
		uPort := u.Port()
		if uPort == "" {
			uPort = strconv.Itoa(GEMINI_DEFAULT_PORT)
		}
		if uPort != serverPort {
			// Reject Gemini Request for at least one of the following reasons:
			// * Port is not valid
			sendGeminiResponseHeader(conn, STATUS_PROXY_REQUEST_REFUSED, "Invalid Port")
		} else {
			handleGeminiResponseBody(conn, u.Path, u.Host)
		}
	}
}

// Handle Gemini request
func handleGeminiResponseBody(conn net.Conn, urlPath, host string) {
	if len(urlPath) > 0 {
		if urlPath[len(urlPath)-1] == '/' {
			urlPath = urlPath[:len(urlPath)-1]
		}
	}
	urlExtension := filepath.Ext(urlPath)
	if urlExtension == ".gmi" || urlExtension == ".gemini" {
		urlPath = urlPath[:len(urlPath)-len(urlExtension)]
		urlExtension = ""
	}
	if urlPath == "" {
		urlPath = "/index"
	}
	geminiDataPath := configData.Gemini.DataPath + "/" + urlPath
	if urlExtension == "" {
		if !isRSSFeed(urlPath) {
			mimeType := "text/gemini"
			content, exists := getGemtextContent(geminiDataPath)
			if exists {
				err := sendGeminiResponseHeader(conn, STATUS_SUCCESS, mimeType)
				if err == nil {
					conn.Write(content)
				}
			} else {
				sendGeminiResponseHeader(conn, STATUS_NOT_FOUND, "Page Not Found")
			}
		} else {
			mimeType := "application/rss+xml"
			if sendGeminiResponseHeader(conn, STATUS_SUCCESS, mimeType) != nil {
				return
			}
			conn.Write([]byte(createRSSFeed("gemini://" + host)))
		}
	} else {
		handleGeminiServeFile(conn, geminiDataPath)
	}
}

func handleGeminiServeFile(conn net.Conn, path string) {
	mimeType := getMIMEType(path)
	f, err := os.Open(path)
	if err != nil {
		sendGeminiResponseHeader(conn, STATUS_NOT_FOUND, "Page Not Found")
		return
	}
	defer f.Close()
	err = sendGeminiResponseHeader(conn, STATUS_SUCCESS, mimeType)
	if err != nil {
		return
	}
	sendGeminiResponseBody(conn, f)
}

// Get Gemtext content from <path>.gmi or <path>.gemini
func getGemtextContent(path string) (content []byte, exists bool) {
	var err error
	geminiExtensions := []string{".gmi", ".gemini"}
	for _, extension := range geminiExtensions {
		content, err = os.ReadFile(path + extension)
		if err == nil {
			exists = true
		}
		if exists {
			break
		}
	}
	return
}

// Start Gemini capsule
func startGeminiServer() {
	cert := loadTLSCert()
	geminiHostList = getDomainList()
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
	network, location := parseLocation(configData.Gemini.ListeningLocation)
	if network == "unix" {
		syscall.Unlink(location)
	}
	ln, err := tls.Listen(network, location, tlsConfig)
	handleErr(err, fmt.Sprintf("Unable to create Gemini capsule network at %s", configData.Gemini.ListeningLocation))
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		go handleGeminiConnection(conn)
	}
}

// Handle client connection to Gemini capsule
func handleGeminiConnection(conn net.Conn) {
	defer conn.Close()
	rBuf := make([]byte, 2048)
	n, err := conn.Read(rBuf)
	if err != nil {
		return
	}
	handleGeminiRequest(conn, string(rBuf[:n]))
}

// Get port of gemini host requested along with if the host is valid
func getGeminiHostPortValid(requestHost string) (port string, validHost bool) {
	for _, host := range geminiHostList {
		if strings.ToLower(requestHost) == host {
			validHost = true
			if host == torAddress {
				port = strconv.Itoa(configData.Gemini.Tor.VirtualPort)
				break
			}
			urlPath := "//" + configData.Gemini.ListeningLocation
			serverLocation, err := url.Parse(urlPath)
			if err == nil {
				port = serverLocation.Port()
			}
			break
		}
	}
	return
}
