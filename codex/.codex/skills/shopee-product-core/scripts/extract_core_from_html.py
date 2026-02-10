#!/usr/bin/env python3
from __future__ import annotations

import argparse
import gzip
import html
import json
import os
import re
from html.parser import HTMLParser
from typing import Any, Dict, List, Optional, Tuple


class TextExtractor(HTMLParser):
    def __init__(self) -> None:
        super().__init__()
        self.out: List[str] = []

    def handle_data(self, data: str) -> None:
        if data:
            self.out.append(data)

    def text(self) -> str:
        s = " ".join(self.out)
        s = re.sub(r"\s+", " ", s)
        return s.strip()


def read_text(path: str) -> str:
    if path.endswith(".gz"):
        with gzip.open(path, "rt", encoding="utf-8", errors="replace") as f:
            return f.read()
    with open(path, "r", encoding="utf-8", errors="replace") as f:
        return f.read()


def pick_html_path(artifact_dir: str, explicit: str) -> str:
    if explicit:
        return explicit

    candidates = [
        "s0-initial.html.gz",
        "s0-initial.html",
        "s0-page.html.gz",
        "s0-page.html",
    ]
    for name in candidates:
        p = os.path.join(artifact_dir, name)
        if os.path.exists(p):
            return p
    raise FileNotFoundError("no input html artifact found")


def load_manifest(artifact_dir: str, manifest_path: str) -> Dict[str, Any]:
    p = manifest_path or os.path.join(artifact_dir, "s0-manifest.json")
    if not p or not os.path.exists(p):
        return {}
    with open(p, "r", encoding="utf-8") as f:
        return json.load(f)


def clean(s: str, max_len: int = 0) -> str:
    s = html.unescape(s or "")
    s = re.sub(r"\s+", " ", s).strip()
    if max_len > 0:
        s = s[:max_len]
    return s


def first_non_empty(values: List[str]) -> str:
    for v in values:
        if v and v.strip():
            return v.strip()
    return ""


def extract_title(src_html: str, plain: str) -> str:
    m = re.search(r'<meta[^>]+property=["\']og:title["\'][^>]+content=["\']([^"\']+)', src_html, re.I)
    og = clean(m.group(1), 300) if m else ""

    m = re.search(r"<h1[^>]*>(.*?)</h1>", src_html, re.I | re.S)
    h1 = clean(re.sub(r"<[^>]+>", " ", m.group(1)), 300) if m else ""

    m = re.search(r"<title[^>]*>(.*?)</title>", src_html, re.I | re.S)
    title_tag = clean(re.sub(r"<[^>]+>", " ", m.group(1)), 300) if m else ""
    if "|" in title_tag:
        title_tag = clean(title_tag.split("|")[0], 300)

    return first_non_empty([h1, og, title_tag])


def extract_jsonld_products(src_html: str) -> List[Dict[str, Any]]:
    blocks = re.findall(r'<script[^>]+type=["\']application/ld\+json["\'][^>]*>(.*?)</script>', src_html, re.I | re.S)
    out: List[Dict[str, Any]] = []
    for b in blocks:
        b = b.strip()
        if not b:
            continue
        try:
            data = json.loads(b)
        except Exception:
            continue

        def walk(x: Any) -> None:
            if isinstance(x, dict):
                t = x.get("@type")
                if t == "Product" or (isinstance(t, list) and "Product" in t):
                    out.append(x)
                for v in x.values():
                    walk(v)
            elif isinstance(x, list):
                for v in x:
                    walk(v)

        walk(data)
    return out


def extract_description(src_html: str, plain: str, products: List[Dict[str, Any]]) -> str:
    for p in products:
        d = clean(str(p.get("description", "")), 1500)
        if len(d) >= 20:
            return d

    m = re.search(r"商品描述\s*(.*?)\s*(商品評價|賣場優惠券|你可能感興趣的商品|專屬推薦商品)", plain, re.S)
    if m:
        return clean(m.group(1), 1500)

    return ""


def extract_currency_price(plain: str, products: List[Dict[str, Any]]) -> Tuple[str, str]:
    for p in products:
        offers = p.get("offers")
        if isinstance(offers, list) and offers:
            offers = offers[0]
        if isinstance(offers, dict):
            c = clean(str(offers.get("priceCurrency", "")), 8).upper()
            price = clean(str(offers.get("price", "")), 64).replace(",", "")
            if c and price:
                return c, price

    m = re.search(r"(NT\$|TWD|NTD|\$)\s*([0-9][0-9,]*(?:\.[0-9]+)?)", plain, re.I)
    if m:
        c = "TWD" if m.group(1).upper() in {"NT$", "TWD", "NTD", "$"} else clean(m.group(1), 8).upper()
        price = m.group(2).replace(",", "")
        return c, price

    return "", ""


def detect_blocked(url: str, plain: str, manifest: Dict[str, Any]) -> bool:
    if isinstance(manifest, dict) and manifest.get("blocked") is True:
        return True
    joined = f"{url} {plain}".lower()
    return bool(re.search(r"captcha|驗證|驗證碼|robot|機器人|verification|security check|verify/captcha|verify/traffic", joined, re.I))


def build_result(artifact_dir: str, html_path: str, manifest_path: str) -> Dict[str, Any]:
    src_html = read_text(html_path)
    tx = TextExtractor()
    tx.feed(src_html)
    plain = tx.text()

    manifest = load_manifest(artifact_dir, manifest_path)
    url = ""
    if isinstance(manifest, dict):
        url = str(manifest.get("url", ""))

    products = extract_jsonld_products(src_html)
    title = extract_title(src_html, plain)
    description = extract_description(src_html, plain, products)
    currency, price = extract_currency_price(plain, products)

    blocked = detect_blocked(url, plain, manifest)
    if blocked and (not title or not price):
        return {
            "status": "needs_manual",
            "title": "",
            "description": "",
            "currency": "",
            "price": "",
            "notes": "blocked or verification wall detected from html/manifest",
            "error": "",
        }

    description = clean(description, 1500)
    title = clean(title, 300)
    currency = clean(currency, 8).upper()
    price = clean(price, 64)

    if not (title and description and currency and price):
        return {
            "status": "error",
            "title": title,
            "description": description,
            "currency": currency,
            "price": price,
            "notes": "",
            "error": "incomplete core fields from html artifact",
        }

    return {
        "status": "ok",
        "title": title,
        "description": description,
        "currency": currency,
        "price": price,
        "notes": "",
        "error": "",
    }


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description="Extract Shopee core fields from HTML artifact")
    p.add_argument("--artifact-dir", default="", help="Artifact directory")
    p.add_argument("--html-path", default="", help="Explicit html(.gz) path")
    p.add_argument("--manifest-path", default="", help="Explicit manifest path")
    p.add_argument("--output", default="", help="Output JSON path (default: <artifact_dir>/core_extract.json)")
    return p.parse_args()


def main() -> int:
    args = parse_args()
    artifact_dir = args.artifact_dir.strip()
    if not artifact_dir and args.html_path:
        artifact_dir = os.path.dirname(os.path.abspath(args.html_path))

    if not artifact_dir and not args.html_path:
        err_obj = {
            "status": "error",
            "title": "",
            "description": "",
            "currency": "",
            "price": "",
            "notes": "",
            "error": "artifact_dir or html_path is required",
        }
        print(json.dumps(err_obj, ensure_ascii=False))
        return 1

    try:
        html_path = pick_html_path(artifact_dir, args.html_path.strip())
        res = build_result(artifact_dir, html_path, args.manifest_path.strip())
        output = args.output.strip()
        if not output and artifact_dir:
            output = os.path.join(artifact_dir, "core_extract.json")
        if output:
            os.makedirs(os.path.dirname(output) or ".", exist_ok=True)
            with open(output, "w", encoding="utf-8") as f:
                json.dump(res, f, ensure_ascii=False)
        print(json.dumps(res, ensure_ascii=False))
        return 0 if res.get("status") != "error" else 1
    except Exception as e:
        err_obj = {
            "status": "error",
            "title": "",
            "description": "",
            "currency": "",
            "price": "",
            "notes": "",
            "error": str(e),
        }
        output = args.output.strip()
        if not output and artifact_dir:
            output = os.path.join(artifact_dir, "core_extract.json")
        if output:
            os.makedirs(os.path.dirname(output) or ".", exist_ok=True)
            with open(output, "w", encoding="utf-8") as f:
                json.dump(err_obj, f, ensure_ascii=False)
        print(json.dumps(err_obj, ensure_ascii=False))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
