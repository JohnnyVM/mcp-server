package tools

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/johnnysvm/searchagent-mcp/internal/registry"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newTestRegistry(t *testing.T, sources []registry.Source) *registry.Registry {
	t.Helper()
	data, err := json.Marshal(sources)
	if err != nil {
		t.Fatalf("marshal sources: %v", err)
	}
	f, err := os.CreateTemp(t.TempDir(), "sources*.json")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	reg, err := registry.Load(f.Name())
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	return reg
}

var testSources = []registry.Source{
	{
		Name:         "hackernews",
		Label:        "Hacker News",
		Tags:         []string{"tech-news", "community"},
		Description:  "Tech news and community discussion.",
		ContentTypes: []string{"html"},
	},
	{
		Name:         "arxiv",
		Label:        "arXiv",
		Tags:         []string{"academic", "research"},
		Description:  "Preprint research papers.",
		ContentTypes: []string{"html", "pdf"},
	},
	{
		Name:         "github",
		Label:        "GitHub",
		Tags:         []string{"code", "tech-news"},
		Description:  "Open source repositories.",
		ContentTypes: []string{"html"},
	},
}

func callListSources(t *testing.T, reg *registry.Registry, tag string) []sourceInfo {
	t.Helper()
	handler := NewListSources(reg)
	result, _, err := handler(context.Background(), &mcp.CallToolRequest{}, ListSourcesInput{Tag: tag})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	var infos []sourceInfo
	if err := json.Unmarshal([]byte(text), &infos); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return infos
}

func TestListSources_All(t *testing.T) {
	reg := newTestRegistry(t, testSources)
	infos := callListSources(t, reg, "")

	if got, want := len(infos), len(testSources); got != want {
		t.Fatalf("got %d sources, want %d", got, want)
	}
	names := make(map[string]bool)
	for _, s := range infos {
		names[s.Name] = true
	}
	for _, s := range testSources {
		if !names[s.Name] {
			t.Errorf("source %q missing from result", s.Name)
		}
	}
}

func TestListSources_ByTag(t *testing.T) {
	reg := newTestRegistry(t, testSources)
	// "tech-news" matches hackernews and github
	infos := callListSources(t, reg, "tech-news")

	if got, want := len(infos), 2; got != want {
		t.Fatalf("got %d sources for tech-news, want %d", got, want)
	}
	for _, s := range infos {
		if !slices.Contains(s.Tags, "tech-news") {
			t.Errorf("source %q does not have tag tech-news, tags: %v", s.Name, s.Tags)
		}
	}
}

func TestListSources_TagMatchesMultipleTags(t *testing.T) {
	reg := newTestRegistry(t, testSources)
	// "community" only matches hackernews (second tag)
	infos := callListSources(t, reg, "community")

	if got, want := len(infos), 1; got != want {
		t.Fatalf("got %d sources for community, want %d", got, want)
	}
	if infos[0].Name != "hackernews" {
		t.Errorf("expected hackernews, got %q", infos[0].Name)
	}
}

func TestListSources_TagNoMatch(t *testing.T) {
	reg := newTestRegistry(t, testSources)

	handler := NewListSources(reg)
	result, _, err := handler(context.Background(), &mcp.CallToolRequest{}, ListSourcesInput{Tag: "nonexistent"})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	var resp map[string]any
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if _, ok := resp["available_tags"]; !ok {
		t.Errorf("expected available_tags in response, got: %s", text)
	}
	if _, ok := resp["error"]; !ok {
		t.Errorf("expected error message in response, got: %s", text)
	}
	tags, _ := resp["available_tags"].([]any)
	if len(tags) == 0 {
		t.Errorf("expected non-empty available_tags")
	}
}

func TestListSources_Fields(t *testing.T) {
	reg := newTestRegistry(t, testSources)
	infos := callListSources(t, reg, "academic")

	if len(infos) != 1 {
		t.Fatalf("expected 1 academic source, got %d", len(infos))
	}
	s := infos[0]
	if s.Name != "arxiv" {
		t.Errorf("name = %q, want arxiv", s.Name)
	}
	if s.Label != "arXiv" {
		t.Errorf("label = %q, want arXiv", s.Label)
	}
	if len(s.Tags) != 2 {
		t.Errorf("tags len = %d, want 2", len(s.Tags))
	}
	if len(s.ContentTypes) != 2 {
		t.Errorf("content_types len = %d, want 2", len(s.ContentTypes))
	}
}
