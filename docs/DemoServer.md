# Demo test server

A minimal multiplayer playground for goThoom. It has shared demo characters,
mouse movement in a bounded empty practice area, chat, and no persistent state.
It is not a Clan Lord game-world implementation.

## Run locally

From the repository root:

```sh
cd source
go run ./cmd/demoserver
```

The default is `127.0.0.1:5010`, with eight slots (`Demo1` through `Demo8`).
The server is headless and does not need the resource bundle or a display.
It uses Go 1.26.6.

In goThoom:

1. Open Settings and find **Server address** in the network settings.
2. Set it to `127.0.0.1:5010` before connecting.
3. Select **Free Demo** on the login screen and log in.
4. Hold the left mouse button in the game view to walk. Type and press Enter
   to chat. Open a second client to try it with another player.

The existing demo login discovers the available character names and retries
another slot if one is busy. You can also add `Demo1` manually with password
`demo`. The slots are shared test identities, not private accounts.

## Let friends connect

```sh
go run ./cmd/demoserver -listen :5010 -slots 8
```

Set each client's server address to the host's IP or hostname plus `:5010`.
Allow **both TCP and UDP** on that port. The server binds only to loopback unless
an explicit listen address is supplied. `-slots` accepts 1–16.

## Commands

- `/who` lists online demo characters.
- `/help` shows the short help message.
- `/quit` disconnects and releases the slot.

Any other ordinary text is broadcast to everyone in the practice area, including
the sender. Commands are acknowledged, and retries do not repeat the same chat.

## Scope

World snapshots run at 10 Hz over TCP. UDP is used for the classic connection
handshake and normal client input. A slot is released on disconnect; inactive
clients time out after 30 seconds. Positions and chat are lost on shutdown.
There are no monsters, combat, inventory, saved characters, terrain/collision
maps, account registration, asset downloads, or backend player metadata.
The client still needs its usual installed Clan Lord artwork to draw characters.

Stop with Ctrl-C. No client source changes or production server changes are
needed. Restore the normal server address in Settings when finished.

## Validate

```sh
cd source
go test . -run TestDemoServerClientCompatibility -count=1 -timeout=30s
go test ./internal/demoserver ./cmd/demoserver
```

The client compatibility test exercises the real demo discovery and challenge
response, two logins, busy and bad-password rejection, movement, chat delivery,
command retry deduplication, draw parsing, and server shutdown.
