#!/usr/bin/env python3
"""Layer 2 benchmark: content quality via ragas (requires LLM API).

Fetches pages via fetch_content, then evaluates with ragas
context_precision and context_recall using the configured LLM backend.

Usage:
    export OVH_AI_TOKEN=<your_token>
    python run_content.py [--config config.yaml] [--dataset datasets/content.jsonl]
"""

import argparse
import json
import os
import sys
from pathlib import Path

import yaml

sys.path.insert(0, str(Path(__file__).parent))
from src.mcp_client import MCPClient
from src.ragas_eval import ContentCase, ContentSample, make_llm, run_ragas


def load_cases(path: str) -> list[ContentCase]:
    cases = []
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            d = json.loads(line)
            cases.append(ContentCase(
                question=d["question"],
                fetch_url=d["fetch_url"],
                ground_truth=d["ground_truth"],
            ))
    return cases


def fetch_all_content(config: dict, cases: list[ContentCase]) -> list[ContentSample]:
    binary = os.path.abspath(config["mcp_server"]["binary"])
    sources = os.path.abspath(config["mcp_server"]["sources_file"])
    timeout = config["benchmark"].get("fetch_timeout", 30)

    samples: list[ContentSample] = []

    print(f"Starting MCP server: {binary}")
    with MCPClient(binary, sources_file=sources) as client:
        for case in cases:
            print(f"\nFetching: {case.fetch_url}")
            print(f"Question: {case.question}")

            try:
                result = client.fetch_content(case.fetch_url, timeout=timeout)
            except Exception as e:
                print(f"  ERROR: {e}")
                continue

            if not result or not result.get("content"):
                print("  WARNING: empty content returned")
                continue

            content = result["content"]
            ct = result.get("content_type", "?")
            print(f"  content_type={ct}  chars={len(content)}")

            # Split long content into chunks (ragas expects a list of context strings)
            chunks = split_content(content, chunk_size=2000)
            print(f"  chunks={len(chunks)}")

            samples.append(ContentSample(
                question=case.question,
                contexts=chunks,
                ground_truth=case.ground_truth,
            ))

    return samples


def split_content(text: str, chunk_size: int = 2000) -> list[str]:
    """Split text into overlapping chunks for ragas context evaluation."""
    if len(text) <= chunk_size:
        return [text]

    chunks = []
    step = chunk_size - 200  # 200-char overlap
    for i in range(0, len(text), step):
        chunk = text[i:i + chunk_size]
        if chunk.strip():
            chunks.append(chunk)
    return chunks


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", default="config.yaml")
    parser.add_argument("--dataset", default="datasets/content.jsonl")
    args = parser.parse_args()

    script_dir = Path(__file__).parent
    os.chdir(script_dir)

    with open(args.config) as f:
        config = yaml.safe_load(f)

    cases = load_cases(args.dataset)
    print(f"Loaded {len(cases)} content test cases")

    # Phase 1: fetch all content
    samples = fetch_all_content(config, cases)
    if not samples:
        print("\nNo content fetched — aborting.")
        sys.exit(1)

    print(f"\nFetched {len(samples)}/{len(cases)} cases successfully")

    # Phase 2: ragas evaluation
    print("\nInitialising LLM for ragas evaluation...")
    llm_wrapper = make_llm(config["llm"])

    print("Running ragas evaluation (this may take a few minutes)...\n")
    scores = run_ragas(samples, llm_wrapper)

    # Report
    print("=" * 60)
    print("RAGAS RESULTS")
    print("=" * 60)
    for metric, score in scores.items():
        label = {
            "LLMContextPrecisionWithReference": "Context Precision",
            "LLMContextRecall": "Context Recall",
        }.get(metric, metric)
        print(f"  {label:<30} {score:.4f}")

    print(f"\nEvaluated {len(samples)} samples")
    print("\nInterpretation:")
    print("  Context Precision: fraction of retrieved context that is relevant to the question")
    print("  Context Recall:    fraction of ground truth covered by retrieved context")


if __name__ == "__main__":
    main()
