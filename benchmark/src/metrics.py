"""Offline retrieval metrics — no LLM required."""

import math
from dataclasses import dataclass, field


@dataclass
class RetrievalCase:
    query: str
    source: str
    relevant_urls: list[str]           # any hit counts as relevant
    grades: dict[str, int] = field(default_factory=dict)  # url -> 0/1/2 for NDCG


@dataclass
class RetrievalResult:
    query: str
    source: str
    retrieved_urls: list[str]
    scores: dict[str, float]           # relevance_score per url from server


def precision_at_k(retrieved: list[str], relevant: set[str], k: int) -> float:
    top_k = retrieved[:k]
    hits = sum(1 for u in top_k if u in relevant)
    return hits / k if k else 0.0


def recall_at_k(retrieved: list[str], relevant: set[str], k: int) -> float:
    if not relevant:
        return 0.0
    top_k = set(retrieved[:k])
    return len(top_k & relevant) / len(relevant)


def ndcg_at_k(retrieved: list[str], grades: dict[str, int], k: int) -> float:
    """Normalized Discounted Cumulative Gain at k.
    grades maps url -> relevance integer (0 = irrelevant, 1 = partial, 2 = relevant).
    """
    dcg = sum(
        grades.get(url, 0) / math.log2(i + 2)
        for i, url in enumerate(retrieved[:k])
    )
    ideal = sorted(grades.values(), reverse=True)[:k]
    idcg = sum(rel / math.log2(i + 2) for i, rel in enumerate(ideal))
    return dcg / idcg if idcg else 0.0


def mrr(retrieved: list[str], relevant: set[str]) -> float:
    """Mean Reciprocal Rank — rank of the first relevant result."""
    for i, url in enumerate(retrieved):
        if url in relevant:
            return 1.0 / (i + 1)
    return 0.0


def score_correlation(retrieved_urls: list[str], server_scores: dict[str, float], relevant: set[str]) -> float:
    """Fraction of relevant URLs that the server ranked in the top half of results.
    Measures whether server-side relevance_score is a useful signal.
    """
    if not relevant or not retrieved_urls:
        return 0.0
    mid = len(retrieved_urls) // 2
    top_half = set(retrieved_urls[:mid])
    hits_in_top = sum(1 for u in relevant if u in top_half)
    return hits_in_top / len(relevant)


@dataclass
class RetrievalMetrics:
    precision_at_5: float = 0.0
    precision_at_10: float = 0.0
    recall_at_5: float = 0.0
    recall_at_10: float = 0.0
    ndcg_at_10: float = 0.0
    mrr: float = 0.0
    score_correlation: float = 0.0

    def __str__(self) -> str:
        return (
            f"P@5={self.precision_at_5:.3f}  P@10={self.precision_at_10:.3f}  "
            f"R@5={self.recall_at_5:.3f}  R@10={self.recall_at_10:.3f}  "
            f"NDCG@10={self.ndcg_at_10:.3f}  MRR={self.mrr:.3f}  "
            f"ScoreCorr={self.score_correlation:.3f}"
        )


def compute_retrieval_metrics(case: RetrievalCase, result: RetrievalResult) -> RetrievalMetrics:
    relevant = set(case.relevant_urls)
    grades = case.grades or {u: 1 for u in relevant}
    retrieved = result.retrieved_urls

    return RetrievalMetrics(
        precision_at_5=precision_at_k(retrieved, relevant, 5),
        precision_at_10=precision_at_k(retrieved, relevant, 10),
        recall_at_5=recall_at_k(retrieved, relevant, 5),
        recall_at_10=recall_at_k(retrieved, relevant, 10),
        ndcg_at_10=ndcg_at_k(retrieved, grades, 10),
        mrr=mrr(retrieved, relevant),
        score_correlation=score_correlation(retrieved, result.scores, relevant),
    )


def average_metrics(all_metrics: list[RetrievalMetrics]) -> RetrievalMetrics:
    n = len(all_metrics)
    if not n:
        return RetrievalMetrics()
    return RetrievalMetrics(
        precision_at_5=sum(m.precision_at_5 for m in all_metrics) / n,
        precision_at_10=sum(m.precision_at_10 for m in all_metrics) / n,
        recall_at_5=sum(m.recall_at_5 for m in all_metrics) / n,
        recall_at_10=sum(m.recall_at_10 for m in all_metrics) / n,
        ndcg_at_10=sum(m.ndcg_at_10 for m in all_metrics) / n,
        mrr=sum(m.mrr for m in all_metrics) / n,
        score_correlation=sum(m.score_correlation for m in all_metrics) / n,
    )
