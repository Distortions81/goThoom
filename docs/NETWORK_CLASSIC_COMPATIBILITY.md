# Network compatibility with the classic client

This document tracks differences between goThoom's live networking path and
the classic Clan Lord client. The goal is protocol correctness first, followed
by accurate diagnostics and timing behavior. Movie playback is out of scope
unless a networking change shares code with it.

## Protocol invariants

- A successful login starts a new draw-state session with acknowledged frame
  `0` and requested resend frame `-1`.
- UDP preserves datagram boundaries. One datagram contains exactly one
  two-byte-length-prefixed protocol message.
- TCP is a byte stream. A partial header or payload remains pending until the
  rest arrives; a temporary lack of bytes must not discard it.
- A draw frame is accepted after its framing, key spool, and packed draw data
  validate. Failure to interpret an optional logical state record must not
  turn an otherwise valid draw frame into packet loss.
- Command latency is measured from the first transmission of a command number
  to the server acknowledgement of that same command number.
- Live frame time is derived from the server's frame acknowledgement, not the
  client's render rate.
- TCP and UDP messages must not mutate shared session state concurrently.
- The classic protocol's maximum framed message payload is 1396 bytes.

## Work list

### P0: correctness and false packet loss

- [x] Preserve fragmented length-prefixed draw-state records across frames.
  Implemented in `8cc4bcd`.
- [x] Reset all live-session state at login, including frame acknowledgement,
  resend bootstrap, loss counters, frame timing, latency tracking, and queued
  input state.
- [x] Accept structurally valid draw frames even when a complete optional state
  record cannot be decoded. Log it and skip the rest of that logical record.
- [x] Treat UDP reads as atomic datagrams and reject length mismatches instead
  of joining or splitting datagrams.
- [x] Preserve partial TCP frames and validate framed sizes before indexing or
  allocating.
- [x] Harden login challenge and response length checks before slicing fields.

### P1: diagnostics and timing

- [x] Correlate ping and jitter samples with acknowledged command numbers;
  remove the arbitrary "latest UDP packet after latest input" measurement.
- [x] Use the accepted server acknowledgement as the live frame clock.
- [x] Advance script tick waits from accepted server/movie frames only, not
  from Ebitengine `Update`, so vsync and render FPS cannot change script time.
- [x] Preserve the measured server frame interval without integer FPS
  quantization.
- [x] Serialize inbound TCP and UDP processing (with classic TCP priority if a
  dispatcher is introduced) or otherwise make all shared session state safe.

### P2: follow-up compatibility

- [x] Match the classic 511-byte player-command limit with an explicit policy
  instead of allowing a framed player-input packet to grow arbitrarily.
- [x] Replace the toolbar's manual ping action, which timed new TCP handshakes,
  with the active connection's command-acknowledgement latency sample.
- [x] Run the race detector over focused networking and draw-state tests after
  inbound processing is serialized.

## Acceptance checks

- Reconnecting cannot request a frame from the previous session.
- A malformed logical state record produces one warning but the enclosing
  frame still advances `ackFrame` and acknowledges its command.
- A short, oversized, or length-mismatched UDP datagram is discarded in full;
  the next valid datagram parses normally.
- A TCP message split across reads, including a pause between chunks, parses
  exactly once without losing bytes.
- Ping samples appear only when the matching command acknowledgement arrives.
- A 240 FPS render loop does not increase network send cadence or script tick
  cadence.
- `go test ./...` and `git diff --check` pass from `source/` and the repository
  root respectively.
