package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/johnnysvm/searchagent-mcp/extractors"
	"github.com/johnnysvm/searchagent-mcp/internal/registry"
	"github.com/johnnysvm/searchagent-mcp/internal/signals"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// QuerySourceInput parameters for query_source.
type QuerySourceInput struct {
	Source string `json:"source"`
	Query  string `json:"query"`
}

// SearchResult is one item in the query_source response.
type SearchResult struct {
	URL            string  `json:"url"`
	Title          string  `json:"title"`
	Snippet        string  `json:"snippet"`
	ContentType    string  `json:"content_type"`
	RelevanceScore float64 `json:"relevance_score"`
}

// QuerySourceOutput is the full query_source response.
type QuerySourceOutput struct {
	Results    []SearchResult `json:"results"`
	TotalFound int            `json:"total_found"`
	HasMore    bool           `json:"has_more"`
	QueryUsed  string         `json:"query_used"`
}

// NewQuerySource returns the query_source tool handler.
func NewQuerySource(reg *registry.Registry) func(context.Context, *mcp.CallToolRequest, QuerySourceInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args QuerySourceInput) (*mcp.CallToolResult, any, error) {
		src, ok := reg.ByName(args.Source)
		if !ok {
			return toolError(fmt.Sprintf("source %q not found; call list_sources() to see available names", args.Source)), nil, nil
		}

		queryURL := strings.ReplaceAll(src.QueryURL, "{query}", url.QueryEscape(args.Query))

		out := QuerySourceOutput{QueryUsed: args.Query}
		var err error

		switch src.ResultType {
		case "json-api":
			out.Results, out.TotalFound, out.HasMore, err = fetchJSONAPI(ctx, queryURL, src, args.Query)
			if err != nil {
				return toolError(fmt.Sprintf("fetching %s: %v", src.Name, err)), nil, nil
			}

		case "html-links":
			out.Results, err = fetchHTMLLinks(ctx, queryURL, args.Query)
			if err != nil {
				return toolError(fmt.Sprintf("fetching %s: %v", src.Name, err)), nil, nil
			}
			out.TotalFound = len(out.Results)

		case "html-article":
			htmlStr, err := extractors.FetchHTML(ctx, queryURL)
			if err != nil {
				return toolError(fmt.Sprintf("fetching %s: %v", src.Name, err)), nil, nil
			}
			title := htmlTitle(htmlStr)
			snippet := htmlMetaDescription(htmlStr)
			out.Results = []SearchResult{{
				URL:            queryURL,
				Title:          title,
				Snippet:        snippet,
				ContentType:    "html",
				RelevanceScore: signals.ScoreRelevance(args.Query, title, snippet),
			}}
			out.TotalFound = 1

		default:
			return toolError(fmt.Sprintf("unknown result_type %q for source %s", src.ResultType, src.Name)), nil, nil
		}

		data, _ := json.Marshal(out)
		return toolText(string(data)), nil, nil
	}
}

func fetchJSONAPI(ctx context.Context, queryURL string, src registry.Source, query string) ([]SearchResult, int, bool, error) {
	body, _, err := extractors.HTTPFetch(ctx, queryURL)
	if err != nil {
		return nil, 0, false, err
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return nil, 0, false, fmt.Errorf("parsing JSON response: %w", err)
	}

	f := src.ResultFields
	itemsRaw, ok := raw[f["items"]]
	if !ok {
		return nil, 0, false, fmt.Errorf("items key %q not found in JSON response", f["items"])
	}
	items, ok := itemsRaw.([]any)
	if !ok {
		return nil, 0, false, fmt.Errorf("items value is not an array")
	}

	var results []SearchResult
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		urlVal := strField(m, f["url"])
		titleVal := strField(m, f["title"])
		snippetVal := strField(m, f["snippet"])
		if urlVal == "" {
			continue
		}
		results = append(results, SearchResult{
			URL:            urlVal,
			Title:          titleVal,
			Snippet:        snippetVal,
			ContentType:    guessContentType(urlVal),
			RelevanceScore: signals.ScoreRelevance(query, titleVal, snippetVal),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})

	total := len(results)
	hasMore := false
	if nbHits, ok := raw["nbHits"].(float64); ok {
		total = int(nbHits)
		hasMore = total > len(results)
	}
	if hm, ok := raw["has_more"].(bool); ok {
		hasMore = hm
	}
	return results, total, hasMore, nil
}

func fetchHTMLLinks(ctx context.Context, queryURL string, query string) ([]SearchResult, error) {
	htmlStr, err := extractors.FetchHTML(ctx, queryURL)
	if err != nil {
		return nil, err
	}

	links := extractors.ExtractLinks(htmlStr, queryURL)
	var results []SearchResult
	for _, l := range links {
		if l.Rel == "navigation" || l.Rel == "external" {
			continue
		}
		results = append(results, SearchResult{
			URL:            l.URL,
			Title:          l.Text,
			Snippet:        l.Snippet,
			ContentType:    guessContentType(l.URL),
			RelevanceScore: signals.ScoreRelevance(query, l.Text, l.Snippet),
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})
	return results, nil
}

func guessContentType(rawURL string) string {
	lower := strings.ToLower(rawURL)
	switch {
	case strings.HasSuffix(lower, ".pdf"):
		return "pdf"
	case strings.HasSuffix(lower, ".docx"), strings.HasSuffix(lower, ".doc"):
		return "docx"
	case strings.HasSuffix(lower, ".png"), strings.HasSuffix(lower, ".jpg"),
		strings.HasSuffix(lower, ".jpeg"), strings.HasSuffix(lower, ".gif"),
		strings.HasSuffix(lower, ".webp"):
		return "image"
	default:
		return "html"
	}
}

func strField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func htmlTitle(htmlStr string) string {
	lower := strings.ToLower(htmlStr)
	start := strings.Index(lower, "<title")
	if start < 0 {
		return ""
	}
	tagEnd := strings.Index(lower[start:], ">")
	if tagEnd < 0 {
		return ""
	}
	contentStart := start + tagEnd + 1
	end := strings.Index(lower[contentStart:], "</title>")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(htmlStr[contentStart : contentStart+end])
}

func htmlMetaDescription(htmlStr string) string {
	lower := strings.ToLower(htmlStr)
	for _, metaKey := range []string{`name="description"`, `property="og:description"`} {
		idx := strings.Index(lower, metaKey)
		if idx < 0 {
			continue
		}
		chunk := lower[idx:]
		cIdx := strings.Index(chunk, `content="`)
		if cIdx < 0 {
			continue
		}
		start := idx + cIdx + 9
		end := strings.Index(htmlStr[start:], `"`)
		if end < 0 {
			continue
		}
		return htmlStr[start : start+end]
	}
	return ""
}
