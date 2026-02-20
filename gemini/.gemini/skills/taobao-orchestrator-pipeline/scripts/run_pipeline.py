#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Tuple


STAGES = [
    "snapshot_capture",
    "core_extract",
    "images_extract",
    "variations_extract",
    "variation_image_map_extract",
]


def utc_now_iso() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def ensure_dir(path: Path) -> None:
    path.mkdir(parents=True, exist_ok=True)


def rel_path(path: Path, cwd: Path) -> str:
    p = path.resolve()
    c = cwd.resolve()
    try:
        return str(p.relative_to(c)).replace("\\", "/")
    except ValueError:
        return str(path).replace("\\", "/")


def write_json(path: Path, obj: Dict[str, Any]) -> None:
    ensure_dir(path.parent)
    with path.open("w", encoding="utf-8") as f:
        json.dump(obj, f, ensure_ascii=False)


def read_json(path: Path) -> Dict[str, Any]:
    with path.open("r", encoding="utf-8") as f:
        obj = json.load(f)
    if not isinstance(obj, dict):
        raise ValueError(f"json root is not an object: {path}")
    return obj


def normalize_url(s: Any) -> str:
    v = str(s or "").strip()
    if v.startswith("//"):
        return "https:" + v
    return v


def unique_http_urls(items: List[Any], limit: int) -> List[str]:
    out: List[str] = []
    seen = set()
    for raw in items:
        u = normalize_url(raw)
        if not u.lower().startswith(("http://", "https://")):
            continue
        if u in seen:
            continue
        seen.add(u)
        out.append(u)
        if len(out) >= limit:
            break
    return out


def stage_default(stage: str) -> Dict[str, Any]:
    return {
        "status": "pending",
        "started_at": "",
        "ended_at": "",
        "error": "",
        "stage": stage,
    }


def new_state(run_id: str, url: str, flags: Dict[str, bool]) -> Dict[str, Any]:
    return {
        "run_id": run_id,
        "url": url,
        "started_at": utc_now_iso(),
        "updated_at": utc_now_iso(),
        "current_stage": "snapshot_capture",
        "status": "running",
        "stages": {s: stage_default(s) for s in STAGES},
        "flags": flags,
    }


def update_state_file(path: Path, state: Dict[str, Any]) -> None:
    state["updated_at"] = utc_now_iso()
    write_json(path, state)


def stage_start(state: Dict[str, Any], state_path: Path, stage: str) -> None:
    state["current_stage"] = stage
    state["stages"][stage]["status"] = "running"
    state["stages"][stage]["started_at"] = utc_now_iso()
    state["stages"][stage]["ended_at"] = ""
    state["stages"][stage]["error"] = ""
    update_state_file(state_path, state)


def stage_end(state: Dict[str, Any], state_path: Path, stage: str, status: str, error: str = "") -> None:
    state["stages"][stage]["status"] = status
    state["stages"][stage]["ended_at"] = utc_now_iso()
    state["stages"][stage]["error"] = str(error or "")
    update_state_file(state_path, state)


def resolve_stage_files(script_path: Path) -> Dict[str, Path]:
    skills_dir = script_path.resolve().parents[2]
    return {
        "core_extract": skills_dir / "taobao-product-core" / "scripts" / "extract_core_from_html.py",
        "images_extract": skills_dir / "taobao-product-images" / "scripts" / "extract_images_from_html.py",
        "variations_extract": skills_dir / "taobao-product-variations" / "scripts" / "extract_variations_from_html.py",
        "variation_image_map_extract": skills_dir / "taobao-variation-image-map" / "scripts" / "extract_variation_image_map_from_html.py",
        "snapshot_skill": skills_dir / "taobao-page-snapshot" / "SKILL.md",
        "core_skill": skills_dir / "taobao-product-core" / "SKILL.md",
        "images_skill": skills_dir / "taobao-product-images" / "SKILL.md",
        "variations_skill": skills_dir / "taobao-product-variations" / "SKILL.md",
        "variation_map_skill": skills_dir / "taobao-variation-image-map" / "SKILL.md",
    }


def run_stage_script(stage: str, script: Path, artifact_dir: Path, output_file: Path) -> Tuple[Dict[str, Any], str]:
    cmd = [
        sys.executable,
        str(script),
        "--artifact-dir",
        str(artifact_dir),
        "--output",
        str(output_file),
    ]
    proc = subprocess.run(cmd, capture_output=True, text=True)
    stdout = (proc.stdout or "").strip()
    stderr = (proc.stderr or "").strip()

    err_details = ""
    if stderr:
        err_details = stderr

    parsed: Dict[str, Any] = {}
    if output_file.exists():
        try:
            parsed = read_json(output_file)
        except Exception as e:
            err_details = (err_details + " | " if err_details else "") + f"read output json failed: {e}"

    if not parsed and stdout:
        try:
            loaded = json.loads(stdout)
            if isinstance(loaded, dict):
                parsed = loaded
        except Exception as e:
            err_details = (err_details + " | " if err_details else "") + f"stdout json decode failed: {e}"

    if not parsed:
        parsed = {
            "status": "error",
            "error": f"{stage} returned no valid json",
        }
        if err_details:
            parsed["error"] = f"{parsed['error']}; {err_details}"
        write_json(output_file, parsed)

    if proc.returncode != 0 and parsed.get("status") != "error":
        parsed["status"] = "error"
        parsed["error"] = str(parsed.get("error") or f"non-zero exit: {proc.returncode}")
        write_json(output_file, parsed)

    return parsed, err_details


def ensure_stage_artifact(path: Path, stage: str) -> Dict[str, Any]:
    if path.exists():
        try:
            return read_json(path)
        except Exception:
            pass

    if stage == "core_extract":
        obj = {
            "status": "error",
            "title": "",
            "description": "",
            "currency": "",
            "price": "",
            "notes": "",
            "error": f"missing stage artifact: {path.name}",
        }
    elif stage == "images_extract":
        obj = {"status": "error", "images": [], "error": f"missing stage artifact: {path.name}"}
    else:
        obj = {"status": "error", "variations": [], "error": f"missing stage artifact: {path.name}"}

    write_json(path, obj)
    return obj


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description="Run Taobao artifact-first orchestrator pipeline")
    p.add_argument("--artifact-dir", required=True, help="Path to out/artifacts/<run_id>")
    p.add_argument("--url", default="", help="Optional URL override")
    p.add_argument("--run-id", default="", help="Optional run id override")
    p.add_argument("--description-max-chars", type=int, default=1500)
    p.add_argument("--images-max", type=int, default=20)
    p.add_argument("--variations-max", type=int, default=20)
    p.add_argument("--variation-image-map-max", type=int, default=20)
    p.add_argument("--images-enabled", dest="images_enabled", action="store_true")
    p.add_argument("--no-images", dest="images_enabled", action="store_false")
    p.set_defaults(images_enabled=True)
    p.add_argument("--variations-enabled", dest="variations_enabled", action="store_true")
    p.add_argument("--no-variations", dest="variations_enabled", action="store_false")
    p.set_defaults(variations_enabled=True)
    p.add_argument("--variation-image-map-enabled", dest="variation_image_map_enabled", action="store_true")
    p.add_argument("--no-variation-image-map", dest="variation_image_map_enabled", action="store_false")
    p.set_defaults(variation_image_map_enabled=True)
    return p.parse_args()


def finalize(
    state: Dict[str, Any],
    state_path: Path,
    final_obj: Dict[str, Any],
    final_path: Path,
    meta: Dict[str, Any],
    meta_path: Path,
) -> Dict[str, Any]:
    state["current_stage"] = "final_merge"
    state["status"] = (
        "completed"
        if final_obj.get("status") == "ok"
        else "needs_manual" if final_obj.get("status") == "needs_manual" else "error"
    )
    update_state_file(state_path, state)
    write_json(meta_path, meta)
    write_json(final_path, final_obj)
    return final_obj


def main() -> int:
    args = parse_args()

    artifact_dir = Path(args.artifact_dir).resolve()
    ensure_dir(artifact_dir)

    cwd = Path(os.getcwd())
    run_id = (args.run_id or "").strip() or artifact_dir.name

    files = resolve_stage_files(Path(__file__))
    required_skill_files = [
        files["snapshot_skill"],
        files["core_skill"],
        files["images_skill"],
        files["variations_skill"],
        files["variation_map_skill"],
    ]

    pipeline_state_path = artifact_dir / "_pipeline-state.json"
    pointer_path = artifact_dir / "s0-snapshot-pointer.json"
    manifest_path = artifact_dir / "s0-manifest.json"
    initial_html = artifact_dir / "s0-initial.html.gz"

    url = (args.url or "").strip()
    if pointer_path.exists() and not url:
        try:
            pointer = read_json(pointer_path)
            url = str(pointer.get("url") or "").strip()
        except Exception:
            url = ""

    flags = {
        "images_enabled": bool(args.images_enabled),
        "variations_enabled": bool(args.variations_enabled),
        "variation_image_map_enabled": bool(args.variation_image_map_enabled),
    }

    state = new_state(run_id=run_id, url=url, flags=flags)
    update_state_file(pipeline_state_path, state)

    meta: Dict[str, Any] = {
        "run_id": run_id,
        "orchestrator_skill": "taobao-orchestrator-pipeline",
        "stage_duration_ms": {
            "snapshot_capture": 0,
            "core_extract": 0,
            "images_extract": 0,
            "variations_extract": 0,
            "variation_image_map_extract": 0,
        },
        "stage_errors": [],
        "limits": {
            "description_max_chars": int(args.description_max_chars),
            "images_max": int(args.images_max),
            "variations_max": int(args.variations_max),
            "variation_image_map_max": int(args.variation_image_map_max),
        },
        "fallbacks": [],
    }

    final_path = artifact_dir / "final.json"
    meta_path = artifact_dir / "meta.json"

    missing = [str(p) for p in required_skill_files if not p.exists()]
    if missing:
        err = f"missing required stage skill files: {', '.join(missing)}"
        final_obj = {
            "url": url,
            "status": "error",
            "captured_at": utc_now_iso(),
            "notes": "",
            "error": err,
            "title": "",
            "description": "",
            "currency": "",
            "price": "",
            "images": [],
            "variations": [],
            "artifact_dir": rel_path(artifact_dir, cwd),
            "run_id": run_id,
        }
        meta["stage_errors"].append({"stage": "snapshot_capture", "error": err})
        out = finalize(state, pipeline_state_path, final_obj, final_path, meta, meta_path)
        print(json.dumps(out, ensure_ascii=False))
        return 1

    pointer: Dict[str, Any] = {}

    stage_start(state, pipeline_state_path, "snapshot_capture")
    t0 = time.time()
    try:
        pointer = read_json(pointer_path)
        _ = read_json(manifest_path)
        if not initial_html.exists():
            raise FileNotFoundError(f"missing required snapshot html: {initial_html.name}")

        pointer_status = str(pointer.get("status") or "").strip().lower()
        if pointer_status == "needs_manual":
            stage_end(state, pipeline_state_path, "snapshot_capture", "needs_manual", "")
            meta["stage_duration_ms"]["snapshot_capture"] = int((time.time() - t0) * 1000)
            notes = str(pointer.get("notes") or "blocked or verification wall detected from snapshot stage")
            final_obj = {
                "url": str(pointer.get("url") or url),
                "status": "needs_manual",
                "captured_at": str(pointer.get("captured_at") or utc_now_iso()),
                "notes": notes,
                "error": "",
                "title": "",
                "description": "",
                "currency": "",
                "price": "",
                "images": [],
                "variations": [],
                "artifact_dir": rel_path(artifact_dir, cwd),
                "run_id": run_id,
            }
            out = finalize(state, pipeline_state_path, final_obj, final_path, meta, meta_path)
            print(json.dumps(out, ensure_ascii=False))
            return 0

        if pointer_status == "error":
            err = str(pointer.get("error") or "snapshot stage returned error")
            stage_end(state, pipeline_state_path, "snapshot_capture", "error", err)
            meta["stage_duration_ms"]["snapshot_capture"] = int((time.time() - t0) * 1000)
            meta["stage_errors"].append({"stage": "snapshot_capture", "error": err})
            final_obj = {
                "url": str(pointer.get("url") or url),
                "status": "error",
                "captured_at": str(pointer.get("captured_at") or utc_now_iso()),
                "notes": "",
                "error": err,
                "title": "",
                "description": "",
                "currency": "",
                "price": "",
                "images": [],
                "variations": [],
                "artifact_dir": rel_path(artifact_dir, cwd),
                "run_id": run_id,
            }
            out = finalize(state, pipeline_state_path, final_obj, final_path, meta, meta_path)
            print(json.dumps(out, ensure_ascii=False))
            return 1

        stage_end(state, pipeline_state_path, "snapshot_capture", "completed", "")
    except Exception as e:
        err = str(e)
        stage_end(state, pipeline_state_path, "snapshot_capture", "error", err)
        meta["stage_errors"].append({"stage": "snapshot_capture", "error": err})
        meta["stage_duration_ms"]["snapshot_capture"] = int((time.time() - t0) * 1000)
        final_obj = {
            "url": url,
            "status": "error",
            "captured_at": utc_now_iso(),
            "notes": "",
            "error": err,
            "title": "",
            "description": "",
            "currency": "",
            "price": "",
            "images": [],
            "variations": [],
            "artifact_dir": rel_path(artifact_dir, cwd),
            "run_id": run_id,
        }
        out = finalize(state, pipeline_state_path, final_obj, final_path, meta, meta_path)
        print(json.dumps(out, ensure_ascii=False))
        return 1
    meta["stage_duration_ms"]["snapshot_capture"] = int((time.time() - t0) * 1000)

    # A: core (gating)
    core_path = artifact_dir / "core_extract.json"
    stage_start(state, pipeline_state_path, "core_extract")
    t1 = time.time()
    core_obj, core_extra_err = run_stage_script("core_extract", files["core_extract"], artifact_dir, core_path)
    if core_extra_err:
        meta["stage_errors"].append({"stage": "core_extract", "error": core_extra_err})

    core_obj = ensure_stage_artifact(core_path, "core_extract")
    core_status = str(core_obj.get("status") or "").strip().lower()

    if core_status == "needs_manual":
        stage_end(state, pipeline_state_path, "core_extract", "needs_manual", "")
        meta["stage_duration_ms"]["core_extract"] = int((time.time() - t1) * 1000)
        final_obj = {
            "url": str(pointer.get("url") or url),
            "status": "needs_manual",
            "captured_at": str(pointer.get("captured_at") or utc_now_iso()),
            "notes": str(core_obj.get("notes") or "blocked or verification wall detected from core stage"),
            "error": "",
            "title": "",
            "description": "",
            "currency": "",
            "price": "",
            "images": [],
            "variations": [],
            "artifact_dir": rel_path(artifact_dir, cwd),
            "run_id": run_id,
        }
        out = finalize(state, pipeline_state_path, final_obj, final_path, meta, meta_path)
        print(json.dumps(out, ensure_ascii=False))
        return 0

    if core_status != "ok":
        err = str(core_obj.get("error") or "core stage failed")
        stage_end(state, pipeline_state_path, "core_extract", "error", err)
        meta["stage_duration_ms"]["core_extract"] = int((time.time() - t1) * 1000)
        meta["stage_errors"].append({"stage": "core_extract", "error": err})
        final_obj = {
            "url": str(pointer.get("url") or url),
            "status": "error",
            "captured_at": str(pointer.get("captured_at") or utc_now_iso()),
            "notes": "",
            "error": err,
            "title": "",
            "description": "",
            "currency": "",
            "price": "",
            "images": [],
            "variations": [],
            "artifact_dir": rel_path(artifact_dir, cwd),
            "run_id": run_id,
        }
        out = finalize(state, pipeline_state_path, final_obj, final_path, meta, meta_path)
        print(json.dumps(out, ensure_ascii=False))
        return 1

    stage_end(state, pipeline_state_path, "core_extract", "completed", "")
    meta["stage_duration_ms"]["core_extract"] = int((time.time() - t1) * 1000)

    # B: images (degraded on error)
    images_path = artifact_dir / "images_extract.json"
    images_obj: Dict[str, Any] = {"status": "ok", "images": [], "error": ""}
    stage_start(state, pipeline_state_path, "images_extract")
    t2 = time.time()
    if flags["images_enabled"]:
        images_obj, images_extra_err = run_stage_script("images_extract", files["images_extract"], artifact_dir, images_path)
        if images_extra_err:
            meta["stage_errors"].append({"stage": "images_extract", "error": images_extra_err})
        images_obj = ensure_stage_artifact(images_path, "images_extract")
        if str(images_obj.get("status") or "").strip().lower() == "ok":
            stage_end(state, pipeline_state_path, "images_extract", "completed", "")
        else:
            err = str(images_obj.get("error") or "images stage failed")
            stage_end(state, pipeline_state_path, "images_extract", "error", err)
            meta["stage_errors"].append({"stage": "images_extract", "error": err})
            meta["fallbacks"].append("images_extract_degraded")
            images_obj = {"status": "error", "images": [], "error": err}
            write_json(images_path, images_obj)
    else:
        images_obj = {"status": "ok", "images": [], "error": ""}
        write_json(images_path, images_obj)
        stage_end(state, pipeline_state_path, "images_extract", "skipped", "")
    meta["stage_duration_ms"]["images_extract"] = int((time.time() - t2) * 1000)

    # C: variations (degraded on error)
    variations_path = artifact_dir / "variations_extract.json"
    variations_obj: Dict[str, Any] = {"status": "ok", "variations": [], "error": ""}
    stage_start(state, pipeline_state_path, "variations_extract")
    t3 = time.time()
    if flags["variations_enabled"]:
        variations_obj, variations_extra_err = run_stage_script(
            "variations_extract", files["variations_extract"], artifact_dir, variations_path
        )
        if variations_extra_err:
            meta["stage_errors"].append({"stage": "variations_extract", "error": variations_extra_err})
        variations_obj = ensure_stage_artifact(variations_path, "variations_extract")
        if str(variations_obj.get("status") or "").strip().lower() == "ok":
            stage_end(state, pipeline_state_path, "variations_extract", "completed", "")
        else:
            err = str(variations_obj.get("error") or "variations stage failed")
            stage_end(state, pipeline_state_path, "variations_extract", "error", err)
            meta["stage_errors"].append({"stage": "variations_extract", "error": err})
            meta["fallbacks"].append("variations_extract_degraded")
            variations_obj = {"status": "error", "variations": [], "error": err}
            write_json(variations_path, variations_obj)
    else:
        variations_obj = {"status": "ok", "variations": [], "error": ""}
        write_json(variations_path, variations_obj)
        stage_end(state, pipeline_state_path, "variations_extract", "skipped", "")
    meta["stage_duration_ms"]["variations_extract"] = int((time.time() - t3) * 1000)

    # D: variation image map (degraded on error)
    variation_map_path = artifact_dir / "variation_image_map_extract.json"
    variation_map_obj: Dict[str, Any] = {"status": "ok", "variations": [], "error": ""}
    stage_start(state, pipeline_state_path, "variation_image_map_extract")
    t4 = time.time()
    if flags["variation_image_map_enabled"]:
        variation_map_obj, variation_map_extra_err = run_stage_script(
            "variation_image_map_extract",
            files["variation_image_map_extract"],
            artifact_dir,
            variation_map_path,
        )
        if variation_map_extra_err:
            meta["stage_errors"].append({"stage": "variation_image_map_extract", "error": variation_map_extra_err})
        variation_map_obj = ensure_stage_artifact(variation_map_path, "variation_image_map_extract")
        if str(variation_map_obj.get("status") or "").strip().lower() == "ok":
            stage_end(state, pipeline_state_path, "variation_image_map_extract", "completed", "")
        else:
            err = str(variation_map_obj.get("error") or "variation image map stage failed")
            stage_end(state, pipeline_state_path, "variation_image_map_extract", "error", err)
            meta["stage_errors"].append({"stage": "variation_image_map_extract", "error": err})
            meta["fallbacks"].append("variation_image_map_extract_degraded")
            variation_map_obj = {"status": "error", "variations": [], "error": err}
            write_json(variation_map_path, variation_map_obj)
    else:
        variation_map_obj = {"status": "ok", "variations": [], "error": ""}
        write_json(variation_map_path, variation_map_obj)
        stage_end(state, pipeline_state_path, "variation_image_map_extract", "skipped", "")
    meta["stage_duration_ms"]["variation_image_map_extract"] = int((time.time() - t4) * 1000)

    core_title = str(core_obj.get("title") or "")
    core_description = str(core_obj.get("description") or "")[: max(0, int(args.description_max_chars))]
    core_currency = str(core_obj.get("currency") or "")
    core_price = core_obj.get("price")

    images = unique_http_urls(list(images_obj.get("images") or []), int(args.images_max))

    map_rows = list(variation_map_obj.get("variations") or [])[: int(args.variation_image_map_max)]
    map_key_to_images: Dict[str, List[str]] = {}
    for row in map_rows:
        if not isinstance(row, dict):
            continue
        title = str(row.get("title") or "").strip()
        pos = row.get("position")
        if not title or not isinstance(pos, int):
            continue
        imgs = unique_http_urls(list(row.get("images") or []), int(args.images_max))
        map_key_to_images[f"{title.lower()}#{pos}"] = imgs

    variations: List[Dict[str, Any]] = []
    rows = list(variations_obj.get("variations") or [])[: int(args.variations_max)]
    for row in rows:
        if not isinstance(row, dict):
            continue
        title = str(row.get("title") or "").strip()
        position = row.get("position")
        if not title or not isinstance(position, int):
            continue
        price = row.get("price")
        key = f"{title.lower()}#{position}"
        variations.append(
            {
                "title": title,
                "position": position,
                "price": "" if price is None else str(price),
                "images": map_key_to_images.get(key, []),
            }
        )

    notes_parts: List[str] = []
    pointer_notes = str(pointer.get("notes") or "").strip()
    if pointer_notes:
        notes_parts.append(pointer_notes)
    for f in meta.get("fallbacks", []):
        notes_parts.append(f"degraded: {f}")

    final_obj = {
        "url": str(pointer.get("url") or url),
        "status": "ok",
        "captured_at": str(pointer.get("captured_at") or utc_now_iso()),
        "notes": "; ".join(notes_parts),
        "error": "",
        "title": core_title,
        "description": core_description,
        "currency": core_currency,
        "price": "" if core_price is None else core_price,
        "images": images,
        "variations": variations,
        "artifact_dir": rel_path(artifact_dir, cwd),
        "run_id": run_id,
    }

    out = finalize(state, pipeline_state_path, final_obj, final_path, meta, meta_path)
    print(json.dumps(out, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
