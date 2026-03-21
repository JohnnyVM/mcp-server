# How to use SearchAgent MCP tools

## Overview

Four tools: `list_sources` → `query_source` → `fetch_metadata` → `fetch_content`.
The recommended workflow always starts narrow (metadata) and goes full-content only when needed.

---

## Workflow

### Step 1 — Discover sources
```
list_sources()                        # all sources
list_sources(category="code")         # filtered
```
Categories: `tech-news` | `docs` | `code` | `community` | `academic` | `encyclopedic`

Pick sources whose `description` matches the query intent.

---

### Step 2 — Search
```
query_source(source="hackernews", query="go 1.24 release")
```
Returns: `{results: [{url, title, snippet, content_type, relevance_score}], total_found, has_more, query_used}`

**Signals to act on:**
- `total_found=0` → try a different source or rephrase the query
- `relevance_score < 0.3` → result is likely off-topic; skip or deprioritize
- `has_more=true` → more pages exist if results are insufficient

---

### Step 3 — Pre-scan with metadata (fast, ~100ms)
Before fetching full content, scan candidates:
```
fetch_metadata(url="https://arxiv.org/abs/2301.07041")
```
Returns: `{title, description, og:*, json_ld, canonical_url, links_preview}`

Use it to:
- Confirm the page is actually about the topic (check `title`, `description`, `og.description`)
- Check `content_type` before fetching (avoid fetching large files unexpectedly)
- Read `json_ld` — may already contain complete structured data (events, products, articles)
- Find direct file links in `links_preview` (e.g. PDF download links)

---

### Step 4 — Fetch full content (selective)
Only fetch pages that passed the metadata check:
```
fetch_content(url="https://arxiv.org/abs/2301.07041")
fetch_content(url="https://arxiv.org/pdf/2301.07041.pdf")  # PDF direct link
```
Returns: `{content, content_type, title, canonical_url, links: [{url, text, rel}]}`

**Content types:**
- `html` → clean markdown, ready to read
- `pdf` → plain text with `--- Page N ---` markers
- `docx` → plain text with headings
- `image` → base64 image (requires multimodal LLM)

**Acting on `links`:**
- `rel=related` → follow if anchor text is relevant to the query
- `rel=pagination` → follow if this page didn't have enough information
- `rel=download` → direct file link (PDF, DOCX); call `fetch_content` on it directly
- `rel=external` → different domain; evaluate before following

---

## Rules

1. Always call `fetch_metadata` before `fetch_content` when evaluating multiple candidates
2. Do not call `fetch_content` on every result URL — filter with metadata + snippet first
3. Prefer sources whose `category` matches the query intent
4. If first search is thin (`total_found < 3` or all scores < 0.3), try another source or rephrase
5. For academic papers: fetch the abstract page (HTML) first; only fetch PDF if you need full text
