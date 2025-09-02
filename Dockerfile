FROM ubuntu:24.04

# Base build tooling and libraries needed for cross-compiling the client on
# Linux, macOS (via osxcross) and Windows.  This mirrors the dependencies used
# by build-scripts/build_binaries.sh so the container can produce release
# binaries for all platforms without further setup.
RUN apt-get update && apt-get install -y \
    build-essential g++-12 libstdc++-12-dev libc6-dev \
    git cmake ninja-build clang llvm lldb pkg-config \
    libgl1-mesa-dev libglu1-mesa-dev xorg-dev libxrandr-dev \
    libasound2-dev alsa-utils libgtk-3-dev xdg-utils \
    libxml2-dev uuid-dev libssl-dev libbz2-dev zlib1g-dev \
    cpio unzip zip xz-utils curl ca-certificates jq \
    osslsigncode imagemagick && sudo \
    rm -rf /var/lib/apt/lists/*

RUN curl -LO https://go.dev/dl/go1.25.0.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz \
    && rm go1.25.0.linux-amd64.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"
ENV PATH="/root/go/bin:${PATH}"

WORKDIR /app

RUN curl -LO https://m45sci.xyz/u/dist/goThoom/gothoom_deps.tar.gz \
    && tar -xzf gothoom_deps.tar.gz \
    && rm gothoom_deps.tar.gz

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Install osxcross with a macOS 13.3 SDK to enable darwin builds
RUN build-scripts/install_osxcross.sh --root /osxcross --sdk-version 13.3 --no-deps
ENV OSXCROSS_ROOT=/osxcross
ENV PATH="$OSXCROSS_ROOT/target/bin:${PATH}"

# Windows builds embed resources via go-winres
RUN go install github.com/tc-hib/go-winres@latest
RUN go get github.com/tc-hib/go-winres@latest
RUN bash /build-scripts/build_binaries.sh

FROM gothoom-build-env AS builder
CMD cp /binaries/ 

CMD ["bash"]
