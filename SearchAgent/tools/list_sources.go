package tools

import (
	"context"
	"encoding/json"

	"github.com/johnnysvm/searchagent-mcp/internal/registry"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListSourcesInput parameters for list_sources.
type ListSourcesInput struct {
	Category string `json:"category,omitempty"`
}

// sourceInfo is the public representation returned to the LLM.
type sourceInfo struct {
	Name         string   `json:"name"`
	Label        string   `json:"label"`
	Category     string   `json:"category"`
	Description  string   `json:"description"`
	ContentTypes []string `json:"content_types"`
}

// NewListSources returns the list_sources tool handler.
func NewListSources(reg *registry.Registry) func(context.Context, *mcp.CallToolRequest, ListSourcesInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args ListSourcesInput) (*mcp.CallToolResult, any, error) {
		var sources []registry.Source
		if args.Category != "" {
			sources = reg.ByCategory(args.Category)
		} else {
			sources = reg.All()
		}

		result := make([]sourceInfo, 0, len(sources))
		for _, s := range sources {
			result = append(result, sourceInfo{
				Name:         s.Name,
				Label:        s.Label,
				Category:     s.Category,
				Description:  s.Description,
				ContentTypes: s.ContentTypes,
			})
		}

		data, err := json.Marshal(result)
		if err != nil {
			return toolError("marshal error: " + err.Error()), nil, nil
		}
		return toolText(string(data)), nil, nil
	}
}
