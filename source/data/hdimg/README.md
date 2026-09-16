# HD picture replacements

Put a PNG named `<sprite-id>.png` anywhere in this folder, or place those PNGs
inside a `.zip` file here. Subfolders inside this folder and inside ZIPs are
supported. The client bundles these files at build time and uses them in place
of the corresponding single-frame CL_Images picture, at the picture's original
in-game size.

For duplicate IDs, a loose PNG in this folder wins, then a loose PNG in a
subfolder, then a PNG inside a ZIP. Within the same group, the first path in
alphabetical order wins. Rebuild the client after changing bundled artwork.
During development, use Debug Settings > Reload HD Sprites to read this source
folder again without restarting; Open HD Sprite Preview compares the original
and replacement at the same UI size.
