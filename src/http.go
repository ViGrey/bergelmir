package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	HTTP_DEFAULT_PORT = 80
)

var (
	geminiContentRe = regexp.MustCompile("(?m)^\\s*%GEMINI_CONTENT%")
	titleRe         = regexp.MustCompile("%TITLE%")
)

func catchAll(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Path
	if len(url) > 0 {
		if url[len(url)-1] == '/' {
			url = url[:len(url)-1]
		}
	}
	urlExtension := filepath.Ext(url)
	if urlExtension == ".gmi" || urlExtension == ".html" || urlExtension == ".gemini" {
		url = url[:len(url)-len(urlExtension)]
		urlExtension = ""
	}
	if url == "" {
		url = "/index"
	}
	if urlExtension == "" {
		if !isRSSFeed(url) {
			content, pageTitle, exists := geminiToHTMLFileContent(configData.Gemini.DataPath +
				"/" + url)
			if exists {
				w.Header().Set("content-type", getMIMEType(".html"))
				w.WriteHeader(http.StatusOK)
				htmlLayoutContent, err := os.ReadFile(configData.HTTP.LayoutHTMLPath)
				handleErr(err, "Unable to read Layout HTML file")
				htmlLayoutContent = titleRe.ReplaceAll(htmlLayoutContent, pageTitle)
				htmlLayoutContent = geminiContentRe.ReplaceAll(htmlLayoutContent, content)
				w.Write(htmlLayoutContent)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		} else {
			w.Header().Set("content-type", "application/rss+xml")
			w.WriteHeader(http.StatusOK)
			rssHost := "http"
			if r.TLS != nil {
				rssHost += "s"
			}
			rssHost += "://" + r.Host
			w.Write([]byte(createRSSFeed(rssHost)))
			return
		}
	}
	handleHTTPFile(w, r, url)
}

func handleHTTPFile(w http.ResponseWriter, r *http.Request, path string) {
	geminiDataPath := configData.Gemini.DataPath + "/" + path
	httpDataPath := configData.HTTP.DataPath + "/" + path
	f, err := os.Open(geminiDataPath)
	if err == nil {
		defer f.Close()
		w.Header().Set("content-type", getMIMEType(path))
		w.WriteHeader(http.StatusOK)
		io.Copy(w, f)
		return
	}
	f, err = os.Open(httpDataPath)
	if err == nil {
		defer f.Close()
		w.Header().Set("content-type", getMIMEType(path))
		w.WriteHeader(http.StatusOK)
		io.Copy(w, f)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func geminiToHTMLFileContent(path string) (content, pageTitle []byte, exists bool) {
	var gmiContent []byte
	gmiContent, exists = getGemtextContent(path)
	content, pageTitle = translateGemtextToHTML(string(gmiContent))
	return
}

func escapeHTMLContent(content string) string {
	content = strings.ReplaceAll(content, "&", "&amp;")
	content = strings.ReplaceAll(content, "<", "&lt;")
	content = strings.ReplaceAll(content, ">", "&gt;")
	return content
}

func escapeHTMLQuotes(content string) string {
	content = strings.ReplaceAll(content, "'", "&#39;")
	content = strings.ReplaceAll(content, "\"", "&#34;")
	return content
}

func translateGemtextToHTML(gmi string) (html, pageTitle []byte) {
	pageTitle = []byte(configData.HTTP.DefaultPageTitle)
	htmlString := "<div id=\"content\">\n"
	gmi = strings.ReplaceAll(gmi, "\r\n", "\n")
	gmiLines := strings.Split(gmi, "\n")
	preformattedToggle := false
	pageTitleSet := false
	for _, line := range gmiLines {
		g := parseGemtextLine(line, preformattedToggle)
		switch g.lineType {
		case GEMTEXT_HEADING:
			htmlString += fmt.Sprintf("<h%d>%s</h%d>\n", g.level,
				escapeHTMLContent(g.text), g.level)
			if g.level == 1 && !pageTitleSet {
				pageTitle = []byte(escapeHTMLQuotes(g.text))
			}
		case GEMTEXT_LIST_ITEM:
			htmlString += fmt.Sprintf("<li>%s</li>\n", escapeHTMLContent(g.text))
		case GEMTEXT_QUOTE:
			htmlString += fmt.Sprintf("<blockquote>%s</blockquote>\n",
				escapeHTMLContent(g.text))
		case GEMTEXT_TEXT:
			if g.text == "" {
				htmlString += "<br />\n"
			} else {
				htmlString += fmt.Sprintf("<p>%s</p>\n", escapeHTMLContent(g.text))
			}
		case GEMTEXT_PREFORMATTED_TOGGLE:
			if !preformattedToggle {
				escapedAltText := escapeHTMLQuotes(escapeHTMLContent(g.altText))
				htmlString += fmt.Sprintf("<pre aria-label=\"%s\" title=\"%s\"><code>",
					escapedAltText, escapedAltText)
			} else {
				if strings.HasSuffix(htmlString, "\n") {
					htmlString = htmlString[:len(htmlString)-1]
				}
				htmlString += "</code></pre>\n"
			}
			preformattedToggle = !preformattedToggle
		case GEMTEXT_PREFORMATTED_TEXT:
			htmlString += fmt.Sprintf("%s\n", escapeHTMLContent(g.text))
		case GEMTEXT_LINK:
			urlData, _ := url.Parse(g.path)
			schemeText := ""
			if urlData.Scheme == "gemini" {
				schemeText += " [Gemini Protocol Link]"
			}
			target := ""
			if urlData.Host != "" {
				target = " target=\"_blank\""
			}
			switch {
			case urlData.Scheme == "" && urlData.Host == "" &&
				strings.HasPrefix(getMIMEType(g.path), "image/"):
				htmlString += fmt.Sprintf("<div class=\"img-container\">"+
					"<img src=\"%s\" alt=\"%s\" title=\"%s\"/></div>\n",
					escapeHTMLQuotes(escapeHTMLContent(g.path)),
					escapeHTMLQuotes(escapeHTMLContent(g.text)),
					escapeHTMLQuotes(escapeHTMLContent(g.text)))
			case g.text != "":
				htmlString += fmt.Sprintf("<a href=\"%s\"%s>%s</a>%s<br />\n",
					escapeHTMLQuotes(escapeHTMLContent(g.path)),
					target,
					escapeHTMLQuotes(escapeHTMLContent(g.text)),
					schemeText)
			default:
				linkPath := escapeHTMLQuotes(escapeHTMLContent(g.path))
				htmlString += fmt.Sprintf("<a href=\"%s\"%s>%s</a>%s<br />\n",
					linkPath, target, linkPath, schemeText)
			}
		}
	}
	html = []byte(htmlString + "</div>")
	return
}

func startHTTPServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", catchAll)
	network, location := parseLocation(configData.HTTP.ListeningLocation)
	srv := &http.Server{
		ReadTimeout: 5 * time.Second,
		Handler:     mux,
	}
	networkListener, err := net.Listen(network, location)
	handleErr(err, "Unable to start HTTP server")
	srv.Serve(networkListener)
}
