#!/bin/sh

set -eu

if ! command -v rsvg-convert >/dev/null 2>&1; then
	echo "render.sh: rsvg-convert is required (Ubuntu package: librsvg2-bin)" >&2
	exit 1
fi

icon_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
for source in "$icon_dir"/*.svg; do
	output=${source%.svg}.png
	rsvg-convert \
		--width 64 \
		--height 64 \
		--keep-aspect-ratio \
		--output "$output" \
		"$source"
done
