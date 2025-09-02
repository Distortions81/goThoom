# Building with Docker

The repository includes a `Dockerfile` for building goThoom in a reproducible environment.

## Build the image and binaries

```bash
./build-scripts/docker_dev_env.sh
```

This builds an image tagged `gothoom-build-env` and compiles the binaries inside it.

## Extract the build artifacts

```bash
docker create --name gothoom-build gothoom-build-env
mkdir -p dist
docker cp gothoom-build:/out ./dist
docker rm gothoom-build
```

The built cross-platform binaries are now in `dist/` on the host.
