package main

import (
	"context"
	"log"

	"github.com/johnnysvm/searchagent-mcp/internal/registry"
	"github.com/johnnysvm/searchagent-mcp/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	reg, err := registry.Load(registry.DefaultPath())
	if err != nil {
		log.Fatalf("loading source registry: %v", err)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "searchagent", Version: "v0.1.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_sources",
		Description: "List available research sources. Returns [{name, label, category, description, content_types}]. " +
			"Call this first to discover sources, then use query_source with a source name. " +
			"Filter by category: tech-news | docs | code | community | academic | encyclopedic.",
	}, tools.NewListSources(reg))

	mcp.AddTool(server, &mcp.Tool{
		Name: "query_source",
		Description: "Search a named source and get structured results [{url, title, snippet, content_type, relevance_score}]. " +
			"Call list_sources first to get valid source names. " +
			"Returns total_found and has_more for pagination awareness. " +
			"relevance_score is 0.0–1.0; skip results below 0.3. " +
			"Use fetch_metadata to pre-scan URLs, or fetch_content to get full page content.",
	}, tools.NewQuerySource(reg))

	mcp.AddTool(server, &mcp.Tool{
		Name: "fetch_metadata",
		Description: "Fast metadata pre-scan of a URL (~100ms): reads only HTTP headers and the HTML <head> section. " +
			"Returns title, description, og:* fields, json_ld structured data, canonical URL, and links_preview. " +
			"Use this to screen multiple result URLs before deciding which to fully fetch with fetch_content. " +
			"json_ld may contain complete structured data (e.g. event info, product details) without needing a full fetch.",
	}, tools.NewFetchMetadata())

	mcp.AddTool(server, &mcp.Tool{
		Name: "fetch_content",
		Description: "Fetch the full content of any URL. Auto-detects type: HTML→clean markdown, PDF→plain text, DOCX→plain text, image→image data. " +
			"Returns {content, content_type, title, canonical_url, links:[{url, text, rel}]}. " +
			"Link rel values: related | pagination | download | external. " +
			"Follow rel=download links (PDFs, DOCX) with another fetch_content call. " +
			"HTML pages use browser fallback automatically if JS-rendered or behind anti-bot protection.",
	}, tools.NewFetchContent())

	if err := server.Run(context.Background(), mcp.NewStdioTransport()); err != nil {
		log.Fatal(err)
	}
}
