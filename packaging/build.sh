#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

mode="onefolder"
if [[ ${1:-} == "--onefile" ]]; then
  mode="onefile"
fi

if [[ ! -x .venv/bin/python ]]; then
  echo "Missing .venv. Create it first: python3 -m venv .venv && .venv/bin/python -m pip install -r requirements.txt && .venv/bin/python -m pip install -e ." >&2
  exit 1
fi

.venv/bin/python -m pip install -U pyinstaller

# Clean previous builds
rm -rf build dist

if [[ "$mode" == "onefile" ]]; then
  .venv/bin/python -m PyInstaller packaging/pyinstaller-onefile.spec
  echo "Built: dist/musipal"
else
  .venv/bin/python -m PyInstaller packaging/pyinstaller.spec
  echo "Built: dist/musipal/musipal"
fi
