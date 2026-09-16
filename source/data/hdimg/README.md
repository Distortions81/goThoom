# HD picture replacements

Put a PNG named `<sprite-id>.png` anywhere in this folder, or place those PNGs
inside a `.zip` file here. Subfolders inside this folder and inside ZIPs are
supported. Keep this folder beside the application as `data/hdimg`, or put it
in the client user-data folder as `hdimg`; both are read at runtime, so packs
can be added, removed, or changed without rebuilding. When both provide the
same sprite, the game-directory pack wins.
Enable **Settings → Experimental → Use sprite pack files** to use
compatible single-frame replacements at their original in-game size. It defaults
off and does not download artwork. On macOS, use the user-data folder to avoid
placing packs inside the application bundle.

For duplicate IDs, a loose PNG in this folder wins, then a loose PNG in a
subfolder, then a PNG inside a ZIP. Within the same group, the first path in
alphabetical order wins. Use **Settings → Experimental → Reload HD Sprites**
after changing pack files; **View HD Sprite Replacements** compares the
original and replacement at the same UI size. Its **Sprite pack** menu can
browse each ZIP separately or show the PNGs stored loose in the `hdimg`
folders.

Keep the PNG canvas proportional to the original, including its transparent
margins. For preparation, exports, and comparisons, see the
[artwork authoring guide](https://gothoom.m45sci.xyz/help/artwork.html).
