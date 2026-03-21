# SearchAgent Benchmark

Two-layer evaluation suite for the SearchAgent MCP server.

| Layer | Script | Metrics | LLM needed? |
|---|---|---|---|
| 1 — Retrieval | `run_retrieval.py` | P@k, R@k, NDCG, MRR, ScoreCorr | No |
| 2 — Content quality | `run_content.py` | Context Precision, Context Recall | Yes (OVH AI) |

---

## Setup

```bash
cd benchmark
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

Build the MCP server binary if not already done:

```bash
cd ../SearchAgent && go build -o searchagent-mcp .
```

---

## Layer 1 — Retrieval benchmark

No API key needed. Runs queries through `query_source`, compares returned URLs
against a hand-annotated ground-truth list, and computes standard IR metrics.

```bash
python run_retrieval.py
```

**Output example:**
```
Query : 'go 1.24'
Source: hackernews
  total_found=54  has_more=True  returned=20
  [1] score=0.67  https://tip.golang.org/doc/go1.24
       Go 1.24
  [2] score=0.67  https://go.dev/blog/go1.24
       Go 1.24 Is Released
  ...
  P@5=0.200  P@10=0.100  R@5=0.500  R@10=0.500  NDCG@10=0.327  MRR=0.250  ScoreCorr=0.500
```

### Growing the ground truth

The dataset `datasets/retrieval.jsonl` has a few annotated cases. After running
the benchmark, review the printed results and add the relevant URLs:

```jsonl
{"query": "...", "source": "...",
 "relevant_urls": ["https://actual-result.com/..."],
 "grades": {"https://actual-result.com/...": 2}}
```

**Grade scale:** 0 = irrelevant, 1 = partially relevant, 2 = highly relevant.
Leave `relevant_urls: []` and `grades: {}` for unannotated cases — they still
print results for inspection but are excluded from metric averages.

---

## Layer 2 — Content quality benchmark

Requires an OVH AI API token. Fetches pages via `fetch_content`, splits the
returned content into chunks, and evaluates with ragas using your LLM.

```bash
export OVH_AI_TOKEN=<your_token>
python run_content.py
```

**Output example:**
```
RAGAS RESULTS
============================================================
  Context Precision              0.8750
  Context Recall                 0.9120
```

### Metrics explained

- **Context Precision** — of all text chunks retrieved, what fraction actually
  contains information relevant to the question? High = low noise.
- **Context Recall** — does the retrieved content cover all facts in the ground
  truth answer? High = no missing information.

### OVH AI configuration

Edit `config.yaml`:

```yaml
llm:
  api_base: "https://endpoints.ai.cloud.ovh.net/api/openai/v1"
  api_key_env: "OVH_AI_TOKEN"
  model: "Meta-Llama-3.1-70B-Instruct"
```

The API is OpenAI-compatible; any model at the configured `api_base` works.

---

## Implementation

### Directory layout

```
benchmark/
├── config.yaml              # LLM endpoint, MCP binary path, benchmark params
├── requirements.txt         # Python dependencies
├── run_retrieval.py         # Layer 1 entry point
├── run_content.py           # Layer 2 entry point
├── src/
│   ├── mcp_client.py        # MCP stdio transport (JSON-RPC)
│   ├── metrics.py           # Offline IR metrics (P@k, NDCG, MRR, ScoreCorr)
│   └── ragas_eval.py        # Ragas LLM-judge wrapper
└── datasets/
    ├── retrieval.jsonl      # Query + ground-truth annotations for Layer 1
    └── content.jsonl        # URL + ground-truth answers for Layer 2
```

Results are written to `results/` (git-ignored).

---

### `src/mcp_client.py` — MCP stdio transport

`MCPClient` launches the server binary as a subprocess and drives it over
JSON-RPC on stdin/stdout. A background reader thread dispatches responses to
per-request `queue.Queue` objects, so the main thread blocks on `q.get()` with
a configurable timeout.

Lifecycle:
1. `subprocess.Popen` — launch binary with `SOURCES_FILE` env if provided
2. `_handshake()` — send `initialize` RPC + `notifications/initialized` notify
3. `_tool(name, args)` — wrap `tools/call` RPC and decode the first `text` content item as JSON
4. `close()` — close stdin, wait for process exit (kill on timeout)

Tool methods mirror the MCP API: `list_sources`, `query_source`,
`fetch_metadata`, `fetch_content`.

---

### `src/metrics.py` — offline IR metrics

All functions operate on plain Python lists/dicts — no LLM involved.

| Function | Description |
|---|---|
| `precision_at_k` | Fraction of top-k results that are relevant |
| `recall_at_k` | Fraction of relevant URLs found in top-k |
| `ndcg_at_k` | Normalized DCG using per-URL grade (0/1/2) |
| `mrr` | Reciprocal rank of the first relevant hit |
| `score_correlation` | Fraction of relevant URLs in the server's top-half by `relevance_score`; measures whether the signals module is a useful ranking signal |

`RetrievalCase` holds the ground truth. `RetrievalResult` holds what the server
returned. `compute_retrieval_metrics` combines them. `average_metrics` averages
across all annotated cases.

---

### `src/ragas_eval.py` — LLM-judge via ragas

Uses `ragas` with `LangchainLLMWrapper(ChatOpenAI(...))` pointed at the OVH
endpoint. Metrics: `LLMContextPrecisionWithReference` and `LLMContextRecall`.

`run_ragas(config, samples)` takes a list of `{"question", "contexts",
"ground_truth"}` dicts, runs the ragas evaluation pipeline, and returns a
`{metric_name: score}` dict.

---

### `datasets/retrieval.jsonl` — retrieval ground truth

One JSON object per line. The `grades` map drives NDCG; `relevant_urls` is the
union of all non-zero-grade URLs and drives P@k, R@k, MRR.

```jsonl
{
  "query": "go 1.24",
  "source": "hackernews",
  "relevant_urls": ["https://go.dev/blog/go1.24"],
  "grades": {"https://go.dev/blog/go1.24": 2}
}
```

### `datasets/content.jsonl` — content quality ground truth

```jsonl
{
  "question": "How does X work?",
  "fetch_url": "https://...",
  "ground_truth": "Expected answer used as ragas reference."
}
```

---

## Iterating on the signals module

After changing `SearchAgent/internal/signals/`, rebuild and re-run the retrieval
benchmark to see whether NDCG and ScoreCorr improve:

```bash
cd ../SearchAgent && go build -o searchagent-mcp . && cd ../benchmark
python run_retrieval.py
```

`ScoreCorr` specifically measures whether `relevance_score` from the server is a
useful ranking signal — it tells you if the signals module changes are helping.
