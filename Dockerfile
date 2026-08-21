# syntax=docker/dockerfile:1

FROM ubuntu:24.04
ARG DEBIAN_FRONTEND=noninteractive

# ---- Core toolchain & libs ----
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential g++-12 libstdc++-12-dev libc6-dev \
    git cmake ninja-build clang llvm lldb pkg-config \
    libgl1-mesa-dev libglu1-mesa-dev xorg-dev libxrandr-dev \
    libasound2-dev alsa-utils libgtk-3-dev xdg-utils \
    libxml2-dev uuid-dev libssl-dev libbz2-dev zlib1g-dev \
    cpio unzip zip xz-utils curl ca-certificates jq \
    osslsigncode imagemagick libpcsclite-dev pcscd \
 && rm -rf /var/lib/apt/lists/*

# ---- Go toolchain ----
ARG GO_VERSION=1.26.6
RUN curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz -o /tmp/go.tgz \
 && tar -C /usr/local -xzf /tmp/go.tgz \
 && rm /tmp/go.tgz
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"

# ---- Workspace ----
WORKDIR /app

# Copy mod files first to populate module cache in the image
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://proxy.golang.org,direct \
 && go env -w GOSUMDB=sum.golang.org \
 && go mod download

# Bring in the rest of the project
COPY . .

# spellcheck_words.txt is generated and git-ignored, but spellcheck.go embeds it.
RUN bash ./build-scripts/download_spellcheck_dict.sh

# ---- osxcross (macOS SDK for darwin builds) ----
# Your script should fetch or embed the SDK; after this runs, /osxcross is fully usable offline.
RUN ./build-scripts/install_osxcross.sh --root /osxcross --sdk-version 13.3 --no-deps
ENV OSXCROSS_ROOT=/osxcross
ENV PATH="$OSXCROSS_ROOT/target/bin:${PATH}"

# ---- Rust + apple-codesign (for mac signing) ----
# We keep the resulting binary and caches so future runs don't need the network.
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y \
 && . "$HOME/.cargo/env" \
 && rustup default stable \
 && cargo install apple-codesign
ENV PATH="/root/.cargo/bin:${PATH}"

# ---- Windows resource tool ----
RUN go install github.com/tc-hib/go-winres@latest

# ---- Build now (optional) to verify toolchain & seed caches) ----
# Comment this out if you want the image to ship without prebuilt artifacts.
RUN bash ./build-scripts/build_binaries.sh

# Keep a predictable artifacts dir inside the image
RUN mkdir -p /binaries \
 && if [ -d /app/binaries ]; then cp -a /app/binaries/. /binaries/; fi

# Nice-to-have: a simple helper to rebuild inside the container when offline
RUN printf '%s\n' \
  '#!/usr/bin/env bash' \
  'set -euo pipefail' \
  'cd /app' \
  'bash ./build-scripts/build_binaries.sh' \
  'mkdir -p /binaries && cp -a /app/binaries/. /binaries/' \
  > /usr/local/bin/rebuild && chmod +x /usr/local/bin/rebuild

# Default shell; you can override with `docker run ... rebuild`
CMD ["bash"]
