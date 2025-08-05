# ClanLord Client

A pre-alpha open source (MIT) client for the Clan Lord MMORPG.
The repository contains:

- `go_client/` – a cross-platform Go implementation.
- `old_mac_client/` – historical C client provided for reference only (do not modify).

## Quick Start

### Requirements

- Go 1.24 or newer
- OpenGL and X11 development libraries

On Debian/Ubuntu:

```bash
sudo apt-get update
sudo apt-get install -y golang-go build-essential libgl1-mesa-dev libglu1-mesa-dev xorg-dev
```

### Build

```bash
cd go_client
go build
```

### Run

Launch the client with:

```bash
cd go_client
go run .
```

or use the helper script:

```bash
scripts/run_go_client.sh
```

## Command-line Flags

The Go client accepts the following flags:

- `-host` – server address (default `server.deltatao.com:5010`)
- `-clmov` – play back a `.clMov` movie file instead of connecting to a server
- `-account` – account name for character selection
- `-account-pass` – account password used to retrieve the character list
- `-name` – character name (default `demo`)
- `-pass` – character password (default `demo`)
- `-client-version` – client version number (`kVersionNumber`, default `1440`)
- `-debug` – enable debug logging (default `true`)
- `-scale` – screen scale factor (default `2`)
- `-interp` – enable movement interpolation
- `-onion` – cross-fade sprite animations
- `-noFastAnimation` – draw a mobile's previous animation frame when available
- `-linear` – use linear filtering instead of nearest-neighbor rendering
- `-night` – force night level (0-100)

## Data and Logging

- The default server is `server.deltatao.com:5010`; override it with `-host`.
- Missing `CL_Images` or `CL_Sounds` archives in `go_client/data` are fetched automatically from `https://www.deltatao.com/downloads/clanlord`.
  They are saved as `CL_Images` and `CL_Sounds`.

