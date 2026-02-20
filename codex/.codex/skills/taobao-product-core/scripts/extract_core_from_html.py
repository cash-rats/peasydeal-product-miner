#!/usr/bin/env python3
from __future__ import annotations

import argparse
import gzip
import html
import json
import os
import re
from html.parser import HTMLParser
from typing import Any, Dict, List, Tuple


class TextExtractor(HTMLParser):
    def __init__(self) -> None:
        super().__init__()
        self.out: List[str] = []

    def handle_data(self, data: str) -> None:
        if data:
            self.out.append(data)

    def text(self) -> str:
        s = " ".join(self.out)
        return re.sub(r"\s+", " ", s).strip()


def read_text(path: str) -> str:
    if path.endswith(".gz"):
        with gzip.open(path, "rt", encoding="utf-8", errors="replace") as f:
            return f.read()
    with open(path, "r", encoding="utf-8", errors="replace") as f:
        return f.read()


def read_json(path: str) -> Dict[str, Any]:
    if not path or not os.path.exists(path):
        return {}
    with open(path, "r", encoding="utf-8") as f:
        obj = json.load(f)
    return obj if isinstance(obj, dict) else {}


def clean(s: str, max_len: int = 0) -> str:
    s = html.unescape(s or "")
    s = re.sub(r"\s+", " ", s).strip()
    if max_len > 0:
        s = s[:max_len]
    return s


def first_non_empty(values: List[str]) -> str:
    for v in values:
        v = clean(v)
        if v:
            return v
    return ""


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


def resolve_metadata_paths(
    artifact_dir: str,
    manifest_path: str,
    pointer_path: str,
) -> Tuple[str, str]:
    mp = manifest_path.strip()
    pp = pointer_path.strip()

    if not mp:
        p = os.path.join(artifact_dir, "s0-manifest.json")
        if os.path.exists(p):
            mp = p
    if not pp:
        p = os.path.join(artifact_dir, "s0-snapshot-pointer.json")
        if os.path.exists(p):
            pp = p

    return mp, pp


def extract_property_pairs(src_html: str) -> List[Tuple[str, str]]:
    out: List[Tuple[str, str]] = []
    seen = set()

    def add_pair(name: str, value: str) -> None:
        n = clean(name, 120)
        v = clean(value, 500)
        if not n or not v:
            return
        k = (n.lower(), v.lower())
        if k in seen:
            return
        seen.add(k)
        out.append((n, v))

    for m in re.finditer(r'\{"valueName":"([^"]{1,600})","propertyName":"([^"]{1,120})"\}', src_html, flags=re.I):
        add_pair(m.group(2), m.group(1))
        if len(out) >= 20:
            return out

    for m in re.finditer(r'\{"text":\["([^"]{1,600})"\],"title":"([^"]{1,120})"\}', src_html, flags=re.I):
        add_pair(m.group(2), m.group(1))
        if len(out) >= 20:
            return out

    for m in re.finditer(r'\{"propertyName":"([^"]{1,120})","valueName":"([^"]{1,600})"\}', src_html, flags=re.I):
        add_pair(m.group(1), m.group(2))
        if len(out) >= 20:
            return out

    for m in re.finditer(r'\{"title":"([^"]{1,120})","text":\["([^"]{1,600})"\]\}', src_html, flags=re.I):
        add_pair(m.group(1), m.group(2))
        if len(out) >= 20:
            return out

    return out


def extract_title(src_html: str) -> str:
    m = re.search(r"<title[^>]*>(.*?)</title>", src_html, re.I | re.S)
    title_tag = clean(re.sub(r"<[^>]+>", " ", m.group(1)), 300) if m else ""
    title_tag = re.sub(r"\s*[-_|]\s*(淘宝网|天猫|taobao).*?$", "", title_tag, flags=re.I).strip()

    patterns = [
        r'"item"\s*:\s*\{.{0,120000}?"title":"([^"]{2,300})"\s*,\s*"itemId"',
        r'"titleVO"\s*:\s*\{.{0,12000}?"title"\s*:\s*\{\s*"title":"([^"]{2,300})"',
        r'"pcBuyParams"\s*:\s*\{.{0,12000}?"title":"([^"]{2,300})"',
    ]

    cands: List[str] = [title_tag]
    for pat in patterns:
        m = re.search(pat, src_html, re.I | re.S)
        if m:
            cands.append(m.group(1))

    return first_non_empty(cands)


def extract_description(src_html: str, title: str, plain: str) -> str:
    # Preferred: structured parameter pairs from Taobao SSR payload.
    props = extract_property_pairs(src_html)

    desc_parts: List[str] = []
    for key, value in props:
        value = clean(value, 180)
        if value:
            desc_parts.append(f"{key}: {value}")
        if len(desc_parts) >= 8:
            break

    if desc_parts:
        return clean("; ".join(desc_parts), 1500)

    # Fallback: seller + sales info from payload.
    shop = ""
    sales = ""
    m = re.search(r'"shopName":"([^"]{1,200})"', src_html, re.I)
    if m:
        shop = clean(m.group(1), 120)
    m = re.search(r'"vagueSellCount":"([^"]{1,80})"', src_html, re.I)
    if m:
        sales = clean(m.group(1), 40)

    fallback = []
    if shop:
        fallback.append(f"店铺: {shop}")
    if sales:
        fallback.append(f"销量: {sales}")
    if title:
        fallback.append(f"商品标题: {title}")

    if fallback:
        return clean("; ".join(fallback), 1500)

    # Last resort: keep it non-empty but bounded.
    if plain:
        return clean(plain[:300], 1500)
    return ""


def normalize_price_text(raw: str) -> str:
    t = clean(raw).replace(",", "")
    # Examples: "119", "119起", "￥119.00"
    m = re.search(r"([0-9]+(?:\.[0-9]{1,2})?)", t)
    if not m:
        return ""
    return m.group(1)


def extract_currency_price(src_html: str, plain: str) -> Tuple[str, str]:
    unit = ""
    price = ""

    # Primary: priceVO in Taobao SSR JSON payload.
    m = re.search(
        r'"priceVO"\s*:\s*\{.{0,6000}?"extraPrice"\s*:\s*\{.{0,1200}?"priceUnit":"([^"]{1,8})".{0,1200}?"priceText":"([^"]{1,32})"',
        src_html,
        re.I | re.S,
    )
    if m:
        unit = clean(m.group(1), 8)
        price = normalize_price_text(m.group(2))

    if not price:
        m = re.search(
            r'"subPrice"\s*:\s*\{.{0,1200}?"priceText":"([^"]{1,32})"',
            src_html,
            re.I | re.S,
        )
        if m:
            price = normalize_price_text(m.group(1))

    if not unit:
        m = re.search(r'"priceUnit":"([^"]{1,8})"', src_html, re.I)
        if m:
            unit = clean(m.group(1), 8)

    if not price:
        m = re.search(r'[￥¥]\s*([0-9][0-9,]*(?:\.[0-9]+)?)', plain, re.I)
        if m:
            price = normalize_price_text(m.group(1))
            if not unit:
                unit = "￥"

    if not price:
        m = re.search(r'"priceMoney":"([0-9]{2,})"', src_html, re.I)
        if m:
            cents = m.group(1)
            if len(cents) > 2:
                price = normalize_price_text(f"{cents[:-2]}.{cents[-2:]}")

    currency = ""
    if unit in {"￥", "¥"}:
        currency = "CNY"
    elif re.fullmatch(r"[A-Za-z]{3}", unit or ""):
        currency = unit.upper()
    elif re.search(r"(CNY|RMB|人民币|元)", f"{unit} {plain}", re.I):
        currency = "CNY"
    elif re.search(r"(TWD|NT\$|NTD)", f"{unit} {plain}", re.I):
        currency = "TWD"

    return clean(currency, 8).upper(), clean(price, 64)


def detect_blocked(url: str, plain: str, manifest: Dict[str, Any], pointer: Dict[str, Any]) -> Tuple[bool, str]:
    for src in (pointer, manifest):
        if not isinstance(src, dict):
            continue
        status = str(src.get("status", "")).strip().lower()
        if status == "needs_manual":
            notes = clean(str(src.get("notes", "")), 300)
            return True, notes or "blocked or verification wall detected from manifest/pointer"
        if src.get("blocked") is True:
            notes = clean(str(src.get("notes", "")), 300)
            return True, notes or "blocked flag detected from manifest/pointer"

    joined = clean(f"{url} {plain}", 4000).lower()
    if re.search(
        r"(captcha|verification|security check|verify|havanaone/login|x5secdata|请登录|登入|登录|验证码|安全验证|人机验证)",
        joined,
        re.I,
    ):
        return True, "blocked or verification wall detected from html"

    return False, ""


def build_result(artifact_dir: str, html_path: str, manifest_path: str, pointer_path: str) -> Dict[str, Any]:
    src_html = read_text(html_path)
    tx = TextExtractor()
    tx.feed(src_html)
    plain = tx.text()

    manifest = read_json(manifest_path)
    pointer = read_json(pointer_path)

    url = ""
    for src in (pointer, manifest):
        if isinstance(src, dict) and src.get("url"):
            url = clean(str(src.get("url")), 500)
            break

    blocked, blocked_note = detect_blocked(url, plain, manifest, pointer)
    if blocked:
        return {
            "status": "needs_manual",
            "title": "",
            "description": "",
            "currency": "",
            "price": "",
            "notes": blocked_note,
            "error": "",
        }

    title = clean(extract_title(src_html), 300)
    description = clean(extract_description(src_html, title, plain), 1500)
    currency, price = extract_currency_price(src_html, plain)

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
    p = argparse.ArgumentParser(description="Extract Taobao core fields from HTML artifact")
    p.add_argument("--artifact-dir", default="", help="Artifact directory")
    p.add_argument("--html-path", default="", help="Explicit html(.gz) path")
    p.add_argument("--manifest-path", default="", help="Explicit manifest path")
    p.add_argument("--pointer-path", default="", help="Explicit pointer path")
    p.add_argument("--output", default="", help="Output JSON path (default: <artifact_dir>/core_extract.json)")
    return p.parse_args()


def main() -> int:
    args = parse_args()
    artifact_dir = args.artifact_dir.strip()
    html_path = args.html_path.strip()

    if not artifact_dir and html_path:
        artifact_dir = os.path.dirname(os.path.abspath(html_path))

    if not artifact_dir and not html_path:
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

    output = args.output.strip()
    if not output and artifact_dir:
        output = os.path.join(artifact_dir, "core_extract.json")

    try:
        html_path = pick_html_path(artifact_dir, html_path)
        manifest_path, pointer_path = resolve_metadata_paths(
            artifact_dir,
            args.manifest_path,
            args.pointer_path,
        )
        res = build_result(artifact_dir, html_path, manifest_path, pointer_path)
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
        if output:
            os.makedirs(os.path.dirname(output) or ".", exist_ok=True)
            with open(output, "w", encoding="utf-8") as f:
                json.dump(err_obj, f, ensure_ascii=False)
        print(json.dumps(err_obj, ensure_ascii=False))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
