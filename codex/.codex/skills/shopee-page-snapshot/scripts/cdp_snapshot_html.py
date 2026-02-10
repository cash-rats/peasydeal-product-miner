#!/usr/bin/env python3
"""Capture raw HTML from a Chrome DevTools page target via CDP.

This script is intentionally dependency-free (Python stdlib only) so it can run
in constrained environments.
"""

from __future__ import annotations

import argparse
import base64
import gzip
import hashlib
import json
import os
import secrets
import socket
import struct
import sys
import urllib.parse
import urllib.request
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional, Tuple


class CDPError(RuntimeError):
    pass


@dataclass
class SnapshotResult:
    target_id: str
    target_url: str
    output: str
    bytes_written: int
    sha256: str
    captured_at: str
    truncated: bool
    original_bytes: int


def _utc_now_iso() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def _http_get_json(url: str, timeout: float = 10.0) -> Any:
    req = urllib.request.Request(url, headers={"Accept": "application/json"})
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.loads(resp.read().decode("utf-8", errors="replace"))


def fetch_targets(browser_url: str) -> List[Dict[str, Any]]:
    base = browser_url.rstrip("/")
    return _http_get_json(f"{base}/json/list")


def pick_target(
    targets: List[Dict[str, Any]],
    target_id: str = "",
    url_contains: str = "",
) -> Dict[str, Any]:
    pages = [t for t in targets if t.get("type") == "page" and t.get("webSocketDebuggerUrl")]
    if not pages:
        raise CDPError("no page targets with webSocketDebuggerUrl found")

    target_id = target_id.strip()
    if target_id:
        for t in pages:
            if t.get("id") == target_id:
                return t
        raise CDPError(f"target_id not found: {target_id}")

    needle = url_contains.strip().lower()
    if needle:
        for t in pages:
            if needle in str(t.get("url", "")).lower():
                return t
        raise CDPError(f"no page target url contains: {url_contains}")

    for t in pages:
        u = str(t.get("url", "")).lower()
        if u and not u.startswith("devtools://") and u != "about:blank":
            return t
    return pages[0]


def _ws_connect(ws_url: str, timeout: float = 10.0) -> socket.socket:
    parsed = urllib.parse.urlparse(ws_url)
    if parsed.scheme != "ws":
        raise CDPError(f"unsupported websocket scheme: {parsed.scheme}")

    host = parsed.hostname or ""
    port = parsed.port or 80
    path = parsed.path or "/"
    if parsed.query:
        path = f"{path}?{parsed.query}"

    sock = socket.create_connection((host, port), timeout=timeout)

    sec_key = base64.b64encode(secrets.token_bytes(16)).decode("ascii")
    req = (
        f"GET {path} HTTP/1.1\r\n"
        f"Host: {host}:{port}\r\n"
        "Upgrade: websocket\r\n"
        "Connection: Upgrade\r\n"
        f"Sec-WebSocket-Key: {sec_key}\r\n"
        "Sec-WebSocket-Version: 13\r\n"
        "\r\n"
    )
    sock.sendall(req.encode("ascii"))

    response = b""
    while b"\r\n\r\n" not in response:
        chunk = sock.recv(4096)
        if not chunk:
            break
        response += chunk
        if len(response) > 65536:
            break

    text = response.decode("latin1", errors="replace")
    if " 101 " not in text and not text.startswith("HTTP/1.1 101"):
        sock.close()
        raise CDPError(f"websocket handshake failed: {text.splitlines()[:1]}")

    return sock


def _ws_send_text(sock: socket.socket, text: str) -> None:
    payload = text.encode("utf-8")
    first = 0x80 | 0x1
    mask_bit = 0x80
    length = len(payload)

    header = bytearray([first])
    if length < 126:
        header.append(mask_bit | length)
    elif length < (1 << 16):
        header.append(mask_bit | 126)
        header.extend(struct.pack("!H", length))
    else:
        header.append(mask_bit | 127)
        header.extend(struct.pack("!Q", length))

    mask = secrets.token_bytes(4)
    header.extend(mask)
    masked = bytes(b ^ mask[i % 4] for i, b in enumerate(payload))

    sock.sendall(bytes(header) + masked)


def _recv_exact(sock: socket.socket, n: int) -> bytes:
    out = bytearray()
    while len(out) < n:
        chunk = sock.recv(n - len(out))
        if not chunk:
            raise CDPError("socket closed while reading websocket frame")
        out.extend(chunk)
    return bytes(out)


def _ws_recv_text(sock: socket.socket) -> str:
    chunks: List[bytes] = []
    opcode = None

    while True:
        hdr = _recv_exact(sock, 2)
        b1, b2 = hdr[0], hdr[1]
        fin = (b1 & 0x80) != 0
        op = b1 & 0x0F
        masked = (b2 & 0x80) != 0
        length = b2 & 0x7F

        if length == 126:
            length = struct.unpack("!H", _recv_exact(sock, 2))[0]
        elif length == 127:
            length = struct.unpack("!Q", _recv_exact(sock, 8))[0]

        mask = _recv_exact(sock, 4) if masked else b""
        payload = _recv_exact(sock, length)
        if masked:
            payload = bytes(b ^ mask[i % 4] for i, b in enumerate(payload))

        if op == 0x8:  # close
            raise CDPError("websocket closed by peer")
        if op == 0x9:  # ping
            _ws_send_control(sock, 0xA, payload)
            continue
        if op == 0xA:  # pong
            continue
        if op in (0x1, 0x0):
            if opcode is None and op == 0x1:
                opcode = op
            chunks.append(payload)
            if fin:
                break
            continue

    return b"".join(chunks).decode("utf-8", errors="replace")


def _ws_send_control(sock: socket.socket, opcode: int, payload: bytes = b"") -> None:
    if len(payload) > 125:
        payload = payload[:125]
    first = 0x80 | (opcode & 0x0F)
    mask_bit = 0x80
    mask = secrets.token_bytes(4)
    header = bytearray([first, mask_bit | len(payload)])
    header.extend(mask)
    masked = bytes(b ^ mask[i % 4] for i, b in enumerate(payload))
    sock.sendall(bytes(header) + masked)


def cdp_eval(ws_url: str, expression: str, timeout: float = 20.0) -> Any:
    sock = _ws_connect(ws_url, timeout=timeout)
    sock.settimeout(timeout)
    try:
        cmd_id = 1
        req = {
            "id": cmd_id,
            "method": "Runtime.evaluate",
            "params": {
                "expression": expression,
                "returnByValue": True,
                "awaitPromise": False,
            },
        }
        _ws_send_text(sock, json.dumps(req, ensure_ascii=False))

        while True:
            msg = _ws_recv_text(sock)
            data = json.loads(msg)
            if data.get("id") != cmd_id:
                continue
            if "error" in data:
                raise CDPError(f"cdp error: {data['error']}")
            res = data.get("result", {})
            if "exceptionDetails" in res:
                raise CDPError(f"runtime exception: {res['exceptionDetails']}")
            value = (((res.get("result") or {}).get("value")))
            return value
    finally:
        try:
            _ws_send_control(sock, 0x8)
        except Exception:
            pass
        sock.close()


def maybe_truncate(content: str, max_bytes: int) -> Tuple[str, bool, int]:
    raw = content.encode("utf-8")
    original = len(raw)
    if max_bytes <= 0 or original <= max_bytes:
        return content, False, original

    truncated = raw[:max_bytes].decode("utf-8", errors="ignore")
    return truncated, True, original


def write_html(path: str, content: str, force_gzip: bool = False) -> Tuple[int, str]:
    os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
    use_gzip = force_gzip or path.endswith(".gz")
    if use_gzip:
        with gzip.open(path, "wb") as f:
            f.write(content.encode("utf-8"))
    else:
        with open(path, "w", encoding="utf-8") as f:
            f.write(content)

    with open(path, "rb") as f:
        data = f.read()
    return len(data), hashlib.sha256(data).hexdigest()


def capture_snapshot(
    browser_url: str,
    output: str,
    target_id: str,
    url_contains: str,
    max_bytes: int,
    gzip_output: bool,
) -> SnapshotResult:
    targets = fetch_targets(browser_url)
    target = pick_target(targets, target_id=target_id, url_contains=url_contains)
    ws_url = str(target.get("webSocketDebuggerUrl", ""))
    if not ws_url:
        raise CDPError("selected target missing webSocketDebuggerUrl")

    html = cdp_eval(ws_url, "document.documentElement ? document.documentElement.outerHTML : ''")
    if not isinstance(html, str):
        raise CDPError("Runtime.evaluate did not return html string")

    html, truncated, original_bytes = maybe_truncate(html, max_bytes)
    bytes_written, sha256 = write_html(output, html, force_gzip=gzip_output)

    return SnapshotResult(
        target_id=str(target.get("id", "")),
        target_url=str(target.get("url", "")),
        output=output,
        bytes_written=bytes_written,
        sha256=sha256,
        captured_at=_utc_now_iso(),
        truncated=truncated,
        original_bytes=original_bytes,
    )


def parse_args(argv: Optional[List[str]] = None) -> argparse.Namespace:
    p = argparse.ArgumentParser(description="Capture HTML snapshot from Chrome CDP")
    p.add_argument("--browser-url", default="http://127.0.0.1:9222", help="Chrome remote debug base URL")
    p.add_argument("--output", required=True, help="Output html path (use .gz for gzip)")
    p.add_argument("--target-id", default="", help="Exact target id")
    p.add_argument("--url-contains", default="", help="Pick target whose url contains this text")
    p.add_argument("--max-bytes", type=int, default=0, help="Optional UTF-8 byte cap for html; 0=unlimited")
    p.add_argument("--gzip", action="store_true", help="Force gzip output even if path does not end with .gz")
    return p.parse_args(argv)


def main(argv: Optional[List[str]] = None) -> int:
    args = parse_args(argv)
    try:
        res = capture_snapshot(
            browser_url=args.browser_url,
            output=args.output,
            target_id=args.target_id,
            url_contains=args.url_contains,
            max_bytes=max(0, int(args.max_bytes)),
            gzip_output=bool(args.gzip),
        )
    except Exception as e:
        print(json.dumps({"status": "error", "error": str(e)}, ensure_ascii=False))
        return 1

    out = {
        "status": "ok",
        "captured_at": res.captured_at,
        "target_id": res.target_id,
        "target_url": res.target_url,
        "output": res.output,
        "bytes": res.bytes_written,
        "sha256": res.sha256,
        "truncated": res.truncated,
        "original_bytes": res.original_bytes,
    }
    print(json.dumps(out, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
