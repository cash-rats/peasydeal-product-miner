#!/usr/bin/env python3
from __future__ import annotations

import argparse
import gzip
import html
import json
import os
import re
from typing import Dict, List, Tuple

MAX_VARIATIONS = 20


def read_text(path: str) -> str:
    if path.endswith('.gz'):
        with gzip.open(path, 'rt', encoding='utf-8', errors='replace') as f:
            return f.read()
    with open(path, 'r', encoding='utf-8', errors='replace') as f:
        return f.read()


def pick_primary_html(artifact_dir: str, explicit_path: str) -> str:
    if explicit_path:
        return explicit_path
    candidates = [
        's0-initial.html.gz',
        's0-initial.html',
        's0-page.html.gz',
        's0-page.html',
    ]
    for name in candidates:
        p = os.path.join(artifact_dir, name)
        if os.path.exists(p):
            return p
    raise FileNotFoundError('no primary html artifact found')


def clean(s: str) -> str:
    s = html.unescape(s or '')
    s = re.sub(r'\s+', ' ', s).strip()
    return s


def is_option_like(s: str) -> bool:
    if not s:
        return False
    if len(s) > 50:
        return False
    # Common shoee variation-like patterns.
    if re.search(r'(US\s*\d|EU\s*\d|\d+(?:\.\d+)?\s*cm|\d{2,3}號|尺寸|規格|款式|顏色|color|size)', s, re.I):
        return True
    # Generic short label fallback.
    return len(s) <= 30


def dedupe_keep_order(items: List[str]) -> List[str]:
    out: List[str] = []
    seen = set()
    for x in items:
        c = clean(x)
        if not c:
            continue
        k = c.lower()
        if k in seen:
            continue
        seen.add(k)
        out.append(c)
    return out


def extract_from_variation_section(src: str) -> List[str]:
    section = ''
    m = re.search(r'<h2[^>]*>\s*Variation\s*</h2>(.*?)</section>', src, flags=re.I | re.S)
    if m:
        section = m.group(1)
    else:
        m2 = re.search(r'(規格|款式|顏色|樣式)(.*?)(</section>|</div></div></section>)', src, flags=re.I | re.S)
        if m2:
            section = m2.group(0)

    if not section:
        return []

    out: List[str] = []
    for m in re.finditer(r'aria-label="([^"]+)"', section, flags=re.I):
        t = clean(m.group(1))
        if is_option_like(t):
            out.append(t)
    for m in re.finditer(r'<span[^>]*>([^<]+)</span>', section, flags=re.I):
        t = clean(m.group(1))
        if is_option_like(t):
            out.append(t)
    return dedupe_keep_order(out)


def extract_from_variation_snapshots(artifact_dir: str) -> List[str]:
    out: List[str] = []
    for i in range(10):
        p_gz = os.path.join(artifact_dir, f's0-variation-{i}.html.gz')
        p_html = os.path.join(artifact_dir, f's0-variation-{i}.html')
        p = p_gz if os.path.exists(p_gz) else p_html if os.path.exists(p_html) else ''
        if not p:
            continue
        try:
            src = read_text(p)
        except Exception:
            continue
        m = re.search(r'aria-label="([^"]+)"[^>]*aria-disabled="(?:true|false)"', src, flags=re.I)
        if m:
            t = clean(m.group(1))
            if is_option_like(t):
                out.append(t)
    return dedupe_keep_order(out)


def build_variations(primary_html: str, artifact_dir: str) -> List[Dict[str, int | str]]:
    src = read_text(primary_html)
    titles = extract_from_variation_section(src)
    if not titles:
        # Global fallback on button aria-labels.
        all_labels = [clean(x) for x in re.findall(r'<button[^>]+aria-label="([^"]+)"', src, flags=re.I)]
        titles = [t for t in dedupe_keep_order(all_labels) if is_option_like(t)]

    # Merge with snapshot-derived titles when available.
    snap_titles = extract_from_variation_snapshots(artifact_dir)
    merged = dedupe_keep_order(titles + snap_titles)[:MAX_VARIATIONS]

    variations: List[Dict[str, int | str]] = []
    for i, title in enumerate(merged):
        variations.append({'title': title, 'position': i})
    return variations


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description='Extract Shopee variations from HTML artifact')
    p.add_argument('--artifact-dir', default='', help='Artifact directory')
    p.add_argument('--html-path', default='', help='Explicit primary html(.gz) path')
    p.add_argument('--output', default='', help='Output JSON path (default: <artifact_dir>/variations_extract.json)')
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
        output = os.path.join(artifact_dir, 'variations_extract.json')

    if not artifact_dir and not html_path:
        res = {'status': 'error', 'variations': [], 'error': 'artifact_dir or html_path is required'}
        if output:
            write_json(output, res)
        print(json.dumps(res, ensure_ascii=False))
        return 1

    try:
        primary_html = pick_primary_html(artifact_dir, html_path)
        variations = build_variations(primary_html, artifact_dir)
        res = {'status': 'ok', 'variations': variations, 'error': ''}
        if output:
            write_json(output, res)
        print(json.dumps(res, ensure_ascii=False))
        return 0
    except Exception as e:
        res = {'status': 'error', 'variations': [], 'error': str(e)}
        if output:
            write_json(output, res)
        print(json.dumps(res, ensure_ascii=False))
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
