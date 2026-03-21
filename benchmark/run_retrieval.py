#!/usr/bin/env python3
"""Layer 1 benchmark: retrieval quality (offline, no LLM needed).

Measures whether query_source returns relevant results and whether
the server-side relevance_score is a useful ranking signal.

Usage:
    python run_retrieval.py [--config config.yaml] [--dataset datasets/retrieval.jsonl]
"""

import argparse
import json
import os
import sys
from pathlib import Path

import yaml

sys.path.insert(0, str(Path(__file__).parent))
from src.mcp_client import MCPClient
from src.metrics import (
    RetrievalCase,
    RetrievalMetrics,
    RetrievalResult,
    average_metrics,
    compute_retrieval_metrics,
)


def load_cases(path: str) -> list[RetrievalCase]:
    cases = []
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            d = json.loads(line)
            cases.append(RetrievalCase(
                query=d["query"],
                source=d["source"],
                relevant_urls=d.get("relevant_urls", []),
                grades=d.get("grades", {}),
            ))
    return cases


def run(config: dict, cases: list[RetrievalCase], top_k: int = 10) -> None:  # noqa: ARG001
    binary = os.path.abspath(config["mcp_server"]["binary"])
    sources = os.path.abspath(config["mcp_server"]["sources_file"])

    print(f"Starting MCP server: {binary}")
    with MCPClient(binary, sources_file=sources) as client:
        all_metrics: list[RetrievalMetrics] = []
        unannotated = 0

        for case in cases:
            print(f"\nQuery : {case.query!r}")
            print(f"Source: {case.source}")

            try:
                response = client.query_source(case.source, case.query)
            except Exception as e:
                print(f"  ERROR: {e}")
                continue

            results = response.get("results") or []
            retrieved_urls = [r["url"] for r in results]
            server_scores = {r["url"]: r.get("relevance_score", 0.0) for r in results}

            print(f"  total_found={response.get('total_found', '?')}  "
                  f"has_more={response.get('has_more', '?')}  "
                  f"returned={len(results)}")

            for i, r in enumerate(results[:5]):
                print(f"  [{i+1}] score={r.get('relevance_score', 0):.2f}  {r['url'][:80]}")
                if r.get("title"):
                    print(f"       {r['title'][:70]}")

            if not case.relevant_urls and not case.grades:
                print("  (no ground truth — skipping metrics for this case)")
                unannotated += 1
                continue

            result = RetrievalResult(
                query=case.query,
                source=case.source,
                retrieved_urls=retrieved_urls,
                scores=server_scores,
            )
            m = compute_retrieval_metrics(case, result)
            all_metrics.append(m)
            print(f"  {m}")

    print("\n" + "=" * 60)
    if all_metrics:
        avg = average_metrics(all_metrics)
        print(f"AVERAGE over {len(all_metrics)} annotated cases:")
        print(f"  {avg}")
    else:
        print("No annotated cases to aggregate.")

    if unannotated:
        print(f"\nNote: {unannotated} cases have no ground truth URLs.")
        print("  To improve coverage, add relevant_urls/grades to retrieval.jsonl after")
        print("  reviewing the results printed above.")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", default="config.yaml")
    parser.add_argument("--dataset", default="datasets/retrieval.jsonl")
    args = parser.parse_args()

    script_dir = Path(__file__).parent
    os.chdir(script_dir)

    with open(args.config) as f:
        config = yaml.safe_load(f)

    cases = load_cases(args.dataset)
    print(f"Loaded {len(cases)} retrieval test cases")
    run(config, cases, top_k=config["benchmark"].get("top_k", 10))


if __name__ == "__main__":
    main()
