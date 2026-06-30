#!/usr/bin/env python3
"""Cursor hook: enforce DESIGN.md cross-check after UI file edits."""

from __future__ import annotations

import json
import os
import re
import sys
from pathlib import Path

ROOT = Path(os.environ.get("DESIGN_CHECK_ROOT", Path(__file__).resolve().parents[2]))
DESIGN_MD = ROOT / "DESIGN.md"

UI_PATH_RE = re.compile(
    r"(^|/)(assets/css/|assets/js/(main|widgets)\.js$|layouts/|static/.+\.(css|html)$)",
    re.IGNORECASE,
)

# Patterns that commonly violate DESIGN.md (see Do's and Don'ts).
LINT_RULES: list[tuple[str, re.Pattern[str], str]] = [
    (
        "no-card-shadow",
        re.compile(r"\.(?:card|site-nav-panel|site-header|section(?:--snow)?)\b[^{]*\{[^}]*box-shadow:\s*(?!none\b)", re.S | re.I),
        "DESIGN.md: no box-shadow on cards/containers — use surface color for elevation only.",
    ),
    (
        "no-transition-all",
        re.compile(r"transition\s*:\s*all\b", re.I),
        "DESIGN.md / motion: avoid `transition: all` — list explicit properties.",
    ),
    (
        "cobalt-not-button-bg",
        re.compile(r"background(?:-color)?\s*:\s*(?:#0066cc|var\(--color-cobalt-link\))", re.I),
        "DESIGN.md: #0066cc (Cobalt Link) is for inline links only, not button fills.",
    ),
]

CHECKLIST = """\
Mandatory DESIGN.md cross-check for this UI change:
1. Surfaces: canvas #f5f5f7, cards #ffffff, recessed #f5f5f7 — never reverse canvas/card.
2. Elevation: no box-shadow on cards or containers; depth via background color only.
3. CTA color: #0071e3 only on .btn-buy; inline links use #0066cc, not button backgrounds.
4. Radii: 28px on feature cards (--radius-cards); pill buttons 999px.
5. Typography: negative letter-spacing scaled to size; weight 700 for hero/display; no weight 300 below 40px headlines.
6. Layout: left-align body copy longer than 2 lines; hero may center short lockups only.
7. Product finish gradients/swatches: never use as UI chrome or generic accents.
8. Motion: 0.344s base / 0.1s quick; no `transition: all`.

Read DESIGN.md at the repo root and confirm this edit adheres before marking the task done.\
"""


def is_ui_path(path: str) -> bool:
    if not path:
        return False
    try:
        rel = Path(path).resolve().relative_to(ROOT.resolve())
    except ValueError:
        rel = Path(path)
    rel_str = rel.as_posix()
    return bool(UI_PATH_RE.search(rel_str))


def read_stdin_json() -> dict:
    raw = sys.stdin.read()
    if not raw.strip():
        return {}
    return json.loads(raw)


def file_path_from_payload(payload: dict) -> str | None:
    hook = payload.get("hook_event_name", "")

    if hook == "afterFileEdit" or hook == "afterTabFileEdit":
        return payload.get("file_path")

    if hook == "postToolUse":
        tool_input = payload.get("tool_input")
        if isinstance(tool_input, str):
            try:
                tool_input = json.loads(tool_input)
            except json.JSONDecodeError:
                tool_input = {}
        if not isinstance(tool_input, dict):
            return None
        for key in ("path", "file_path", "target_notebook", "notebook_path"):
            if tool_input.get(key):
                return str(tool_input[key])
        return None

    return payload.get("file_path")


def lint_file(path: Path) -> list[str]:
    if not path.is_file():
        return []

    try:
        content = path.read_text(encoding="utf-8")
    except OSError:
        return []

    findings: list[str] = []
    for _rule_id, pattern, message in LINT_RULES:
        if pattern.search(content):
            findings.append(message)
    return findings


def lint_edits(payload: dict) -> list[str]:
    findings: list[str] = []
    combined = []
    for edit in payload.get("edits") or []:
        combined.append(edit.get("new_string") or "")
    blob = "\n".join(combined)
    if not blob.strip():
        return findings

    for _rule_id, pattern, message in LINT_RULES:
        if pattern.search(blob):
            findings.append(message)
    return findings


def build_context(file_path: str, findings: list[str]) -> str:
    rel = file_path
    try:
        rel = Path(file_path).resolve().relative_to(ROOT.resolve()).as_posix()
    except ValueError:
        pass

    parts = [
        f"UI file changed: `{rel}`.",
        CHECKLIST,
    ]
    if findings:
        unique = list(dict.fromkeys(findings))
        parts.append("Automated DESIGN.md lint flags:")
        parts.extend(f"- {item}" for item in unique)
        parts.append("Fix these violations or explain why the deviation is intentional.")
    return "\n\n".join(parts)


def main() -> None:
    payload = read_stdin_json()
    hook = payload.get("hook_event_name", "")

    file_path = file_path_from_payload(payload)
    if not file_path or not is_ui_path(file_path):
        print("{}")
        return

    findings = lint_edits(payload)
    if not findings and file_path:
        findings = lint_file(Path(file_path))

    context = build_context(file_path, findings)
    output = {"additional_context": context}

    # postToolUse and sessionStart support additional_context; afterFileEdit may
    # consume guardrail output in supported Cursor versions — harmless if ignored.
    print(json.dumps(output))


if __name__ == "__main__":
    main()
