# mcp-server

A personal collection of MCP (Model Context Protocol) server implementations in Go.

## Servers

| Server | Docker Hub | Description |
|--------|------------|-------------|
| [WebGet](./WebGet/) | `johnnyvm90/webget-mcp` | Smart web fetcher with SPA/anti-bot detection and Playwright fallback |
| [SearchAgent](./SearchAgent/) | `johnnyvm90/searchagent-mcp` | Source-driven research with 4 tools: list, query, fetch metadata, fetch content |

---

## Quick Start (Docker)

### WebGet

```bash
podman pull johnnyvm90/webget-mcp
```

MCP config (`~/.config/claude/claude_desktop_config.json` or equivalent):

```json
{
  "mcpServers": {
    "webget": {
      "command": "podman",
      "args": ["run", "-i", "--rm", "johnnyvm90/webget-mcp"]
    }
  }
}
```

### SearchAgent

```bash
podman pull johnnyvm90/searchagent-mcp
```

MCP config:

```json
{
  "mcpServers": {
    "searchagent": {
      "command": "podman",
      "args": ["run", "-i", "--rm", "johnnyvm90/searchagent-mcp"]
    }
  }
}
```

To use a custom `sources.json`, mount it over the default:

```json
{
  "mcpServers": {
    "searchagent": {
      "command": "podman",
      "args": [
        "run", "-i", "--rm",
        "-v", "/path/to/your/sources.json:/app/sources.json:z",
        "johnnyvm90/searchagent-mcp"
      ]
    }
  }
}
```

---

## Building from Source

```bash
# WebGet
cd WebGet && go build -o webget-mcp .

# SearchAgent
cd SearchAgent && go mod tidy && go build -o searchagent-mcp .
```

## Building Images Locally

```bash
# WebGet
podman build -t johnnyvm90/webget-mcp ./WebGet

# SearchAgent
podman build -t johnnyvm90/searchagent-mcp ./SearchAgent
```

## Publishing to Docker Hub

```bash
podman push johnnyvm90/webget-mcp
podman push johnnyvm90/searchagent-mcp
```
