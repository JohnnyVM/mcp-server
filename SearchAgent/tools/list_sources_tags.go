package tools

import (
	"context"
	"encoding/json"

	"github.com/johnnysvm/searchagent-mcp/internal/registry"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewListSourcesTags returns the list_sources_tags tool handler.
func NewListSourcesTags(reg *registry.Registry) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		tags := reg.AllTags()
		data, err := json.Marshal(tags)
		if err != nil {
			return toolError("marshal error: " + err.Error()), nil, nil
		}
		return toolText(string(data)), nil, nil
	}
}
