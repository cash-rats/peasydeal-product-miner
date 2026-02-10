#!/usr/bin/env python3
from __future__ import annotations

import argparse
import gzip
import html
import json
import os
import re
from typing import Dict, List

MAX_OPTIONS = 10
MAX_IMAGES_PER_OPTION = 1


def read_text(path: str) -> str:
    if path.endswith('.gz'):
        with gzip.open(path, 'rt', encoding='utf-8', errors='replace') as f:
            return f.read()
    with open(path, 'r', encoding='utf-8', errors='replace') as f:
        return f.read()


def clean(s: str) -> str:
    s = html.unescape(s or '')
    s = re.sub(r'\s+', ' ', s).strip()
    return s


def load_title_fallbacks(artifact_dir: str) -> Dict[int, str]:
    out: Dict[int, str] = {}

    p = os.path.join(artifact_dir, 'variations_extract.json')
    if os.path.exists(p):
        try:
            data = json.load(open(p, 'r', encoding='utf-8'))
            for item in data.get('variations', []):
                pos = item.get('position')
                title = clean(str(item.get('title', '')))
                if isinstance(pos, int) and title:
                    out[pos] = title
        except Exception:
            pass

    if out:
        return out

    initial_candidates = [
        os.path.join(artifact_dir, 's0-initial.html.gz'),
        os.path.join(artifact_dir, 's0-initial.html'),
        os.path.join(artifact_dir, 's0-page.html.gz'),
        os.path.join(artifact_dir, 's0-page.html'),
    ]
    src = ''
    for p0 in initial_candidates:
        if os.path.exists(p0):
            try:
                src = read_text(p0)
                break
            except Exception:
                pass

    if not src:
        return out

    m = re.search(r'<h2[^>]*>\s*Variation\s*</h2>(.*?)</section>', src, flags=re.I | re.S)
    block = m.group(1) if m else src
    labels = [clean(x) for x in re.findall(r'<button[^>]+aria-label="([^"]+)"', block, flags=re.I)]
    seen = set()
    ordered: List[str] = []
    for t in labels:
        if not t:
            continue
        k = t.lower()
        if k in seen:
            continue
        seen.add(k)
        ordered.append(t)

    for i, t in enumerate(ordered[:MAX_OPTIONS]):
        out[i] = t
    return out


def extract_selected_title(src: str) -> str:
    m = re.search(r'<button[^>]*selection-box-selected[^>]*aria-label="([^"]+)"', src, flags=re.I)
    if m:
        return clean(m.group(1))
    m = re.search(r'<button[^>]*selection-box-selected[^>]*>.*?<span[^>]*>([^<]+)</span>', src, flags=re.I | re.S)
    if m:
        return clean(m.group(1))
    return ''


def _extract_attr_from_img_tag(tag: str, attr: str) -> str:
    m = re.search(rf'{attr}="([^"]+)"', tag, flags=re.I)
    if not m:
        return ''
    return clean(m.group(1)).replace('\\/', '/')


def extract_images(src: str) -> List[str]:
    urls: List[str] = []
    seen = set()

    def add(u: str) -> None:
        if not u:
            return
        if not u.lower().startswith(('http://', 'https://')):
            return
        if 'susercontent.com/file/' not in u:
            return
        if u in seen:
            return
        seen.add(u)
        urls.append(u)

    # Primary: hero product image(s) in main product image box.
    hero_tags = re.findall(r'<img[^>]*alt="Product image[^"]*"[^>]*>', src, flags=re.I)
    if hero_tags:
        candidates = []
        for tag in hero_tags:
            for attr in ('currentSrc', 'src', 'data-src', 'data-lazy', 'data-original'):
                u = _extract_attr_from_img_tag(tag, attr)
                if not u or 'susercontent.com/file/' not in u:
                    continue
                score = 0
                if '@resize_w450' in u:
                    score += 10
                if '@resize_w900' in u:
                    score += 9
                if '_tn' in u:
                    score -= 5
                candidates.append((score, u))
        for _, u in sorted(candidates, key=lambda x: x[0], reverse=True):
            add(u)
            if len(urls) >= MAX_IMAGES_PER_OPTION:
                return urls[:MAX_IMAGES_PER_OPTION]

    # Secondary: pick highest-confidence susercontent product-like URLs only.
    img_tags = re.findall(r'<img[^>]+>', src, flags=re.I)
    scored = []
    for tag in img_tags:
        src_url = _extract_attr_from_img_tag(tag, 'src')
        if not src_url or 'susercontent.com/file/' not in src_url:
            continue
        alt = _extract_attr_from_img_tag(tag, 'alt').lower()
        score = 0
        if 'product image' in alt:
            score += 10
        if '@resize_w450' in src_url or '@resize_w900' in src_url:
            score += 5
        if '_tn' in src_url:
            score -= 3
        scored.append((score, src_url))
    for _, u in sorted(scored, key=lambda x: x[0], reverse=True):
        add(u)
        if len(urls) >= MAX_IMAGES_PER_OPTION:
            break

    return urls[:MAX_IMAGES_PER_OPTION]


def build_map(artifact_dir: str) -> List[Dict[str, object]]:
    title_fallbacks = load_title_fallbacks(artifact_dir)
    out: List[Dict[str, object]] = []

    for i in range(MAX_OPTIONS):
        p_gz = os.path.join(artifact_dir, f's0-variation-{i}.html.gz')
        p_html = os.path.join(artifact_dir, f's0-variation-{i}.html')
        p = p_gz if os.path.exists(p_gz) else p_html if os.path.exists(p_html) else ''
        if not p:
            continue
        try:
            src = read_text(p)
        except Exception:
            continue

        title = extract_selected_title(src)
        if not title:
            title = title_fallbacks.get(i, '')
        if not title:
            # per-item failure: skip
            continue

        images = extract_images(src)
        out.append({'title': title, 'position': i, 'images': images})

    return out


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description='Extract Shopee variation->images map from HTML artifacts')
    p.add_argument('--artifact-dir', required=True, help='Artifact directory')
    p.add_argument('--output', default='', help='Output JSON path (default: <artifact_dir>/variation_image_map_extract.json)')
    return p.parse_args()


def write_json(path: str, obj: dict) -> None:
    os.makedirs(os.path.dirname(path) or '.', exist_ok=True)
    with open(path, 'w', encoding='utf-8') as f:
        json.dump(obj, f, ensure_ascii=False)


def main() -> int:
    args = parse_args()
    artifact_dir = args.artifact_dir.strip()
    output = args.output.strip() or os.path.join(artifact_dir, 'variation_image_map_extract.json')

    try:
        variations = build_map(artifact_dir)
        res = {'status': 'ok', 'variations': variations, 'error': ''}
        write_json(output, res)
        print(json.dumps(res, ensure_ascii=False))
        return 0
    except Exception as e:
        res = {'status': 'error', 'variations': [], 'error': str(e)}
        write_json(output, res)
        print(json.dumps(res, ensure_ascii=False))
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
