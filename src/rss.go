package main

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var (
	geminiFeedRe = regexp.MustCompile(`(\d{4}-\d{2}-\d{2})\s(?:[-|–|—|―|‖|:|\|]\s)(.*)`)
)

type feedEntry struct {
	title   string
	link    string
	pubDate string
}

// Check if rss value in config is enabled and if url is /feed or /rss
func isRSSFeed(url string) bool {
	return (configData.RSS.Enabled &&
		(url == "/feed" || url == "/rss"))
}

func createRSSFeed(host string) string {
	rssGeminiDataPath := configData.Gemini.DataPath + "/" +
		configData.RSS.FeedSourceGeminiPath
	gmiContent, exists := getGemtextContent(rssGeminiDataPath)
	if exists {
		return translateGemtextToRSS(string(gmiContent), host)
	}
	return ""
}

func translateGemtextToRSS(gmi, host string) (rssFeedString string) {
	rssFeedURL := joinPath(host,
		configData.RSS.FeedSourceGeminiPath)
	rssFeedEntries := []feedEntry{}
	feedTitle := ""
	gmi = strings.ReplaceAll(gmi, "\r\n", "\n")
	gmiLines := strings.Split(gmi, "\n")
	preformattedToggle := false
	feedTitleSet := false
	for _, line := range gmiLines {
		g := parseGemtextLine(line, preformattedToggle)
		switch g.lineType {
		case GEMTEXT_HEADING:
			if g.level == 1 && !feedTitleSet {
				feedTitle = escapeHTMLQuotes(escapeHTMLContent(g.text))
			}
		case GEMTEXT_PREFORMATTED_TOGGLE:
			preformattedToggle = !preformattedToggle
		case GEMTEXT_LINK:
			entry, valid := parseTextToFeedEntry(g.text)
			if valid {
				gemtextPathURL, _ := url.Parse(g.path)
				if !gemtextPathURL.IsAbs() {
					entry.link = joinPath(host, g.path)
				} else {
					entry.link = g.path
				}
				rssFeedEntries = append(rssFeedEntries, entry)
			}
		}
	}
	sort.Slice(rssFeedEntries, func(i, j int) bool {
		return rssFeedEntries[i].pubDate > rssFeedEntries[j].pubDate
	})
	if len(rssFeedEntries) > 0 {
		rssFeedString = fmt.Sprintf(
			"<?xml version=\"1.0\" encoding=\"utf-8\"?>\n"+
				"<feed xmlns=\"http://www.w3.org/2005/Atom\">\n\n"+
				"  <title>%s</title>\n"+
				"  <link href=\"%s\"/>\n"+
				"  <updated>%s</updated>\n"+
				"  <id>%s</id>\n\n", feedTitle, rssFeedURL,
			rssFeedEntries[0].pubDate, rssFeedURL)
		for _, entry := range rssFeedEntries {
			rssFeedString += fmt.Sprintf(
				"  <entry>\n"+
					"    <title>%s</title>\n"+
					"    <link rel=\"alternate\" href=\"%s\"/>\n"+
					"    <id>%s</id>\n"+
					"    <updated>%s</updated>\n"+
					"  </entry>\n\n", entry.title, entry.link,
				entry.link, entry.pubDate)
		}
		rssFeedString += "</feed>"
	}
	return
}

func parseTextToFeedEntry(text string) (entry feedEntry, valid bool) {
	geminiFeedMatch := geminiFeedRe.FindStringSubmatch(text)
	if len(geminiFeedMatch) > 0 {
		return feedEntry{pubDate: geminiFeedMatch[1] + "T12:00:00Z",
			title: escapeHTMLQuotes(escapeHTMLContent(geminiFeedMatch[2]))}, true
	} else {
		return
	}
}

func joinPath(base, elem string) string {
	for len(base) > 0 && strings.LastIndex(base, "/") == len(base)-1 {
		base = base[:len(base)-1]
	}
	for strings.LastIndex(elem, "/") == 0 {
		elem = elem[1:]
	}
	return base + "/" + elem
}
