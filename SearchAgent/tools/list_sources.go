package tools

import (
	"context"
	"encoding/json"

	"github.com/johnnysvm/searchagent-mcp/internal/registry"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListSourcesInput parameters for list_sources.
type ListSourcesInput struct {
	Tag string `json:"tag,omitempty"`
}

// sourceInfo is the public representation returned to the LLM.
type sourceInfo struct {
	Name         string   `json:"name"`
	Label        string   `json:"label"`
	Tags         []string `json:"tags"`
	Description  string   `json:"description"`
	ContentTypes []string `json:"content_types"`
}

// NewListSources returns the list_sources tool handler.
func NewListSources(reg *registry.Registry) func(context.Context, *mcp.CallToolRequest, ListSourcesInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args ListSourcesInput) (*mcp.CallToolResult, any, error) {
		var sources []registry.Source
		if args.Tag != "" {
			sources = reg.ByTag(args.Tag)
		} else {
			sources = reg.All()
		}

		if len(sources) == 0 {
			available := reg.AllTags()
			data, err := json.Marshal(map[string]any{
				"error":           "no sources found for tag: " + args.Tag,
				"available_tags": available,
			})
			if err != nil {
				return toolError("marshal error: " + err.Error()), nil, nil
			}
			return toolText(string(data)), nil, nil
		}

		result := make([]sourceInfo, 0, len(sources))
		for _, s := range sources {
			result = append(result, sourceInfo{
				Name:         s.Name,
				Label:        s.Label,
				Tags:         s.Tags,
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
