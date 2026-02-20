#!/usr/bin/env python3
from __future__ import annotations

import argparse
import gzip
import html
import json
import os
import re
from typing import Any, Dict, List, Tuple

MAX_VARIATIONS = 20
MAX_VARIATION_SNAPSHOTS = 20


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


def normalize_price(raw: Any) -> str:
    t = clean(str(raw)).replace(',', '')
    m = re.search(r'(\d+(?:\.\d{1,2})?)', t)
    return m.group(1) if m else ''


def build_sku_price_map(res: Dict[str, Any]) -> Dict[str, str]:
    out: Dict[str, str] = {}
    sku2info = ((res.get('skuCore') or {}).get('sku2info') or {})
    if not isinstance(sku2info, dict):
        return out

    for sku_id, info in sku2info.items():
        if not isinstance(info, dict):
            continue

        price = normalize_price(((info.get('subPrice') or {}).get('priceText') or ''))
        if not price:
            price = normalize_price(((info.get('price') or {}).get('priceText') or ''))
        if not price:
            money = normalize_price(((info.get('subPrice') or {}).get('priceMoney') or ''))
            if money and money.isdigit() and len(money) > 2:
                price = normalize_price(f'{money[:-2]}.{money[-2:]}')
        if not price:
            money = normalize_price(((info.get('price') or {}).get('priceMoney') or ''))
            if money and money.isdigit() and len(money) > 2:
                price = normalize_price(f'{money[:-2]}.{money[-2:]}')

        out[str(sku_id)] = price

    return out


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


def build_group_value_maps(group: Dict[str, Any]) -> Tuple[List[str], Dict[str, str]]:
    ordered_titles: List[str] = []
    vid_to_title: Dict[str, str] = {}
    seen = set()

    values = group.get('values') or []
    if not isinstance(values, list):
        return ordered_titles, vid_to_title

    for v in values:
        if not isinstance(v, dict):
            continue
        title = clean(str(v.get('name', '')))
        vid = clean(str(v.get('vid', '')))
        if not title or not vid:
            continue
        key = title.lower()
        if key in seen:
            continue
        seen.add(key)
        ordered_titles.append(title)
        vid_to_title[vid] = title
        if len(ordered_titles) >= MAX_VARIATIONS:
            break

    return ordered_titles, vid_to_title


def build_title_price_maps(
    res: Dict[str, Any],
    group_pid: str,
    vid_to_title: Dict[str, str],
    sku_price_map: Dict[str, str],
) -> Tuple[Dict[str, str], Dict[int, str]]:
    title_price: Dict[str, str] = {}
    pos_price: Dict[int, str] = {}

    skus = ((res.get('skuBase') or {}).get('skus') or [])
    if not isinstance(skus, list):
        return title_price, pos_price

    title_order = list(vid_to_title.values())
    title_pos = {t: i for i, t in enumerate(title_order)}
    pat = re.compile(rf'(?:^|;){re.escape(group_pid)}:([^;]+)')

    for sku in skus:
        if not isinstance(sku, dict):
            continue
        prop_path = clean(str(sku.get('propPath', '')))
        if not prop_path:
            continue
        m = pat.search(prop_path)
        if not m:
            continue

        vid = m.group(1)
        title = vid_to_title.get(vid, '')
        if not title:
            continue

        sku_id = clean(str(sku.get('skuId', '')))
        price = sku_price_map.get(sku_id, '')
        if title not in title_price and price:
            title_price[title] = price

        pos = title_pos.get(title)
        if isinstance(pos, int) and pos not in pos_price and price:
            pos_price[pos] = price

    return title_price, pos_price


def build_sku_title_map(res: Dict[str, Any], group_pid: str, vid_to_title: Dict[str, str]) -> Dict[str, str]:
    out: Dict[str, str] = {}
    skus = ((res.get('skuBase') or {}).get('skus') or [])
    if not isinstance(skus, list):
        return out

    pat = re.compile(rf'(?:^|;){re.escape(group_pid)}:([^;]+)')
    for sku in skus:
        if not isinstance(sku, dict):
            continue
        m = pat.search(clean(str(sku.get('propPath', ''))))
        if not m:
            continue
        vid = m.group(1)
        title = vid_to_title.get(vid, '')
        sku_id = clean(str(sku.get('skuId', '')))
        if sku_id and title and sku_id not in out:
            out[sku_id] = title
    return out


def extract_sku_id_from_capture(artifact_dir: str, position: int) -> str:
    p = os.path.join(artifact_dir, f'_variation{position}_capture.json')
    obj = read_json(p)
    if not obj:
        return ''
    url = clean(str(obj.get('target_url', '')))
    if not url:
        return ''
    m = re.search(r'(?:[?&]|%26)skuId(?:=|%3D)([0-9]+)', url, re.I)
    return m.group(1) if m else ''


def parse_snapshot_prices(
    artifact_dir: str,
    sku_price_map: Dict[str, str],
    sku_title_map: Dict[str, str],
) -> Tuple[Dict[str, str], Dict[int, str]]:
    title_out: Dict[str, str] = {}
    pos_out: Dict[int, str] = {}

    for i in range(MAX_VARIATION_SNAPSHOTS):
        p_gz = os.path.join(artifact_dir, f's0-variation-{i}.html.gz')
        p_html = os.path.join(artifact_dir, f's0-variation-{i}.html')
        if not (os.path.exists(p_gz) or os.path.exists(p_html)):
            continue

        sku_id = extract_sku_id_from_capture(artifact_dir, i)
        if not sku_id:
            continue

        price = sku_price_map.get(sku_id, '')
        if not price:
            continue

        title = sku_title_map.get(sku_id, '')
        if title:
            title_out[title] = price
        else:
            pos_out[i] = price

    return title_out, pos_out


def build_variations(artifact_dir: str, primary_html: str) -> List[Dict[str, Any]]:
    src = read_text(primary_html)
    res = extract_res_from_html(src)
    if not isinstance(res, dict) or not res:
        return []

    group = pick_variation_group(res)
    if not group:
        return []

    group_pid = clean(str(group.get('pid', '')))
    if not group_pid:
        return []

    titles, vid_to_title = build_group_value_maps(group)
    if not titles:
        return []

    sku_price_map = build_sku_price_map(res)
    title_price_map, pos_price_map = build_title_price_maps(res, group_pid, vid_to_title, sku_price_map)
    sku_title_map = build_sku_title_map(res, group_pid, vid_to_title)

    # Prefer snapshot-derived prices when present.
    snapshot_title_price, snapshot_pos_price = parse_snapshot_prices(artifact_dir, sku_price_map, sku_title_map)
    merged_title_price = dict(title_price_map)
    merged_title_price.update(snapshot_title_price)
    merged_pos_price = dict(pos_price_map)
    merged_pos_price.update(snapshot_pos_price)

    out: List[Dict[str, Any]] = []
    seen = set()

    for pos, title in enumerate(titles[:MAX_VARIATIONS]):
        key = title.lower()
        if key in seen:
            continue
        seen.add(key)

        price = merged_title_price.get(title, '') or merged_pos_price.get(pos, '')
        out.append({'title': title, 'position': pos, 'price': normalize_price(price)})

    return out


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description='Extract Taobao variations from HTML artifact')
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
        variations = build_variations(artifact_dir, primary_html)
        res = {'status': 'ok', 'variations': variations[:MAX_VARIATIONS], 'error': ''}
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
