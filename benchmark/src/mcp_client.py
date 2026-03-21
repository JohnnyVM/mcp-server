"""Minimal MCP stdio client for benchmarking the SearchAgent server."""

import json
import os
import queue
import subprocess
import threading
from typing import Any


class MCPClient:
    """Drives the SearchAgent MCP server over JSON-RPC / stdio."""

    def __init__(self, binary: str, sources_file: str | None = None, timeout: int = 30):
        env = dict(os.environ)
        if sources_file:
            env["SOURCES_FILE"] = sources_file

        self._timeout = timeout
        self._next_id = 0
        self._pending: dict[int, queue.Queue] = {}
        self._lock = threading.Lock()

        self._proc = subprocess.Popen(
            [binary],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            env=env,
            text=True,
            bufsize=1,
        )

        self._reader = threading.Thread(target=self._read_loop, daemon=True)
        self._reader.start()
        self._handshake()

    # ------------------------------------------------------------------ #
    # Transport                                                            #
    # ------------------------------------------------------------------ #

    def _read_loop(self) -> None:
        assert self._proc.stdout
        for line in self._proc.stdout:
            line = line.strip()
            if not line:
                continue
            try:
                msg = json.loads(line)
            except json.JSONDecodeError:
                continue
            req_id = msg.get("id")
            if req_id is not None:
                with self._lock:
                    q = self._pending.get(req_id)
                if q is not None:
                    q.put(msg)

    def _send(self, msg: dict) -> None:
        assert self._proc.stdin
        self._proc.stdin.write(json.dumps(msg) + "\n")
        self._proc.stdin.flush()

    def _rpc(self, method: str, params: dict | None = None, timeout: int | None = None) -> Any:
        with self._lock:
            self._next_id += 1
            req_id = self._next_id
            q: queue.Queue = queue.Queue()
            self._pending[req_id] = q

        self._send({"jsonrpc": "2.0", "id": req_id, "method": method, "params": params or {}})

        try:
            response = q.get(timeout=timeout or self._timeout)
        finally:
            with self._lock:
                self._pending.pop(req_id, None)

        if "error" in response:
            raise RuntimeError(f"MCP error [{method}]: {response['error']}")
        return response.get("result")

    def _notify(self, method: str, params: dict | None = None) -> None:
        self._send({"jsonrpc": "2.0", "method": method, "params": params or {}})

    def _handshake(self) -> None:
        self._rpc("initialize", {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "benchmark", "version": "1.0"},
        })
        self._notify("notifications/initialized")

    # ------------------------------------------------------------------ #
    # Tool helpers                                                         #
    # ------------------------------------------------------------------ #

    def _tool(self, name: str, arguments: dict, timeout: int | None = None) -> Any:
        result = self._rpc("tools/call", {"name": name, "arguments": arguments}, timeout=timeout)
        if result is None:
            return None
        for item in result.get("content", []):
            if item.get("type") == "text":
                try:
                    return json.loads(item["text"])
                except json.JSONDecodeError:
                    return item["text"]
        return None

    def list_sources(self, category: str = "") -> list[dict]:
        args: dict = {}
        if category:
            args["category"] = category
        return self._tool("list_sources", args) or []

    def query_source(self, source: str, query: str) -> dict:
        return self._tool("query_source", {"source": source, "query": query}) or {}

    def fetch_metadata(self, url: str) -> dict:
        return self._tool("fetch_metadata", {"url": url}) or {}

    def fetch_content(self, url: str, timeout: int | None = None) -> dict:
        return self._tool("fetch_content", {"url": url}, timeout=timeout) or {}

    # ------------------------------------------------------------------ #
    # Lifecycle                                                            #
    # ------------------------------------------------------------------ #

    def close(self) -> None:
        try:
            if self._proc.stdin:
                self._proc.stdin.close()
            self._proc.wait(timeout=5)
        except Exception:
            self._proc.kill()

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()
