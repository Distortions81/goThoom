#!/usr/bin/env bash
# Restore the pinned Unicode groups, ordering, and emoji names used by the picker.
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
data_dir="${script_dir}/../source/data/emoji"
data_sha=1d8a944f88d7952f7ef7c5167fef3c67995bcae24543949710231b03a201acda
verify() { printf '%s  %s\n' "$data_sha" "$1" | sha256sum --check --status; }
if [[ -f "${data_dir}/emoji-test.txt" ]] && verify "${data_dir}/emoji-test.txt"; then
  exit 0
fi
download_file="$(mktemp)"
trap 'rm -f "$download_file"' EXIT
curl -fsSL https://www.unicode.org/Public/17.0.0/emoji/emoji-test.txt -o "$download_file"
verify "$download_file"
mkdir -p "$data_dir"
mv "$download_file" "${data_dir}/emoji-test.txt"
