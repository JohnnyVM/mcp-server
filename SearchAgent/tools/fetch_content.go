package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/johnnysvm/searchagent-mcp/extractors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// FetchContentInput parameters for fetch_content.
type FetchContentInput struct {
	URL string `json:"url"`
}

// contentLink is a classified link found on the fetched page.
type contentLink struct {
	URL  string `json:"url"`
	Text string `json:"text"`
	Rel  string `json:"rel"`
}

// textContentResult is the JSON payload for HTML/PDF/DOCX responses.
type textContentResult struct {
	Content      string        `json:"content"`
	ContentType  string        `json:"content_type"`
	Title        string        `json:"title,omitempty"`
	CanonicalURL string        `json:"canonical_url,omitempty"`
	Links        []contentLink `json:"links"`
}

// imageContentMeta is the JSON text payload accompanying an ImageContent response.
type imageContentMeta struct {
	ContentType  string        `json:"content_type"`
	Title        string        `json:"title,omitempty"`
	CanonicalURL string        `json:"canonical_url,omitempty"`
	Links        []contentLink `json:"links"`
}

// NewFetchContent returns the fetch_content tool handler.
func NewFetchContent() func(context.Context, *mcp.CallToolRequest, FetchContentInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args FetchContentInput) (*mcp.CallToolResult, any, error) {
		ct, err := headContentType(ctx, args.URL)
		if err != nil {
			// Fall back to URL extension guessing
			ct = guessContentType(args.URL)
		}

		switch {
		case isHTML(ct):
			return fetchHTMLContent(ctx, args.URL)
		case strings.Contains(ct, "application/pdf") || strings.HasSuffix(strings.ToLower(args.URL), ".pdf"):
			return fetchPDFContent(ctx, args.URL)
		case strings.HasPrefix(ct, "image/") || isImageURL(args.URL):
			return fetchImageContent(ctx, args.URL)
		case isDOCX(ct) || isDOCXURL(args.URL):
			return fetchDOCXContent(ctx, args.URL)
		case strings.Contains(ct, "text/plain"):
			return fetchPlainContent(ctx, args.URL)
		default:
			// Try as HTML by default
			return fetchHTMLContent(ctx, args.URL)
		}
	}
}

func headContentType(ctx context.Context, rawURL string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "HEAD", rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", extractors.BrowserUA)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	return resp.Header.Get("Content-Type"), nil
}

func fetchHTMLContent(ctx context.Context, rawURL string) (*mcp.CallToolResult, any, error) {
	htmlStr, err := extractors.FetchHTML(ctx, rawURL)
	if err != nil {
		return toolError("fetch failed: " + err.Error()), nil, nil
	}

	markdown := extractors.HTMLToMarkdown(htmlStr)
	links := extractors.ExtractLinks(htmlStr, rawURL)

	var cls []contentLink
	for _, l := range links {
		cls = append(cls, contentLink{URL: l.URL, Text: l.Text, Rel: l.Rel})
	}

	result := textContentResult{
		Content:      markdown,
		ContentType:  "html",
		Title:        htmlTitle(htmlStr),
		CanonicalURL: rawURL,
		Links:        cls,
	}
	data, _ := json.Marshal(result)
	return toolText(string(data)), nil, nil
}

func fetchPDFContent(ctx context.Context, rawURL string) (*mcp.CallToolResult, any, error) {
	data, _, err := extractors.HTTPFetchBytes(ctx, rawURL)
	if err != nil {
		return toolError("fetch failed: " + err.Error()), nil, nil
	}
	text, err := extractors.ExtractPDF(data)
	if err != nil {
		return toolError("PDF extraction failed: " + err.Error()), nil, nil
	}
	result := textContentResult{
		Content:     text,
		ContentType: "pdf",
		Links:       []contentLink{},
	}
	out, _ := json.Marshal(result)
	return toolText(string(out)), nil, nil
}

func fetchImageContent(ctx context.Context, rawURL string) (*mcp.CallToolResult, any, error) {
	img, err := extractors.FetchImage(ctx, rawURL)
	if err != nil {
		return toolError("image fetch failed: " + err.Error()), nil, nil
	}

	meta := imageContentMeta{
		ContentType: img.MIMEType,
		Links:       []contentLink{},
	}
	metaJSON, _ := json.Marshal(meta)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(metaJSON)},
			&mcp.ImageContent{Data: img.Data, MIMEType: img.MIMEType},
		},
	}, nil, nil
}

func fetchDOCXContent(ctx context.Context, rawURL string) (*mcp.CallToolResult, any, error) {
	data, _, err := extractors.HTTPFetchBytes(ctx, rawURL)
	if err != nil {
		return toolError("fetch failed: " + err.Error()), nil, nil
	}
	text, err := extractors.ExtractDOCX(data)
	if err != nil {
		return toolError("DOCX extraction failed: " + err.Error()), nil, nil
	}
	result := textContentResult{
		Content:     text,
		ContentType: "docx",
		Links:       []contentLink{},
	}
	out, _ := json.Marshal(result)
	return toolText(string(out)), nil, nil
}

func fetchPlainContent(ctx context.Context, rawURL string) (*mcp.CallToolResult, any, error) {
	data, _, err := extractors.HTTPFetchBytes(ctx, rawURL)
	if err != nil {
		return toolError("fetch failed: " + err.Error()), nil, nil
	}
	result := textContentResult{
		Content:     string(data),
		ContentType: "text",
		Links:       []contentLink{},
	}
	out, _ := json.Marshal(result)
	return toolText(string(out)), nil, nil
}

func isHTML(ct string) bool {
	return ct == "" || strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml")
}

func isDOCX(ct string) bool {
	return strings.Contains(ct, "officedocument.wordprocessingml") ||
		strings.Contains(ct, "application/msword")
}

func isImageURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".svg"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func isDOCXURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.HasSuffix(lower, ".docx") || strings.HasSuffix(lower, ".doc")
}
