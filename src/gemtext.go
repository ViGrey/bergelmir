package main

import (
	"strings"
)

const (
	GEMTEXT_TEXT int = iota
	GEMTEXT_LINK
	GEMTEXT_PREFORMATTED_TOGGLE
	GEMTEXT_PREFORMATTED_TEXT
	GEMTEXT_HEADING
	GEMTEXT_LIST_ITEM
	GEMTEXT_QUOTE
)

type gemtextLine struct {
	lineType int
	text     string
	path     string
	altText  string
	level    int
}

func parseGemtextLine(line string, preformattedToggle bool) (g gemtextLine) {
	if !preformattedToggle {
		switch {
		// Check if preformatted toggle (5.4.3 of specification.gmi)
		case strings.HasPrefix(line, "```"):
			g.lineType = GEMTEXT_PREFORMATTED_TOGGLE
			g.altText = strings.TrimSpace(line[3:])
		// Check if invalid heading
		case strings.HasPrefix(line, "####"):
			g.lineType = GEMTEXT_TEXT
			g.text = strings.TrimSpace(line)
		// Check if heading level 3 (5.5.1 of specification.gmi)
		case strings.HasPrefix(line, "###"):
			g.lineType = GEMTEXT_HEADING
			g.level = 3
			g.text = strings.TrimSpace(line[3:])
		// Check if heading level 2 (5.5.1 of specification.gmi)
		case strings.HasPrefix(line, "##"):
			g.lineType = GEMTEXT_HEADING
			g.level = 2
			g.text = strings.TrimSpace(line[2:])
		// Check if heading level 1 (5.5.1 of specification.gmi)
		case strings.HasPrefix(line, "#"):
			g.lineType = GEMTEXT_HEADING
			g.level = 1
			g.text = strings.TrimSpace(line[1:])
		// Check if list item (5.5.2 of specification.gmi)
		case strings.HasPrefix(line, "* "):
			g.lineType = GEMTEXT_LIST_ITEM
			g.text = strings.TrimSpace(line[2:])
		// Check if link (5.4.2 of specification.gmi)
		case strings.HasPrefix(line, "=> ") || strings.HasPrefix(line, "=>\t"):
			g.lineType = GEMTEXT_LINK
			lineFields := strings.Fields(strings.TrimSpace(line[3:]))
			if len(lineFields) > 0 {
				g.path = lineFields[0]
				g.text = strings.TrimSpace(line[3+len(lineFields[0]):])
			}
		// Check if quote (5.5.3 of specification.gmi)
		case strings.HasPrefix(line, ">"):
			g.lineType = GEMTEXT_QUOTE
			g.text = strings.TrimSpace(line[1:])
		//
		default:
			g.lineType = GEMTEXT_TEXT
			g.text = strings.TrimSpace(line)
		}
	} else {
		switch {
		// Check if preformatted text (5.4.3 of specification.gmi)
		case strings.HasPrefix(line, "```"):
			g.lineType = GEMTEXT_PREFORMATTED_TOGGLE
		// Preformatted text (5.4.4 of specification.gmi)
		default:
			g.lineType = GEMTEXT_PREFORMATTED_TEXT
			g.text = line
		}
	}
	return
}
