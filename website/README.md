# goThoom project website

This directory is a standalone static project website. The WASM client files
live separately under `website/wasm/`.

Serve the contents of this directory at `https://gothoom.m45sci.xyz/`. No build
step is required.

The server should return `index.html` for `/`, serve the image and CSS files as
static assets, serve `help/index.html` for `/help/` (with `/help` redirected to
it), and use HTTPS. Versioned Clan Lord data archives may continue to live under
`/data/`; they are not part of this directory.

## Illustrated manual

The hand-written manual has three areas: `help/index.html` for ordinary client
use, `help/automation.html` for macros, scripts, and hotkeys, and
`help/performance.html` for graphics and performance tuning.
`help/reference.html` is a generated visual control reference. All areas share
navigation and search, and display the goThoom test number, Clan Lord data
version, and date relevant to the most recent update. There are no
version-specific routes or automatic redirects.

The screenshots come from real client window constructors, using default
settings, a temporary data folder, and illustrative character/script data.
Capturing does not log in, send commands, download files, or open personal
profiles. The Gamepad window is documented as the work in progress it currently
is. File-path examples use generic locations.

To refresh screenshots, callouts, menu references, and search data from the
repository root after the usual resource/development setup:

```sh
./build-scripts/build_help.sh
```

The script uses the active display or starts its own `xvfb-run` display. It
requires the project's Go toolchain and Python 3, with no extra Python or Node
packages. It calls the opt-in `TestCaptureHelp` test alone because Ebitengine's
game loop cannot be restarted within the same process. The normal Go suite
skips capture. To record a specific editorial update date:

```sh
./build-scripts/build_help.sh --updated 2026-09-08
```

- Add a window/state to `source/help_capture_test.go` to capture another UI.
- The capture exports PNGs and `help/images/capture.json`: labels, help text,
  choices, values shown, and actual rendered control rectangles.
- Write callouts in `help/annotations.json`, naming the target control and an
  explanation of what happens next, what it affects, or when it helps. Do not
  restate its caption or describe basic checkbox/button mechanics. Leave a
  screen or control out when there is nothing useful to add; it stays unboxed.
  Missing targets fail generation so changed labels cannot silently point at
  the wrong control. Coordinates still come from the client, not the author.
- Insert `<!-- help-figure:screen-id --><!-- /help-figure -->` in any manual area to
  embed a generated figure. Edit prose outside those blocks; regeneration
  preserves it. The metadata block is generated the same way.
- Annotated screenshots have HTML highlight boxes with numbered explanations
  and a visibility toggle. Plain screenshots have only their navigation path
  and original-image link. The reference includes menu choices and slider
  ranges where these add information beyond the picture; it does not generate
  filler descriptions for every control. Images, explanations, instructions,
  and navigation work without JavaScript. Search and hiding highlights are
  progressive enhancements.
- Commit the generated pages and images with the relevant UI/documentation
  changes. Serving the site still requires no build step or backend service.

For prose-only edits, rebuild the figures/reference/search from the saved
capture without launching the client, retaining its existing date unless you
have reviewed and updated the manual:

```sh
python3 build-scripts/build_help.py website/help/images --updated 2026-09-08
python3 build-scripts/check_help.py
python3 build-scripts/help_annotations_test.py
node --test build-scripts/help_ui_test.cjs
```

`check_help.py` checks local links and anchors, PNG dimensions, control bounds,
figure/reference freshness, and search-index freshness. Review native captures
when changing the UI, and verify the guide text still describes the workflow;
generation cannot determine whether a procedural explanation is accurate.
