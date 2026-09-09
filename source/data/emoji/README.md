# Emoji picker data

`emoji-test.txt` is the Unicode Emoji 17.0 snapshot from
https://www.unicode.org/Public/17.0.0/emoji/emoji-test.txt.

The client reads fully-qualified entries in Unicode's order, using group and
subgroup comments for navigation. Existing shortcode aliases are retained;
Unicode names add aliases for the other entries. The picker works offline.

Restore the exact snapshot with `bash build-scripts/download_emoji_data.sh`
from the repository root. Its SHA-256 is
`1d8a944f88d7952f7ef7c5167fef3c67995bcae24543949710231b03a201acda`.
See [the Unicode license](../../licenses/Unicode.txt).
