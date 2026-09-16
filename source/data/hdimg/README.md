# HD picture replacements

Put a PNG named `<sprite-id>.png` anywhere in this folder, or place those PNGs
inside a `.zip` file here. Subfolders inside this folder and inside ZIPs are
supported. Keep this folder beside the application as `data/hdimg`, or put it
in the client user-data folder as `hdimg`; both are read at runtime, so packs
can be added, removed, or changed without rebuilding. When both provide the
same sprite, the game-directory pack wins.
Enable **Use sprite pack files** in Graphics & Performance to use compatible
single-frame replacements at their original in-game size.

For duplicate IDs, a loose PNG in this folder wins, then a loose PNG in a
subfolder, then a PNG inside a ZIP. Within the same group, the first path in
alphabetical order wins. Use Debug Settings > Reload HD Sprites after changing
pack files; Open HD Sprite Preview compares the original and replacement at
the same UI size.
