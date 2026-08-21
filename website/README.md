# goThoom project website

This directory is a standalone static project website. It is separate from the
WASM client and the existing `web/` bundle.

Serve the contents of this directory at `https://gothoom.m45sci.xyz/`. No build
step is required.

The server should return `index.html` for `/`, serve the image and CSS files as
static assets, and use HTTPS. Versioned Clan Lord data archives may continue to
live under `/data/`; they are not part of this directory.
