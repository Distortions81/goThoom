#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

DICT_URL="https://raw.githubusercontent.com/dwyl/english-words/master/words_alpha.txt"
TARGET="$ROOT_DIR/spellcheck_words.txt"

echo "Downloading US English dictionary..."
curl -L "$DICT_URL" -o "$TARGET"
echo "Dictionary saved to $TARGET"
