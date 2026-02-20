#!/usr/bin/env python3
from __future__ import annotations

import argparse
import gzip
import json
import os
import re
from typing import List

MAX_IMAGES = 20


def read_text(path: str) -> str:
    if path.endswith('.gz'):
        with gzip.open(path, 'rt', encoding='utf-8', errors='replace') as f:
            return f.read()
    with open(path, 'r', encoding='utf-8', errors='replace') as f:
        return f.read()


def pick_input_path(artifact_dir: str, explicit_path: str) -> str:
    if explicit_path:
        return explicit_path
    candidates = [
        's0-overlay.html.gz',
        's0-overlay.html',
        's0-initial.html.gz',
        's0-initial.html',
        's0-page.html.gz',
        's0-page.html',
    ]
    for name in candidates:
        p = os.path.join(artifact_dir, name)
        if os.path.exists(p):
            return p
    raise FileNotFoundError('no html input artifact found')


def normalize_url(url: str) -> str:
    u = (url or '').replace('\\/', '/').strip().strip('"\'')
    if u.startswith('//'):
        u = 'https:' + u
    return u


def is_valid_http_url(url: str) -> bool:
    return bool(url) and url.lower().startswith(('http://', 'https://'))


def add_url(urls: List[str], seen: set, raw: str) -> bool:
    u = normalize_url(raw)
    if not is_valid_http_url(u):
        return False
    if u in seen:
        return False
    seen.add(u)
    urls.append(u)
    return len(urls) >= MAX_IMAGES


def extract_urls_from_array_block(block: str) -> List[str]:
    out: List[str] = []
    for m in re.finditer(r'"((?:https?:)?//[^"\s]+)"', block, flags=re.I):
        out.append(m.group(1))
    return out


def extract_primary_image_urls(src: str, urls: List[str], seen: set) -> bool:
    # Prefer item-level gallery images from Taobao SSR payload.
    patterns = [
        r'"item"\s*:\s*\{.{0,200000}?"images"\s*:\s*\[(.*?)\]',
        r'"headImageVO"\s*:\s*\{.{0,20000}?"images"\s*:\s*\[(.*?)\]',
    ]

    for pat in patterns:
        for m in re.finditer(pat, src, flags=re.I | re.S):
            for raw in extract_urls_from_array_block(m.group(1)):
                if add_url(urls, seen, raw):
                    return True
    return False


def extract_fallback_image_urls(src: str, urls: List[str], seen: set) -> None:
    # High-confidence product image CDNs for Taobao payload.
    patterns = [
        r'"((?:https?:)?//img\.alicdn\.com/imgextra/[^"\s]+)"',
        r'"((?:https?:)?//gw\.alicdn\.com/bao/uploaded/[^"\s]+)"',
    ]

    for pat in patterns:
        for m in re.finditer(pat, src, flags=re.I):
            if add_url(urls, seen, m.group(1)):
                return

    # Last fallback: generic image links in markup/scripts.
    generic_pat = r'((?:https?:)?//[^"\'\s>]+\.(?:jpg|jpeg|png|webp)(?:\?[^"\'\s>]*)?)'
    for m in re.finditer(generic_pat, src, flags=re.I):
        if add_url(urls, seen, m.group(1)):
            return


def extract_image_urls(src: str) -> List[str]:
    urls: List[str] = []
    seen = set()

    full = extract_primary_image_urls(src, urls, seen)
    # Keep primary gallery images as authoritative when available.
    if urls:
        return urls[:MAX_IMAGES]

    if not full and len(urls) < MAX_IMAGES:
        extract_fallback_image_urls(src, urls, seen)

    return urls[:MAX_IMAGES]


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description='Extract Taobao image URLs from HTML artifact')
    p.add_argument('--artifact-dir', default='', help='Artifact directory')
    p.add_argument('--html-path', default='', help='Explicit html(.gz) path')
    p.add_argument('--output', default='', help='Output JSON path (default: <artifact_dir>/images_extract.json)')
    return p.parse_args()


def write_json(path: str, obj: dict) -> None:
    os.makedirs(os.path.dirname(path) or '.', exist_ok=True)
    with open(path, 'w', encoding='utf-8') as f:
        json.dump(obj, f, ensure_ascii=False)


def main() -> int:
    args = parse_args()
    artifact_dir = args.artifact_dir.strip()
    html_path = args.html_path.strip()

    if not artifact_dir and html_path:
        artifact_dir = os.path.dirname(os.path.abspath(html_path))

    output = args.output.strip()
    if not output and artifact_dir:
        output = os.path.join(artifact_dir, 'images_extract.json')

    if not artifact_dir and not html_path:
        res = {'status': 'error', 'images': [], 'error': 'artifact_dir or html_path is required'}
        if output:
            write_json(output, res)
        print(json.dumps(res, ensure_ascii=False))
        return 1

    try:
        html_path = pick_input_path(artifact_dir, html_path)
        src = read_text(html_path)
        images = extract_image_urls(src)
        res = {'status': 'ok', 'images': images, 'error': ''}
        if output:
            write_json(output, res)
        print(json.dumps(res, ensure_ascii=False))
        return 0
    except Exception as e:
        res = {'status': 'error', 'images': [], 'error': str(e)}
        if output:
            write_json(output, res)
        print(json.dumps(res, ensure_ascii=False))
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
