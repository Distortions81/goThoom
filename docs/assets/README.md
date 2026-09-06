# Local asset catalog

First reviewed batch: **all 100 image IDs from 0 through 99**, using the local
CL_Images archive on 2026-09-06. Searchable classifications and archive metadata
are in [low-ids-0000-0099.json](low-ids-0000-0099.json).

This is a visual review, not a reconstruction of the original server database.
No sounds or movie occurrences have been classified in this first batch.

## Useful starting points

| Use | IDs | Notes |
| --- | --- | --- |
| Indoor floors | 44, 45 | Gray checkerboard and woven-brick patterns, 200x200 each. |
| Wooden fencing | 34-39 | Modular spans and posts; not automatically solid. |
| Signs | 41 | Wooden signpost, 24x24. |
| Tables | 59, 60 | Narrow and wide orientations. |
| Interior decoration | 58 | Dragon-pattern rug; 199x223, plane -5. |
| Trees | 68 | Leafy green tree, 94x105. |
| Well | 87 | Round stone well, 74x74; identity confirmed by the user. |
| Animated water | 20 | Four 200x200 frames in a 200x800 sheet. |
| Small flowers | 8, 9 | Yellow and red, 8x11 each. |
| Terrain rims | 83-86, 89-99 | Straight and curved brown edges; this family continues beyond 99. |
| Customizable humanoids | 22, 25, 54, 62, 64, 65, 72, 76 | Previewed with default palettes, not particular characters' clothing. |
| Creature visuals | 69, 70, 71, 77, 80, 82, 88 | Beetle-like, lion-like, rat-like, skull, imp-like, ghost-like, skeleton. These are descriptive labels. |
| Ambiguous special props | 19, 49 | Shimmering teal panel and blue arched structures; function still unverified. |

## Evidence and limitations

- `sheet_width`, `sheet_height`, `animation_frames`, `plane`, `flags`, and
  `custom_colors` come directly from the client's existing `climg` reader.
  The archive SHA-256 identifies the resource revision reviewed.
- `label`, `category`, `tags`, and `notes` are manually reviewed descriptions.
  `visual_confidence` measures confidence in the visible description, not an
  official name, class, race, item function, or map location.
- `evidence` distinguishes direct visual review, client-code references, and
  user confirmations. A user-confirmed identity is noted explicitly.
- `animation_frames: 1` does **not** mean a mobile has only one pose. Mobile
  sheets contain directional/pose cells using a separate layout. The preview
  tool shows the whole sheet and an enlarged first cell for mobile-like layouts;
  that layout detection is a heuristic, not proof an asset is a mobile.
- Custom palettes can substantially change a character's appearance. A sprite
  ID does not identify a named player or NPC. Future movie evidence should
  retain descriptor type, name, palette, movie source, and frame reference.
- Draw planes and opaque pixels are not collision geometry. Door anchors,
  footprints, triggers, and seamless tiling need separate scene review.
- Current exports show original decoded colors without denoise or upscale
  filtering. PNGs retain the decoder's one-pixel transparent border; archive
  dimensions in the catalog exclude that border.

## Reproduce the previews

From `source/`, with installed game resources:

```sh
xvfb-run -a go run ./cmd/assetpreview \
  -images /path/to/CL_Images \
  -out /tmp/gothoom-assets-review-0-99 \
  -min 0 -max 99
```

Omit `xvfb-run -a` when a display is already available. The tool uses CPU image
decoding, but `climg` imports Ebitengine, which requires display initialization
on Linux. The output directory's parent must exist, and the directory itself
must **not** exist; the tool refuses to overwrite an earlier review.

The output contains full `image-NNNN.png` sheets, labeled `contact-NN.png`
overview pages (20 assets each), and `metadata.json`. It uses numeric IDs in
sorted order; changing the range to `-min 100 -max 199` produces the next batch.
Contact sheets show the first animation frame; inspect full sheets when
classifying animated assets. Partial exports can remain if an image fails to
decode; a completed export prints its count and writes `metadata.json` last.

Export PNGs locally, outside the repository (or under ignored `asset-exports/`).
The game assets belong to their respective owners. Commit the tooling and
review notes, not an extracted art or audio bundle. The hand-reviewed catalog
is separate from generated `metadata.json` so another export cannot erase labels.

## Quick searches

```sh
# From the repository root: find floor candidates.
jq '.assets[] | select(.tags | index("floor")) | {id, label, plane}' \
  docs/assets/low-ids-0000-0099.json

# Find entries that need more visual or movie evidence.
jq '.assets[] | select(.visual_confidence == "low") | {id, label, notes}' \
  docs/assets/low-ids-0000-0099.json
```

Next steps: review 100-199, add named/paletted sightings from clMov recordings,
and build a separate sound-ID catalog with playable previews. Preserve the
distinction between recorded facts, user confirmations, and visual guesses.
