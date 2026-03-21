package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/johnnysvm/searchagent-mcp/internal/signals"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/net/html"
)

// FetchMetadataInput parameters for fetch_metadata.
type FetchMetadataInput struct {
	URL string `json:"url"`
}

// MetadataResult is the fetch_metadata response.
type MetadataResult struct {
	URL          string          `json:"url"`
	CanonicalURL string          `json:"canonical_url,omitempty"`
	Title        string          `json:"title,omitempty"`
	Description  string          `json:"description,omitempty"`
	ContentType  string          `json:"content_type,omitempty"`
	Author       string          `json:"author,omitempty"`
	PublishedAt  string          `json:"published_at,omitempty"`
	Keywords     []string        `json:"keywords,omitempty"`
	OG           ogData          `json:"og,omitempty"`
	JSONLD       any             `json:"json_ld,omitempty"`
	LinksPreview []linkPreview   `json:"links_preview,omitempty"`
}

type ogData struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
	Type        string `json:"type,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
}

type linkPreview struct {
	URL  string `json:"url"`
	Text string `json:"text"`
	Rel  string `json:"rel"`
}

const maxMetadataRead = 64 * 1024 // 64 KB — enough for any <head>

// NewFetchMetadata returns the fetch_metadata tool handler.
func NewFetchMetadata() func(context.Context, *mcp.CallToolRequest, FetchMetadataInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args FetchMetadataInput) (*mcp.CallToolResult, any, error) {
		client := &http.Client{Timeout: 10 * time.Second}
		httpReq, err := http.NewRequestWithContext(ctx, "GET", args.URL, nil)
		if err != nil {
			return toolError("invalid URL: " + err.Error()), nil, nil
		}
		httpReq.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
		httpReq.Header.Set("Accept", "text/html,*/*;q=0.8")

		resp, err := client.Do(httpReq)
		if err != nil {
			return toolError("fetch failed: " + err.Error()), nil, nil
		}
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, maxMetadataRead))
		bodyStr := string(bodyBytes)

		// Truncate at </head> to avoid processing body content
		if idx := strings.Index(strings.ToLower(bodyStr), "</head>"); idx > 0 {
			bodyStr = bodyStr[:idx+7]
		}

		result := MetadataResult{
			URL:         args.URL,
			ContentType: resp.Header.Get("Content-Type"),
		}
		parseHTMLMeta(bodyStr, args.URL, &result)

		data, _ := json.Marshal(result)
		return toolText(string(data)), nil, nil
	}
}

// parseHTMLMeta tokenizes the head section and extracts all metadata.
func parseHTMLMeta(headHTML, pageURL string, result *MetadataResult) {
	base, _ := url.Parse(pageURL)
	pageHost := ""
	if base != nil {
		pageHost = base.Host
	}

	tokenizer := html.NewTokenizer(strings.NewReader(headHTML))

	var inTitle bool
	var inScript bool
	var scriptContent strings.Builder

	// Track current <a> for anchor text
	var pendingLink *linkPreview
	var anchorText strings.Builder

	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			return

		case html.StartTagToken, html.SelfClosingTagToken:
			tok := tokenizer.Token()
			tag := strings.ToLower(tok.Data)

			switch tag {
			case "title":
				inTitle = true
			case "meta":
				processMeta(tok, result)
			case "link":
				processLinkTag(tok, base, result)
			case "script":
				if attrValTok(tok, "type") == "application/ld+json" {
					inScript = true
					scriptContent.Reset()
				}
			case "a":
				href := attrValTok(tok, "href")
				if href != "" && !strings.HasPrefix(href, "#") && !strings.HasPrefix(href, "javascript:") {
					if base != nil {
						if rel, err := url.Parse(href); err == nil {
							href = base.ResolveReference(rel).String()
						}
					}
					pendingLink = &linkPreview{URL: href}
					anchorText.Reset()
				}
			}

		case html.EndTagToken:
			tok := tokenizer.Token()
			switch strings.ToLower(tok.Data) {
			case "title":
				inTitle = false
			case "script":
				if inScript {
					var v any
					if err := json.Unmarshal([]byte(scriptContent.String()), &v); err == nil {
						result.JSONLD = v
					}
					inScript = false
				}
			case "a":
				if pendingLink != nil {
					pendingLink.Text = strings.TrimSpace(anchorText.String())
					pendingLink.Rel = signals.ClassifyLink(pendingLink.URL, pendingLink.Text, pageHost)
					result.LinksPreview = append(result.LinksPreview, *pendingLink)
					pendingLink = nil
				}
			}

		case html.TextToken:
			text := string(tokenizer.Text())
			if inTitle && result.Title == "" {
				result.Title = strings.TrimSpace(text)
			}
			if inScript {
				scriptContent.WriteString(text)
			}
			if pendingLink != nil {
				anchorText.WriteString(text)
			}
		}
	}
}

func processMeta(tok html.Token, result *MetadataResult) {
	name := strings.ToLower(attrValTok(tok, "name"))
	property := strings.ToLower(attrValTok(tok, "property"))
	content := attrValTok(tok, "content")

	switch name {
	case "description":
		result.Description = content
	case "author":
		result.Author = content
	case "keywords":
		for _, k := range strings.Split(content, ",") {
			if k = strings.TrimSpace(k); k != "" {
				result.Keywords = append(result.Keywords, k)
			}
		}
	case "article:published_time", "date", "publishdate":
		result.PublishedAt = content
	}

	switch property {
	case "og:title":
		result.OG.Title = content
	case "og:description":
		result.OG.Description = content
	case "og:image":
		result.OG.Image = content
	case "og:type":
		result.OG.Type = content
	case "og:site_name":
		result.OG.SiteName = content
	case "article:published_time":
		result.PublishedAt = content
	}
}

func processLinkTag(tok html.Token, base *url.URL, result *MetadataResult) {
	if strings.ToLower(attrValTok(tok, "rel")) != "canonical" {
		return
	}
	href := attrValTok(tok, "href")
	if href == "" {
		return
	}
	if base != nil {
		if u, err := url.Parse(href); err == nil {
			href = base.ResolveReference(u).String()
		}
	}
	result.CanonicalURL = href
}

func attrValTok(tok html.Token, key string) string {
	for _, a := range tok.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
