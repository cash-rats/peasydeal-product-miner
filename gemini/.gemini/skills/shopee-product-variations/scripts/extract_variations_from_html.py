#!/usr/bin/env python3
from __future__ import annotations

import argparse
import gzip
import html
import json
import os
import re
from typing import Dict, List

MAX_VARIATIONS = 20
MAX_VARIATION_SNAPSHOTS = 20
BLOCKED_LABELS = {
    '跳到主要內容',
    '加入購物車',
    '直接購買',
    '購物車',
    '登入',
    '註冊',
}


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
    if not s or s in BLOCKED_LABELS:
        return False
    if len(s) > 80:
        return False
    # Avoid obvious navigation/meta labels.
    if re.search(r'(主要內容|賣家中心|通知|追蹤我們|蝦皮購物)', s, re.I):
        return False
    # Common variation-like patterns.
    if re.search(r'(\d|\*|x|cm|mm|kg|ml|絲|號|入|組|包|款|色|\(|\)|-|/|尺寸|規格|款式|顏色|color|size)', s, re.I):
        return True
    # Also allow concise plain labels (e.g. 黑/白/S/M/L) when not blocked.
    return 1 <= len(s) <= 24


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


def extract_primary_titles(src: str) -> List[str]:
    # Strictly read option buttons used by Shopee variation picker.
    patterns = [
        r'<button[^>]*class="[^"]*selection-box[^"]*"[^>]*aria-label="([^"]+)"',
        r'<button[^>]*aria-label="([^"]+)"[^>]*class="[^"]*selection-box[^"]*"',
    ]
    out: List[str] = []
    for pat in patterns:
        for m in re.finditer(pat, src, flags=re.I):
            t = clean(m.group(1))
            if is_option_like(t):
                out.append(t)
        if out:
            break
    return dedupe_keep_order(out)


def parse_variation_price(src: str) -> str:
    patterns = [
        r'<div[^>]*class="[^"]*IZPeQz[^"]*B67UQ0[^"]*"[^>]*>\s*([^<]+)\s*</div>',
        r'aria-live="polite".{0,1200}?\$([0-9][0-9,]*)',
    ]
    for idx, pat in enumerate(patterns):
        m = re.search(pat, src, flags=re.I | re.S)
        if not m:
            continue
        raw = clean(m.group(1))
        if idx == 1 and raw and not raw.startswith('$'):
            raw = '$' + raw
        if re.search(r'\$?\d', raw):
            return raw
    return ''


def parse_selected_title_from_snapshot(src: str) -> str:
    pats = [
        r'<button[^>]*class="[^"]*selection-box-selected[^"]*"[^>]*aria-label="([^"]+)"',
        r'<button[^>]*aria-label="([^"]+)"[^>]*class="[^"]*selection-box-selected[^"]*"',
        r'Product image[^"]*?\s([^\s"<]{1,80})"',
    ]
    for pat in pats:
        m = re.search(pat, src, flags=re.I)
        if not m:
            continue
        t = clean(m.group(1))
        if is_option_like(t):
            return t
    return ''


def extract_from_variation_snapshots(artifact_dir: str, primary_titles: List[str]) -> List[Dict[str, int | str]]:
    out: List[Dict[str, int | str]] = []
    for i in range(MAX_VARIATION_SNAPSHOTS):
        p_gz = os.path.join(artifact_dir, f's0-variation-{i}.html.gz')
        p_html = os.path.join(artifact_dir, f's0-variation-{i}.html')
        p = p_gz if os.path.exists(p_gz) else p_html if os.path.exists(p_html) else ''
        if not p:
            continue
        try:
            src = read_text(p)
        except Exception:
            continue
        title = parse_selected_title_from_snapshot(src)
        if not title and i < len(primary_titles):
            title = primary_titles[i]
        if not is_option_like(title):
            continue
        out.append({
            'title': title,
            'position': i,
            'price': parse_variation_price(src),
        })
    return out


def build_variations(primary_html: str, artifact_dir: str) -> List[Dict[str, int | str]]:
    src = read_text(primary_html)
    titles = extract_primary_titles(src)
    snap_rows = extract_from_variation_snapshots(artifact_dir, titles)

    price_by_title: Dict[str, str] = {}
    price_by_pos: Dict[int, str] = {}
    for row in snap_rows:
        t = str(row.get('title', ''))
        p = str(row.get('price', ''))
        pos = int(row.get('position', 0))
        if t and p and t not in price_by_title:
            price_by_title[t] = p
        if p and pos not in price_by_pos:
            price_by_pos[pos] = p

    if not titles:
        titles = [str(r.get('title', '')) for r in snap_rows if is_option_like(str(r.get('title', '')))]
        titles = dedupe_keep_order(titles)

    variations: List[Dict[str, int | str]] = []
    for i, title in enumerate(titles[:MAX_VARIATIONS]):
        price = price_by_title.get(title, price_by_pos.get(i, ''))
        variations.append({'title': title, 'position': i, 'price': price})
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
