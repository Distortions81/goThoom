# goThoom Client

A open source (MIT) client for the Clan Lord MMORPG.

This repository hosts the Go implementation of the client. The
`old_mac_client/` directory contains a historical C implementation provided
for reference only (do not modify).

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

From the repository root run:

```bash
go build
```

### Run

Launch the client with:

```bash
go build
./gothoom
```

To exercise parsing and the GUI without a server, replay a captured
network trace:

```bash
go run . -pcap reference-client.pcapng
```

To build release binaries for Linux and Windows, use:

```bash
scripts/build_binaries.sh
```

## Command-line Flags

The Go client accepts the following flags:

- `-clmov` – play back a `.clMov` movie file instead of connecting to a server
- `-pcap` – replay network frames from a `.pcap/.pcapng` file
- `-pgo` – create `default.pgo` by playing `test.clMov` at 30 fps for 30 seconds
- `-client-version` – client version number (`kVersionNumber`, default `1445`)
- `-debug` – enable debug logging (default `true`)

## Setup

- Missing `CL_Images` or `CL_Sounds` archives in `data` are fetched automatically

