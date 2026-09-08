#!/usr/bin/env bash
# Refresh the manual from real UI controls. No login, user profile, or network is required.
set -euo pipefail
root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
capture_dir="$(mktemp -d)"
trap 'rm -rf "$capture_dir"' EXIT
cd "$root_dir/source"
if [[ -z "${DISPLAY:-}" ]]; then
  xvfb-run -a env GOTHOOM_CAPTURE_HELP="$capture_dir" go test . -run '^TestCaptureHelp$' -count=1
else
  GOTHOOM_CAPTURE_HELP="$capture_dir" go test . -run '^TestCaptureHelp$' -count=1
fi
python3 "$root_dir/build-scripts/build_help.py" "$capture_dir" "$@"
python3 "$root_dir/build-scripts/check_help.py"
