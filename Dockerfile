# syntax=docker/dockerfile:1

FROM ubuntu:24.04 AS gothoom-build-env
ARG DEBIAN_FRONTEND=noninteractive

# Base build tooling and libraries needed for cross-compiling
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential g++-12 libstdc++-12-dev libc6-dev \
    git cmake ninja-build clang llvm lldb pkg-config \
    libgl1-mesa-dev libglu1-mesa-dev xorg-dev libxrandr-dev \
    libasound2-dev alsa-utils libgtk-3-dev xdg-utils \
    libxml2-dev uuid-dev libssl-dev libbz2-dev zlib1g-dev \
    cpio unzip zip xz-utils curl ca-certificates jq \
    osslsigncode imagemagick \
  && rm -rf /var/lib/apt/lists/*

# Go toolchain
RUN curl -fsSL https://go.dev/dl/go1.25.0.linux-amd64.tar.gz -o /tmp/go.tgz \
 && tar -C /usr/local -xzf /tmp/go.tgz \
 && rm /tmp/go.tgz

ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Install osxcross with a macOS 13.3 SDK to enable darwin builds
RUN ./build-scripts/install_osxcross.sh --root /osxcross --sdk-version 13.3 --no-deps
ENV OSXCROSS_ROOT=/osxcross
ENV PATH="$OSXCROSS_ROOT/target/bin:${PATH}"

# Windows resource tool (no need to go get as well)
RUN go install github.com/tc-hib/go-winres@latest

# Build (assumes script writes to /binaries)
RUN bash ./build-scripts/build_binaries.sh

# ---- Packaging stage: copy artifacts out cleanly ----
FROM ubuntu:24.04 AS pack
WORKDIR /out
COPY --from=gothoom-build-env /binaries /out/
CMD ["bash"]
