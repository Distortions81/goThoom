#!/usr/bin/env bash
set -euo pipefail

# Build the gothoom development environment Docker image including
# toolchains for cross-compiling to Linux, Windows and macOS.
cd "$(dirname "$0")/.."

docker build -t gothoom-build-env .
