# AGENTS

This repository contains the Go-based ThoomSpeak client for the Clan Lord MMORPG.

Do not compile or run tests unless explicitly instructed. After modifying Go code, run:

```bash
go fmt ./...
go vet ./...
```

## Installing dependencies

Install Go 1.24 or later. On Debian/Ubuntu you can run:

```bash
sudo apt-get update
sudo apt-get install -y golang-go build-essential libgl1-mesa-dev libglu1-mesa-dev xorg-dev
```

`libgl1-mesa-dev`, `libglu1-mesa-dev`, and `xorg-dev` provide the OpenGL and X11 libraries required by Ebiten. On other distributions install the equivalent development packages.

## Build and run

To build the client from the repository root:

```bash
go build
```

To run directly:

```bash
go run .
```

The `scripts` directory contains helper scripts such as `scripts/build_binaries.sh` for building release binaries.

The `old_mac_client/` directory contains a historical C implementation for reference only and should not be modified.

Running the client without a display (i.e. no `$DISPLAY` variable) will exit with an X11 initialization error.

