#!/usr/bin/env python3
from __future__ import annotations

import argparse
import gzip
import html
import json
import os
import re
from typing import Any, Dict, List, Tuple

MAX_OPTIONS = 20


def read_text(path: str) -> str:
    if path.endswith('.gz'):
        with gzip.open(path, 'rt', encoding='utf-8', errors='replace') as f:
            return f.read()
    with open(path, 'r', encoding='utf-8', errors='replace') as f:
        return f.read()


def read_json(path: str) -> Dict[str, Any]:
    if not path or not os.path.exists(path):
        return {}
    with open(path, 'r', encoding='utf-8') as f:
        obj = json.load(f)
    return obj if isinstance(obj, dict) else {}


def clean(s: str) -> str:
    s = html.unescape(s or '')
    s = re.sub(r'\s+', ' ', s).strip()
    return s


def normalize_url(url: str) -> str:
    u = clean(str(url)).replace('\\/', '/')
    if u.startswith('//'):
        u = 'https:' + u
    return u


def is_http_url(url: str) -> bool:
    return bool(url) and url.lower().startswith(('http://', 'https://'))


def dedupe_keep_order(items: List[str]) -> List[str]:
    out: List[str] = []
    seen = set()
    for x in items:
        v = normalize_url(x)
        if not is_http_url(v):
            continue
        if v in seen:
            continue
        seen.add(v)
        out.append(v)
    return out


def pick_primary_html(artifact_dir: str) -> str:
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


def extract_js_object_after(src: str, marker: str) -> Dict[str, Any]:
    idx = src.find(marker)
    if idx < 0:
        return {}

    j = idx + len(marker)
    in_str = False
    esc = False
    depth = 0
    start = -1
    end = -1

    for pos in range(j, len(src)):
        ch = src[pos]
        if start < 0:
            if ch == '{':
                start = pos
                depth = 1
            continue

        if in_str:
            if esc:
                esc = False
            elif ch == '\\':
                esc = True
            elif ch == '"':
                in_str = False
            continue

        if ch == '"':
            in_str = True
        elif ch == '{':
            depth += 1
        elif ch == '}':
            depth -= 1
            if depth == 0:
                end = pos + 1
                break

    if start < 0 or end < 0:
        return {}

    try:
        obj = json.loads(src[start:end])
    except Exception:
        return {}

    return obj if isinstance(obj, dict) else {}


def extract_res_from_html(src: str) -> Dict[str, Any]:
    ctx = extract_js_object_after(src, 'var b = ')
    if not ctx:
        return {}
    return (
        ((ctx.get('loaderData') or {}).get('home') or {}).get('data') or {}
    ).get('res') or {}


def pick_variation_group(res: Dict[str, Any]) -> Dict[str, Any]:
    props = ((res.get('skuBase') or {}).get('props') or [])
    if not isinstance(props, list):
        return {}

    for p in props:
        if not isinstance(p, dict):
            continue
        name = clean(str(p.get('name', '')))
        values = p.get('values') or []
        if name and '颜色分类' in name and isinstance(values, list) and values:
            return p

    for p in props:
        if not isinstance(p, dict):
            continue
        values = p.get('values') or []
        if isinstance(values, list) and values:
            return p

    return {}


def load_variation_order(artifact_dir: str) -> List[Tuple[str, int]]:
    p = os.path.join(artifact_dir, 'variations_extract.json')
    obj = read_json(p)
    out: List[Tuple[str, int]] = []
    if not obj:
        return out

    for row in obj.get('variations', []):
        if not isinstance(row, dict):
            continue
        title = clean(str(row.get('title', '')))
        pos = row.get('position')
        if not title or not isinstance(pos, int):
            continue
        out.append((title, pos))
        if len(out) >= MAX_OPTIONS:
            break

    return out


def build_title_images(group: Dict[str, Any]) -> Dict[str, List[str]]:
    out: Dict[str, List[str]] = {}
    values = group.get('values') or []
    if not isinstance(values, list):
        return out

    for v in values:
        if not isinstance(v, dict):
            continue
        title = clean(str(v.get('name', '')))
        if not title:
            continue

        candidates = [
            v.get('image', ''),
            v.get('imageUrl', ''),
            ((v.get('corner') or {}).get('icon', '') if isinstance(v.get('corner'), dict) else ''),
        ]
        images = dedupe_keep_order([str(x) for x in candidates if x])
        if images:
            out[title] = images

    return out


def build_map(artifact_dir: str) -> List[Dict[str, object]]:
    src = read_text(pick_primary_html(artifact_dir))
    res = extract_res_from_html(src)
    if not isinstance(res, dict) or not res:
        return []

    group = pick_variation_group(res)
    if not group:
        return []

    title_images = build_title_images(group)
    if not title_images:
        return []

    ordered = load_variation_order(artifact_dir)
    out: List[Dict[str, object]] = []
    seen_titles = set()

    if ordered:
        for title, pos in ordered:
            key = title.lower()
            if key in seen_titles:
                continue
            imgs = title_images.get(title, [])
            if not imgs:
                # per-item failure, skip
                continue
            seen_titles.add(key)
            out.append({'title': title, 'position': pos, 'images': imgs})
            if len(out) >= MAX_OPTIONS:
                break
        return out

    # Fallback: derive order directly from group values when variations_extract.json is absent.
    values = group.get('values') or []
    if not isinstance(values, list):
        return []

    for idx, v in enumerate(values[:MAX_OPTIONS]):
        if not isinstance(v, dict):
            continue
        title = clean(str(v.get('name', '')))
        if not title:
            continue
        key = title.lower()
        if key in seen_titles:
            continue
        imgs = title_images.get(title, [])
        if not imgs:
            continue
        seen_titles.add(key)
        out.append({'title': title, 'position': idx, 'images': imgs})

    return out


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description='Extract Taobao variation->images map from HTML artifacts')
    p.add_argument('--artifact-dir', default='', help='Artifact directory')
    p.add_argument('--output', default='', help='Output JSON path (default: <artifact_dir>/variation_image_map_extract.json)')
    return p.parse_args()


def write_json(path: str, obj: dict) -> None:
    os.makedirs(os.path.dirname(path) or '.', exist_ok=True)
    with open(path, 'w', encoding='utf-8') as f:
        json.dump(obj, f, ensure_ascii=False)


def main() -> int:
    args = parse_args()
    artifact_dir = args.artifact_dir.strip()

    if not artifact_dir:
        res = {'status': 'error', 'variations': [], 'error': 'artifact_dir is required'}
        print(json.dumps(res, ensure_ascii=False))
        return 1

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
