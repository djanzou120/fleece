# syntax=docker/dockerfile:1
#
# Image des services Go. Paramétrée par PKG : un seul Dockerfile pour N services.
#   docker build -f docker/go.dockerfile --build-arg PKG=messaging -t anthill/backend:messaging .
# (généralement invoqué via `make image pkg=messaging`).

FROM golang:1.26-bookworm AS build
ARG PKG
ARG VERSION

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends make

WORKDIR /w
COPY . .
RUN make build pkg=$PKG version=$VERSION

# Image runtime minimale (distroless, sans shell).
FROM gcr.io/distroless/base-debian12 AS dist
ARG PKG
COPY --from=build /w/bin/$PKG /usr/local/bin/service
ENTRYPOINT ["/usr/local/bin/service"]
