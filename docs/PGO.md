# Compiler profile

`source/default.pgo` contains CPU samples from movie playback of commit
`3939322`, captured on October 9, 2026. The capture binary used Go 1.27.1,
`CGO_ENABLED=0`, and `-pgo=off` on Linux/amd64.
Captures ran under Xvfb with software OpenGL because this environment has no
hardware GPU device.

The merged workload contains:

- 120 seconds of `tour-2025.08.02.clMov.zip`, following a 10-second warmup.
- 120 seconds of `lore1.clMov.zip`, following a 10-second warmup.

Movie captures used the built-in `-pgo` mode (30 movie updates per second),
a 120 FPS render cap, VSync disabled, and separate isolated user-data
directories with copies of the installed game archives. The window was
1920x1000, with tiled panes, 2x game/artwork scale, and UI scale 1. Audio,
bubble animation, and power saving were disabled. Other rendering settings
used the client defaults. Both captures used the current client and the
movies' own version metadata.

The profiles are summed without normalizing sample weights: 240 seconds of
playback contain 106.21 CPU-seconds of samples. Startup and the initial ten
seconds of playback are excluded. The workload includes moving scenes,
asset preparation, packet parsing, and ordinary client UI updates.

To reproduce, configure an isolated `XDG_DATA_HOME` for each capture with the
settings above and copies of `CL_Images` and `CL_Sounds` in its `goThoom`
directory. Run from the repository root with Go 1.27.1:

```sh
repo_root="$(pwd)"
cd source
CGO_ENABLED=0 go build -pgo=off -o /tmp/gothoom-profile .
xvfb-run -a -s '-screen 0 1920x1080x24' \
  env XDG_DATA_HOME=/tmp/gothoom-pgo-tour /tmp/gothoom-profile \
  -pgo -clmov="$repo_root/source/clmovFiles/tour-2025.08.02.clMov.zip" \
  -pgoWarmup=10s -pgoDuration=120s -pgoMaxFPS=120 -pgoOutput=/tmp/tour.cpu
xvfb-run -a -s '-screen 0 1920x1080x24' \
  env XDG_DATA_HOME=/tmp/gothoom-pgo-lore /tmp/gothoom-profile \
  -pgo -clmov="$repo_root/source/clmovFiles/lore1.clMov.zip" \
  -pgoWarmup=10s -pgoDuration=120s -pgoMaxFPS=120 -pgoOutput=/tmp/lore.cpu
go tool pprof -proto /tmp/tour.cpu /tmp/lore.cpu > default.pgo
```

Use absolute movie paths: the client changes its working directory to the
executable directory during startup. A hardware display can be used instead
of Xvfb when available.

Normal builds automatically consume `default.pgo`. Profile collection is
training input for the compiler. Software rendering changes the workload's
CPU distribution, so this capture does not establish hardware rendering
performance or an additional timing gain.

Validation with this profile passed: `CGO_ENABLED=0 go test ./...` under Xvfb,
a normal client build, and `git diff --check`.
