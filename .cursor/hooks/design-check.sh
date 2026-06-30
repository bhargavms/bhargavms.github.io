#!/usr/bin/env bash
# Cross-check UI edits against DESIGN.md design language.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export DESIGN_CHECK_ROOT="$ROOT"

python3 "$(dirname "$0")/design-check.py"
