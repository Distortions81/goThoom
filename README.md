# goThoom Client

goThoom is a pre-alpha, open source (MIT) client for the Clan Lord MMORPG
written in Go. The `old_mac_client/` directory contains a historical C
implementation provided for reference only and should **not** be modified.

## Quick Start

### Install dependencies

- Go 1.24 or newer
- OpenGL and X11 development libraries

On Debian/Ubuntu:

```bash
sudo apt-get update
sudo apt-get install -y golang-go build-essential libgl1-mesa-dev libglu1-mesa-dev xorg-dev
```

### Build

From the repository root run:

```bash
go fmt ./...
go vet ./...
go build
```

Alternatively, `scripts/build_gothoom.sh` will fetch Go modules and build the
client in one step.

### Run

Launch the client with:

```bash
go run .
```

The helper script `scripts/run_gothoom.sh` performs the same steps.

To exercise parsing and the GUI without a server, replay a captured
network trace:

```bash
go run . -pcap reference-client.pcapng
```

To build release binaries for Linux and Windows, use `scripts/build_binaries.sh`.

### Discord Rich Presence

Set the `DISCORD_APP_ID` environment variable to enable Discord Rich Presence via
[rich-go](https://github.com/hugolgst/rich-go).

## Command-line Flags

The Go client accepts the following flags:

- `-host` – server address (default `server.deltatao.com:5010`)
- `-clmov` – play back a `.clMov` movie file instead of connecting to a server
- `-pcap` – replay network frames from a `.pcap/.pcapng` file
- `-pgo` – create `default.pgo` by playing `test.clMov` at 60 fps for 30 seconds
- `-client-version` – client version number (`kVersionNumber`, default `1440`)
- `-debug` – enable debug logging (default `true`)
- `-scale` – screen scale factor (default `2`)
- `-interp` – enable movement interpolation
- `-onion` – cross-fade sprite animations
- `-noFastAnimation` – draw a mobile's previous animation frame when available
- `-night` – force night level (0-100)

## Data and Logging

- The default server is `server.deltatao.com:5010`; override it with `-host`.
- Missing `CL_Images` or `CL_Sounds` archives in `data` are fetched automatically from `https://m45sci.xyz/downloads/clanlord`.
  They are saved as `CL_Images` and `CL_Sounds`.

