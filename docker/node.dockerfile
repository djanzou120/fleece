# syntax=docker/dockerfile:1

# Build the package in a dedicated stage.
FROM node:21-slim AS build

ARG PKG
ARG VERSION

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt update && apt install -y make 

RUN mkdir -p /w
WORKDIR /w

COPY . .
RUN make build pkg=$PKG

# dist is used for node projects
FROM node:21-slim AS dist
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt update && apt install -y openssl

COPY --from=build /w/bin/$PKG /usr/local/bin/$PKG


# dist-caddy is used for SPAs
FROM caddy:2-alpine AS dist-caddy
ARG PKG
COPY --from=build /w/bin/$PKG /srv
