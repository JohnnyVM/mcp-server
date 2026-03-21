"""Ragas-based content quality evaluation using an OpenAI-compatible LLM backend."""

import os
from dataclasses import dataclass

from langchain_openai import ChatOpenAI
from ragas import evaluate
from ragas.dataset_schema import EvaluationDataset, SingleTurnSample
from ragas.llms import LangchainLLMWrapper
from ragas.metrics import LLMContextPrecisionWithReference, LLMContextRecall


@dataclass
class ContentCase:
    question: str
    fetch_url: str
    ground_truth: str   # expected factual answer — used as reference by ragas


@dataclass
class ContentSample:
    question: str
    contexts: list[str]  # text chunks returned by fetch_content
    ground_truth: str


def make_llm(config: dict) -> LangchainLLMWrapper:
    api_key = os.environ.get(config["api_key_env"], "")
    if not api_key:
        raise EnvironmentError(
            f"LLM API key not set. Export {config['api_key_env']} before running."
        )
    llm = ChatOpenAI(
        model=config["model"],
        api_key=api_key,
        base_url=config["api_base"],
    )
    return LangchainLLMWrapper(llm)


def build_dataset(samples: list[ContentSample]) -> EvaluationDataset:
    return EvaluationDataset(samples=[
        SingleTurnSample(
            user_input=s.question,
            retrieved_contexts=s.contexts,
            reference=s.ground_truth,
        )
        for s in samples
    ])


def run_ragas(samples: list[ContentSample], llm_wrapper: LangchainLLMWrapper) -> dict:
    """Run context_precision and context_recall using the provided LLM.

    Returns a dict of metric_name -> score (0.0–1.0).
    """
    dataset = build_dataset(samples)
    metrics = [
        LLMContextPrecisionWithReference(llm=llm_wrapper),
        LLMContextRecall(llm=llm_wrapper),
    ]
    result = evaluate(dataset=dataset, metrics=metrics)
    return dict(result)
