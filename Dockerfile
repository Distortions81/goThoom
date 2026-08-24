# syntax=docker/dockerfile:1

FROM ubuntu:24.04 AS builder
ARG DEBIAN_FRONTEND=noninteractive

# ---- Core toolchain & libs ----
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential g++-12 libstdc++-12-dev libc6-dev \
    git cmake ninja-build clang llvm lldb pkg-config \
    libgl1-mesa-dev libglu1-mesa-dev xorg-dev libxrandr-dev \
    libasound2-dev alsa-utils libgtk-3-dev xdg-utils \
    libxml2-dev uuid-dev libssl-dev libbz2-dev zlib1g-dev \
    cpio unzip zip xz-utils curl ca-certificates jq \
    osslsigncode imagemagick \
 && rm -rf /var/lib/apt/lists/*

# ---- Go toolchain ----
ARG GO_VERSION=1.26.6
RUN curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz -o /tmp/go.tgz \
 && tar -C /usr/local -xzf /tmp/go.tgz \
 && rm /tmp/go.tgz
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"

# ---- Workspace ----
WORKDIR /app

# ---- Cross-platform release tools ----
# Install these before copying the project so ordinary source changes can reuse
# the expensive toolchain layers in CI.
COPY build-scripts/install_osxcross.sh ./build-scripts/
RUN ./build-scripts/install_osxcross.sh --root /osxcross --sdk-version 13.3 --no-deps
ENV OSXCROSS_ROOT=/osxcross
ENV PATH="$OSXCROSS_ROOT/target/bin:${PATH}"

# Use the official static release instead of compiling apple-codesign and its
# Rust dependency tree for every clean build.
ARG APPLE_CODESIGN_VERSION=0.29.0
ARG APPLE_CODESIGN_ARCHIVE_SHA256=dbe85cedd8ee4217b64e9a0e4c2aef92ab8bcaaa41f20bde99781ff02e600002
RUN archive="apple-codesign-${APPLE_CODESIGN_VERSION}-x86_64-unknown-linux-musl.tar.gz" \
 && url="https://github.com/indygreg/apple-platform-rs/releases/download/apple-codesign%2F${APPLE_CODESIGN_VERSION}/${archive}" \
 && curl -fsSL "$url" -o "/tmp/${archive}" \
 && echo "${APPLE_CODESIGN_ARCHIVE_SHA256}  /tmp/${archive}" | sha256sum -c - \
 && tar -C /tmp -xzf "/tmp/${archive}" \
 && install -m 0755 \
      "/tmp/apple-codesign-${APPLE_CODESIGN_VERSION}-x86_64-unknown-linux-musl/rcodesign" \
      /usr/local/bin/rcodesign \
 && rm -rf "/tmp/${archive}" \
      "/tmp/apple-codesign-${APPLE_CODESIGN_VERSION}-x86_64-unknown-linux-musl"

# ---- Windows resource tool ----
RUN go install github.com/tc-hib/go-winres@v0.3.3

# spellcheck_words.txt is generated and git-ignored, but spellcheck.go embeds it.
# Keep the download ahead of the source copy so normal edits reuse this layer.
COPY build-scripts/download_spellcheck_dict.sh ./build-scripts/
RUN mkdir -p source \
 && bash ./build-scripts/download_spellcheck_dict.sh

# Copy mod files first to populate module cache in the image
COPY source/go.mod source/go.sum ./source/
COPY source/gt2/go.mod ./source/gt2/
RUN cd source \
 && go env -w GOPROXY=https://proxy.golang.org,direct \
 && go env -w GOSUMDB=sum.golang.org \
 && go mod download

# Bring in the rest of the project
COPY . .

# ---- Build now (optional) to verify toolchain & seed caches) ----
# Comment this out if you want the image to ship without prebuilt artifacts.
RUN GOTHOOM_SKIP_SYSTEM_DEPS=1 bash ./build-scripts/build_binaries.sh

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

# GitHub Actions exports only these files instead of loading the multi-gigabyte
# build environment into Docker just to copy the release archives back out.
FROM scratch AS artifacts
COPY --from=builder /binaries/ /

# Keep the default image as the interactive, offline-capable build environment.
FROM builder AS build-env
