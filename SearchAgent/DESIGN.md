# SearchAgent MCP Server — Architecture

## Design Goals

1. **No external search API** — uses a curated `sources.json` registry of trusted sources
2. **LLM-agnostic** — works with any MCP-compatible model; tool descriptions are self-contained
3. **No AI/agent logic in the server** — pure infrastructure: fetch, extract, score
4. **Multi-format** — HTML, PDF, DOCX, and images handled uniformly

---

## Scope Boundary

```
┌─────────────────────────────────────────┐
│          SearchAgent MCP Server         │
│                                         │
│  list_sources / query_source /          │
│  fetch_metadata / fetch_content         │
│                                         │
│  [signals — isolated, quality-critical] │
└────────────────┬────────────────────────┘
                 │ MCP protocol
       ┌─────────┴──────────┐
       │   any LLM / agent  │  ← NOT in this repo
       └────────────────────┘
```

The LLM drives all orchestration. The server provides clean, structured data and signals.

---

## Tools

### `list_sources(category?)`
Returns the source registry filtered by optional category.
No URLs are exposed — sources are referenced by name only.

### `query_source(source, query)`
Constructs the search URL from `sources.json` template, fetches the results page,
and parses it into a structured array with relevance scores.

Signals returned:
- `relevance_score` per result — keyword match quality (0.0–1.0)
- `total_found` — total results on source (not just page)
- `has_more` — pagination available

### `fetch_metadata(url)`
Reads only HTTP headers + `<head>` section (max 64KB, no Playwright).
Returns SEO metadata, OpenGraph, JSON-LD structured data, and a links preview.
**Use for pre-screening before committing to a full fetch.**

### `fetch_content(url)`
Detects content type via HEAD request → routes to appropriate extractor:
- HTML: HTTP fetch + Playwright fallback for SPAs/anti-bot → html-to-markdown
- PDF: raw bytes → `ledongthuc/pdf` text extraction
- DOCX: raw bytes → `archive/zip` + `word/document.xml` parsing
- Image: raw bytes → base64 MCP `ImageContent`

Returns content + classified link list for further navigation.

---

## Signals Module (`internal/signals/`)

**Isolated package — change frequently, test thoroughly.**

- `ScoreRelevance(query, title, snippet)` → float64
  - Tokenize + weighted keyword overlap (title 2×, snippet 1×)
  - Upgrade path: BM25, then embedding cosine similarity
- `ClassifyLink(url, text, pageHost)` → rel string
  - `download` — file extension (pdf, docx, etc.)
  - `pagination` — URL pattern or anchor text
  - `navigation` — boilerplate (home, about, login...)
  - `related` — "see also", "learn more", etc.
  - `external` — different domain

---

## Source Registry (`sources.json`)

Loaded at startup from `$SOURCES_FILE` env or binary directory.
No recompile needed to add sources.

**result_type values:**
- `json-api` — parse JSON body; fields mapped via `result_fields`
- `html-links` — extract link list from HTML results page
- `html-article` — the page IS the content (e.g. Wikipedia)

---

## Package Structure

```
main.go                     — server init + tool registration
internal/registry/          — source config loading
internal/signals/           — ISOLATED: scoring + link classification
extractors/
  html.go                   — HTTP fetch + SPA/anti-bot + html-to-markdown
  pdf.go                    — PDF text extraction
  docx.go                   — DOCX XML extraction (stdlib only)
  image.go                  — image download → base64
  links.go                  — HTML link extraction + classification
tools/
  list_sources.go
  query_source.go
  fetch_metadata.go
  fetch_content.go
  helpers.go
```

---

## Adding a New Source

Edit `sources.json` and restart the server. No code changes needed for:
- Any `json-api` source with a clean JSON search API
- Any `html-links` source where results are a list of `<a>` links
- Any `html-article` source that is itself the content

For non-standard result formats, extend `query_source.go`'s `ResultType` switch.

---

## Upgrading the Signals Module

The signals module is deliberately isolated so scoring can be improved independently:

```
Current:  keyword overlap (TF-style)
Next:     BM25 (better term frequency weighting)
Future:   embedding cosine similarity (requires embedding service)
```

Changes to `internal/signals/` do not require touching any other package.
Run `go test ./internal/signals/` after every change.
