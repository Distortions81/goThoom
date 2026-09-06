# Compiler profile refresh for test51

`source/default.pgo` contains fresh CPU samples from the renderer and Settings
input optimizations shipped in test51. Capture binaries were built using
Go 1.26.6 with `-pgo=off`, on Linux/amd64, using a hardware OpenGL display
(AMD Radeon Renoir integrated graphics).

The merged workload contains:

- 120 seconds of `lore1.clMov.zip`, following a 10-second warmup.
- 120 seconds of `tour-2025.08.02.clMov.zip`, following a 10-second warmup.
- 30 seconds of moving `-bubbleTorture` with Settings open on Display,
  following a five-second warmup.

Movie captures used the built-in `-pgo` mode (30 movie updates per second),
a 120 FPS render cap, VSync disabled, and an isolated user-data directory.
The window was 1920x1000, with 2x game/artwork scale and UI scale 1. Audio
and bubble animation were disabled. The lore capture used compatibility
metadata 1497; the tour capture used the updated 1501 metadata.

The Settings capture used the temporary timing overlay described in
[SettingsWindowPerformance.md](SettingsWindowPerformance.md), with a fixed
pointer outside the window and uncapped rendering. Its CPU profile was
collected separately from the unprofiled performance comparisons. The
shorter Settings sample supplements the longer movie workload; profiles
were summed without normalizing sample weights. The resulting 270-second
profile contains 143.99 CPU-seconds of samples.

Movie capture commands, run from `source/` with an isolated, configured
`XDG_DATA_HOME` and hardware `DISPLAY`:

```sh
go build -pgo=off -o /tmp/gothoom-profile .
/tmp/gothoom-profile -pgo -clmov=clmovFiles/lore1.clMov.zip \
  -pgoWarmup=10s -pgoDuration=120s -pgoMaxFPS=120 \
  -pgoOutput=/tmp/lore.cpu
/tmp/gothoom-profile -pgo -clmov=clmovFiles/tour-2025.08.02.clMov.zip \
  -pgoWarmup=10s -pgoDuration=120s -pgoMaxFPS=120 \
  -pgoOutput=/tmp/tour.cpu
go tool pprof -proto /tmp/lore.cpu /tmp/tour.cpu /tmp/settings.cpu > default.pgo
```

Normal builds automatically consume `default.pgo`. Profile collection is
training input for the compiler, not evidence of an additional timing gain.

Validation with this profile: `go test ./...` under Xvfb, `go build ./...`,
and `build-scripts/build_binaries_local.sh` passed for Linux/amd64,
Windows/amd64, macOS/arm64, and macOS/amd64. Local package signing was skipped
because signing credentials/tools were not configured.
