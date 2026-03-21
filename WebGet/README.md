# WebGet MCP Server

An MCP server exposing `get_webpage_html(url)` — a smarter web fetcher that transparently handles JS-rendered (SPA) pages and anti-bot walls. It tries plain HTTP first (~100ms), then falls back to headless Chrome only when needed. Chrome is kept alive for 3 minutes after last use to amortize startup cost across batched fetches.

## Fetch Decision Strategy

```
get_webpage_html(url)
         │
  [1] Plain HTTP (15s timeout, browser-like headers, follow redirects ≤10)
         │
         ├─ error/timeout ──► return error
         │
  [2] SPA? ALL of:
      • <body> visible text < 300 chars (strip tags)
      • <script> tag present
      • <body> tag exists
  [3] Anti-bot? ANY of:
      • status 403 / 429 / 503
      • body contains: "just a moment", "cf-browser-verification",
        "enable javascript and cookies", "captcha", "hcaptcha",
        "recaptcha", "please verify you are a human", "datadome",
        "_incapsula_resource", "access denied", "automated access",
        "unusual traffic"
         │
  ──[no]──► return HTTP html   (~100ms)
  ──[yes]─► Chrome fallback    (~1-3s first call, ~200ms warm)
```

## Prerequisites

- Go 1.22+
- Chromium installed system-wide (used by chromedp)

```bash
which chromium   # must exist
```

## Build

```bash
cd WebGet
go mod tidy
go build -o webget-mcp .
```

## Configure in Claude Code

**`.claude/mcp.json`** (already set up in this repo):
```json
{
  "mcpServers": {
    "webget": {
      "type": "stdio",
      "command": "/home/johnny/git/ai/mcp-server/WebGet/webget-mcp"
    }
  }
}
```

**`.claude/settings.json`** — disables built-in web tools so Claude uses `get_webpage_html` instead:
```json
{
  "permissions": {
    "deny": ["WebFetch", "WebSearch"]
  }
}
```

`enableAllProjectMcpServers: true` is already set in `settings.local.json`.

Restart Claude Code after updating config, then run `/mcp` to verify `webget` is listed.

## Configure in OpenCode

Add to `~/.config/opencode/opencode.json` (global) or `opencode.json` (project):

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "webget": {
      "type": "local",
      "command": ["/home/johnny/git/ai/mcp-server/WebGet/webget-mcp"],
      "enabled": true
    }
  }
}
```

> OpenCode differences from Claude Code: root key is `mcp` (not `mcpServers`), type is `"local"` (not `"stdio"`), command is an array.

## Usage Examples

```
# Fetch a static page (fast path, no browser)
get_webpage_html("https://example.com")

# Fetch a JS-rendered SPA (auto Chrome fallback)
get_webpage_html("https://react.dev/learn")

# Behind Cloudflare (auto Chrome fallback)
get_webpage_html("https://some-cf-protected-site.com")
```

**Example AI conversation:**
> User: "Summarize the changelog at https://github.com/golang/go/releases"
> → Claude calls `get_webpage_html("https://github.com/golang/go/releases")`
> → Returns rendered HTML with release notes
> → Claude summarizes

## Verification

1. **Smoke test** (tools/list):
   ```bash
   echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | ./webget-mcp
   ```
   Expect `"name": "get_webpage_html"`.

2. **Fast path** (no browser):
   ```bash
   echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_webpage_html","arguments":{"url":"https://example.com"}}}' | ./webget-mcp
   ```
   Returns in ~100ms with `<h1>Example Domain</h1>`.

3. **SPA path** (browser triggered):
   Use `https://react.dev` — raw HTTP gives sparse content, browser gives full rendered HTML.

4. **Keep-alive**: Call twice in quick succession — second call should be faster (Chrome already warm).
