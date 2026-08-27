#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd -P)"
SCREENSHOT_DIR="${ROOT_DIR}/dev-screenshots"
README_FILE="${ROOT_DIR}/README.md"
WEBSITE_HTML="${ROOT_DIR}/website/index.html"
WEBSITE_IMAGE="${ROOT_DIR}/website/client.webp"

usage() {
  echo "Usage: $0 dev-screenshots/Screenshot.png" >&2
}

if [ "$#" -ne 1 ]; then
  usage
  exit 2
fi

if ! command -v cwebp >/dev/null 2>&1; then
  echo "cwebp is required. Install the webp package first." >&2
  exit 1
fi

input_path="$1"
if [[ "${input_path}" != /* ]]; then
  input_path="${PWD}/${input_path}"
fi

if [ ! -f "${input_path}" ]; then
  echo "Screenshot not found: $1" >&2
  exit 1
fi

input_dir="$(cd "$(dirname "${input_path}")" && pwd -P)"
input_name="$(basename "${input_path}")"
if [ "${input_dir}" != "${SCREENSHOT_DIR}" ]; then
  echo "Screenshot must be stored directly in dev-screenshots/." >&2
  exit 1
fi
input_path="${input_dir}/${input_name}"
readme_path="dev-screenshots/${input_name}"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

encode_log="${tmp_dir}/cwebp.log"
if ! cwebp -q 82 -resize 1600 0 "${input_path}" \
  -o "${tmp_dir}/client.webp" >"${encode_log}" 2>&1; then
  cat "${encode_log}" >&2
  exit 1
fi

image_width=""
image_height=""
dimensions="$(awk '/^Dimension:/ { print $2, $4; exit }' "${encode_log}")"
read -r image_width image_height <<<"${dimensions}"
if [[ ! "${image_width}" =~ ^[0-9]+$ || ! "${image_height}" =~ ^[0-9]+$ ]]; then
  echo "Could not determine the generated WebP dimensions." >&2
  exit 1
fi

awk -v replacement="${readme_path}" '
  {
    line = $0
    if (line ~ /<img src="dev-screenshots\/[^"]+"/ &&
        match(line, /dev-screenshots\/[^"]+/)) {
      line = substr(line, 1, RSTART - 1) replacement \
        substr(line, RSTART + RLENGTH)
      updated++
    }
    print line
  }
  END {
    if (updated != 1) {
      print "Expected exactly one README screenshot; found " updated > "/dev/stderr"
      exit 1
    }
  }
' "${README_FILE}" >"${tmp_dir}/README.md"

awk -v width="${image_width}" -v height="${image_height}" '
  {
    line = $0
    if (line ~ /<img src="client\.webp"/) {
      width_updated = sub(/width="[0-9]+"/, "width=\"" width "\"", line)
      height_updated = sub(/height="[0-9]+"/, "height=\"" height "\"", line)
      if (!width_updated || !height_updated) {
        invalid = 1
      }
      updated++
    }
    print line
  }
  END {
    if (updated != 1 || invalid) {
      print "Expected one website screenshot with width and height attributes" > "/dev/stderr"
      exit 1
    }
  }
' "${WEBSITE_HTML}" >"${tmp_dir}/index.html"

cp "${tmp_dir}/README.md" "${README_FILE}"
cp "${tmp_dir}/index.html" "${WEBSITE_HTML}"
cp "${tmp_dir}/client.webp" "${WEBSITE_IMAGE}"

echo "Updated README.md and website/client.webp from ${readme_path} (${image_width}x${image_height})."
