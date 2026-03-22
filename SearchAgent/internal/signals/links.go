// Package signals provides signal processing utilities for the SearchAgent.
package signals

import (
	"net/url"
	"path"
	"slices"
	"strings"
)

var downloadExts = map[string]bool{
	".pdf": true, ".docx": true, ".doc": true,
	".xlsx": true, ".xls": true, ".pptx": true, ".ppt": true,
	".zip": true, ".tar": true, ".gz": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".mp4": true, ".mp3": true,
}

var paginationPhrases = []string{"next page", "next →", "next »", "page 2", "load more", "show more"}
var relatedPhrases = []string{"see also", "related", "similar", "you might", "read more", "further reading", "learn more"}
var navPhrases = []string{"home", "about", "contact", "privacy", "terms", "login", "sign in", "register", "subscribe", "menu"}

// ClassifyLink returns the rel type for a hyperlink.
// pageHost is the hostname of the page that contains the link.
func ClassifyLink(rawURL, anchorText, pageHost string) string {
	text := strings.ToLower(strings.TrimSpace(anchorText))

	u, err := url.Parse(rawURL)
	if err != nil {
		return "external"
	}

	// Download by file extension
	ext := strings.ToLower(path.Ext(u.Path))
	if downloadExts[ext] {
		return "download"
	}

	// Pagination by text or URL pattern
	for _, hint := range paginationPhrases {
		if strings.Contains(text, hint) {
			return "pagination"
		}
	}
	if strings.Contains(u.RawQuery, "page=") || strings.Contains(u.Path, "/page/") {
		return "pagination"
	}

	// Navigation boilerplate
	if slices.Contains(navPhrases, text) {
		return "navigation"
	}

	// Related content
	for _, hint := range relatedPhrases {
		if strings.Contains(text, hint) {
			return "related"
		}
	}

	// External if different host
	if u.Host != "" && u.Host != pageHost {
		return "external"
	}

	return "related"
}
