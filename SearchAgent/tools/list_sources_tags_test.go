package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestListSourcesTags(t *testing.T) {
	reg := newTestRegistry(t, testSources)
	handler := NewListSourcesTags(reg)
	result, _, err := handler(context.Background(), &mcp.CallToolRequest{}, struct{}{})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	var tags []string
	if err := json.Unmarshal([]byte(text), &tags); err != nil {
		t.Fatalf("unmarshal tags: %v", err)
	}

	// testSources has tags: tech-news, community, academic, research, code — 5 unique tags
	if len(tags) == 0 {
		t.Fatal("expected non-empty tags list")
	}

	// verify sorted and deduplicated
	for i := 1; i < len(tags); i++ {
		if tags[i] <= tags[i-1] {
			t.Errorf("tags not sorted/deduplicated at index %d: %v", i, tags)
		}
	}
}
