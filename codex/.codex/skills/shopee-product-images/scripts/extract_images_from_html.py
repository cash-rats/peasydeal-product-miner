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


def clean_url(url: str) -> str:
    u = url.replace('\\/', '/').strip().strip('"\'')
    return u


def extract_image_urls(src: str) -> List[str]:
    urls: List[str] = []
    seen = set()

    patterns = [
        r'https?://[^"\'\s>]*susercontent\.com/file/[^"\'\s>]*',
        r'https?://[^"\'\s>]*img\.susercontent\.com/file/[^"\'\s>]*',
    ]

    for pat in patterns:
        for m in re.finditer(pat, src, flags=re.I):
            u = clean_url(m.group(0))
            if not u.lower().startswith(('http://', 'https://')):
                continue
            if u in seen:
                continue
            seen.add(u)
            urls.append(u)
            if len(urls) >= MAX_IMAGES:
                return urls

    # Fallback: generic image extensions in img tags or script blobs.
    generic_pat = r'https?://[^"\'\s>]+\.(?:jpg|jpeg|png|webp)(?:\?[^"\'\s>]*)?'
    for m in re.finditer(generic_pat, src, flags=re.I):
        u = clean_url(m.group(0))
        if not u.lower().startswith(('http://', 'https://')):
            continue
        if u in seen:
            continue
        seen.add(u)
        urls.append(u)
        if len(urls) >= MAX_IMAGES:
            break

    return urls[:MAX_IMAGES]


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description='Extract Shopee image URLs from HTML artifact')
    p.add_argument('--artifact-dir', default='', help='Artifact directory')
    p.add_argument('--html-path', default='', help='Explicit html(.gz) path')
    p.add_argument('--output', default='', help='Output JSON path (default: <artifact_dir>/images_extract.json)')
    return p.parse_args()


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
            os.makedirs(os.path.dirname(output) or '.', exist_ok=True)
            with open(output, 'w', encoding='utf-8') as f:
                json.dump(res, f, ensure_ascii=False)
        print(json.dumps(res, ensure_ascii=False))
        return 1

    try:
        html_path = pick_input_path(artifact_dir, html_path)
        src = read_text(html_path)
        images = extract_image_urls(src)
        res = {'status': 'ok', 'images': images, 'error': ''}
        if output:
            os.makedirs(os.path.dirname(output) or '.', exist_ok=True)
            with open(output, 'w', encoding='utf-8') as f:
                json.dump(res, f, ensure_ascii=False)
        print(json.dumps(res, ensure_ascii=False))
        return 0
    except Exception as e:
        res = {'status': 'error', 'images': [], 'error': str(e)}
        if output:
            os.makedirs(os.path.dirname(output) or '.', exist_ok=True)
            with open(output, 'w', encoding='utf-8') as f:
                json.dump(res, f, ensure_ascii=False)
        print(json.dumps(res, ensure_ascii=False))
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
