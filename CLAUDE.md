# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

A personal collection of MCP (Model Context Protocol) server implementations in Go.

## Repository Structure

- `WebGet/` — MCP tool `get_webpage_html(url)`: smart web fetcher with SPA/anti-bot detection and Playwright fallback.
- `SearchAgent/` — MCP server with 4 tools for source-driven research: `list_sources`, `query_source`, `fetch_metadata`, `fetch_content`. See `SearchAgent/DESIGN.md` for architecture and `SearchAgent/SKILL.md` for usage guide.

## WebGet Tool

`get_webpage_html(url)` — fetches HTML, detects SPAs and anti-bot pages, falls back to headless Chrome.

## SearchAgent Tools

Research tools backed by a curated source registry (`SearchAgent/sources.json`).
No external search API dependency. LLM-agnostic — works with any MCP-compatible model.

- `list_sources(category?)` — discover available sources
- `query_source(source, query)` — search a source → structured results with relevance scores
- `fetch_metadata(url)` — fast metadata pre-scan (headers + `<head>` only, ~100ms)
- `fetch_content(url)` — full content fetch (HTML→markdown, PDF→text, DOCX→text, image→base64)

See `.claude/commands/research.md` for the `/research` skill (LLM usage guide).

## Build

```bash
# WebGet
cd WebGet && go build -o webget-mcp .

# SearchAgent
cd SearchAgent && go mod tidy && go build -o searchagent-mcp .
```

## Adding Sources

Edit `SearchAgent/sources.json` and restart the MCP server. No recompile needed.

## Signals Module

`SearchAgent/internal/signals/` is isolated on purpose — it changes frequently.
Always run `go test ./internal/signals/` after touching it.
