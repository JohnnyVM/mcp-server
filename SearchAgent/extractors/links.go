package extractors

import (
	"net/url"
	"strings"

	"github.com/johnnysvm/searchagent-mcp/internal/signals"
	"golang.org/x/net/html"
)

// ExtractedLink is a classified hyperlink found in an HTML page.
type ExtractedLink struct {
	URL     string
	Text    string
	Rel     string
	Snippet string // surrounding block text (useful for search result pages)
}

// ExtractLinks parses htmlStr and returns classified, deduplicated content links.
// pageURL is used to resolve relative URLs and detect external links.
func ExtractLinks(htmlStr, pageURL string) []ExtractedLink {
	base, _ := url.Parse(pageURL)
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil
	}

	pageHost := ""
	if base != nil {
		pageHost = base.Host
	}

	seen := make(map[string]int) // url -> index in results
	var results []ExtractedLink

	var walk func(*html.Node, int)
	walk = func(n *html.Node, skipDepth int) {
		if n.Type == html.ElementNode {
			tag := strings.ToLower(n.Data)
			if tag == "nav" || tag == "header" || tag == "footer" || tag == "aside" {
				skipDepth++
			}
			if tag == "a" && skipDepth == 0 {
				href := nodeAttr(n, "href")
				if href != "" && !strings.HasPrefix(href, "#") && !strings.HasPrefix(href, "javascript:") {
					if base != nil {
						if rel, err := url.Parse(href); err == nil {
							href = base.ResolveReference(rel).String()
						}
					}
					text := strings.TrimSpace(nodeText(n))
					if idx, exists := seen[href]; exists {
						// Prefer the anchor with longer (more descriptive) text
						if len(text) > len(results[idx].Text) {
							results[idx].Text = text
							results[idx].Snippet = blockSnippet(n)
						}
					} else if len(text) >= 3 {
						rel := signals.ClassifyLink(href, text, pageHost)
						snippet := blockSnippet(n)
						seen[href] = len(results)
						results = append(results, ExtractedLink{
							URL:     href,
							Text:    text,
							Rel:     rel,
							Snippet: snippet,
						})
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, skipDepth)
		}
	}
	walk(doc, 0)
	return results
}

// blockSnippet walks up through all block-level ancestors and returns the largest
// block text (truncated to 600 chars). Taking the largest ancestor rather than the
// innermost ensures we capture siblings like paper abstracts alongside the link text.
func blockSnippet(n *html.Node) string {
	blockTags := map[string]bool{
		"li": true, "article": true, "div": true, "section": true, "p": true, "td": true,
	}
	best := ""
	p := n.Parent
	for p != nil {
		if p.Type == html.ElementNode && blockTags[strings.ToLower(p.Data)] {
			text := strings.TrimSpace(nodeText(p))
			if len(text) > len(best) {
				best = text
			}
		}
		p = p.Parent
	}
	if len(best) > 600 {
		best = best[:600]
	}
	return best
}

func nodeAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}
