#!/usr/bin/env python3
from __future__ import annotations

import argparse
import glob
import json
import os
from datetime import datetime, timezone
from typing import Any, Dict, List


def utc_now_iso() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def load_json(path: str) -> Dict[str, Any]:
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def rel(path: str, cwd: str) -> str:
    p = os.path.abspath(path)
    c = os.path.abspath(cwd)
    if p.startswith(c + os.sep):
        return os.path.relpath(p, c).replace("\\", "/")
    return path.replace("\\", "/")


def find_capture_entries(artifact_dir: str) -> List[Dict[str, Any]]:
    out: List[Dict[str, Any]] = []
    for p in sorted(glob.glob(os.path.join(artifact_dir, "_*capture.json"))):
        try:
            data = load_json(p)
        except Exception as e:
            out.append(
                {
                    "name": os.path.basename(p),
                    "capture_file": rel(p, os.getcwd()),
                    "status": "error",
                    "error": str(e),
                }
            )
            continue

        out.append(
            {
                "name": os.path.basename(p),
                "capture_file": rel(p, os.getcwd()),
                "status": str(data.get("status", "")),
                "captured_at": str(data.get("captured_at", "")),
                "target_id": str(data.get("target_id", "")),
                "target_url": str(data.get("target_url", "")),
                "output": str(data.get("output", "")),
                "bytes": int(data.get("bytes", 0)) if str(data.get("bytes", "")).isdigit() else data.get("bytes", 0),
                "sha256": str(data.get("sha256", "")),
                "truncated": bool(data.get("truncated", False)),
                "original_bytes": data.get("original_bytes", 0),
                "error": str(data.get("error", "")),
            }
        )
    return out


def main() -> int:
    ap = argparse.ArgumentParser(description="Build taobao S0 manifest and pointer JSON outputs")
    ap.add_argument("--artifact-dir", required=True, help="out/artifacts/<run_id>")
    ap.add_argument("--url", required=True, help="Target URL")
    ap.add_argument("--run-id", default="", help="Run ID (defaults to basename of artifact dir)")
    ap.add_argument("--status", required=True, choices=["ok", "needs_manual", "error"], help="Final status")
    ap.add_argument("--notes", default="", help="Notes")
    ap.add_argument("--error", default="", help="Error message")
    ap.add_argument("--created-page-idx", type=int, default=-1, help="Created tab/page index")
    ap.add_argument("--created-target-id", default="", help="Created tab target id")
    ap.add_argument("--created-url", default="", help="Created tab URL")
    ap.add_argument("--close-attempted", action="store_true", help="Whether close was attempted")
    ap.add_argument("--close-succeeded", action="store_true", help="Whether close succeeded")
    ap.add_argument("--close-error", default="", help="Close error message")
    args = ap.parse_args()

    artifact_dir = os.path.abspath(args.artifact_dir)
    os.makedirs(artifact_dir, exist_ok=True)

    run_id = args.run_id.strip() or os.path.basename(artifact_dir.rstrip("/"))
    captured_at = utc_now_iso()

    initial_html = os.path.join(artifact_dir, "s0-initial.html.gz")
    overlay_html = os.path.join(artifact_dir, "s0-overlay.html.gz")

    pointer_obj: Dict[str, Any] = {
        "url": args.url,
        "status": args.status,
        "captured_at": captured_at,
        "run_id": run_id,
        "artifact_dir": rel(artifact_dir, os.getcwd()),
        "snapshot_files": {
            "snapshot": rel(os.path.join(artifact_dir, "s0-snapshot-pointer.json"), os.getcwd()),
            "manifest": rel(os.path.join(artifact_dir, "s0-manifest.json"), os.getcwd()),
            "initial_html": rel(initial_html, os.getcwd()) if os.path.exists(initial_html) else "",
            "overlay_html": rel(overlay_html, os.getcwd()) if os.path.exists(overlay_html) else "",
            "variation_html_dir": rel(artifact_dir, os.getcwd()),
        },
        "tab_tracking": {
            "created_tab": {
                "page_idx": args.created_page_idx,
                "target_id": args.created_target_id,
                "url": args.created_url,
            },
            "close_attempted": bool(args.close_attempted),
            "close_succeeded": bool(args.close_succeeded),
            "close_error": args.close_error,
        },
        "notes": args.notes,
        "error": args.error,
    }

    manifest_obj: Dict[str, Any] = {
        "url": args.url,
        "status": args.status,
        "run_id": run_id,
        "captured_at": captured_at,
        "artifact_dir": rel(artifact_dir, os.getcwd()),
        "snapshot_files": pointer_obj["snapshot_files"],
        "tab_tracking": pointer_obj["tab_tracking"],
        "captures": find_capture_entries(artifact_dir),
        "notes": args.notes,
        "error": args.error,
    }

    manifest_path = os.path.join(artifact_dir, "s0-manifest.json")
    pointer_path = os.path.join(artifact_dir, "s0-snapshot-pointer.json")
    with open(manifest_path, "w", encoding="utf-8") as f:
        json.dump(manifest_obj, f, ensure_ascii=False)
    with open(pointer_path, "w", encoding="utf-8") as f:
        json.dump(pointer_obj, f, ensure_ascii=False)

    print(json.dumps(pointer_obj, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
