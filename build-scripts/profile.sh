#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

go tool pprof -svg "${ROOT_DIR}/source/default.pgo" > "${ROOT_DIR}/cpu.svg"
