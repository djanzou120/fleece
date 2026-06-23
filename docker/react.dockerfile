# syntax=docker/dockerfile:1
#
# Image Next.js App Router — paramétrée par PKG (ex: platform-app).
# Un seul Dockerfile pour tout package de type react.
#   docker build -f docker/react.dockerfile --build-arg PKG=platform-app -t anthill/backend:platform-app .
# (généralement invoqué via `make image pkg=platform-app`).
#
# PRÉREQUIS : src/$PKG/next.config.js (ou .ts) DOIT contenir `output: 'standalone'`.
# Sans cette option, le build Next ne génère pas le répertoire .next/standalone
# et le stage `dist` échouera lors du COPY de server.js.
# C'est la responsabilité du frontend-engineer (T-008) de l'activer.

# ---------------------------------------------------------------------------
# Stage build : compile le package via le système de build du monorepo.
# ---------------------------------------------------------------------------
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

# ---------------------------------------------------------------------------
# Stage dist : image runtime minimale.
# Next standalone place les fichiers dans src/$PKG/.next/standalone/.
# Les assets statiques (.next/static) et le répertoire public/ doivent être
# copiés séparément — ils ne sont PAS inclus dans le bundle standalone.
# ---------------------------------------------------------------------------
FROM node:21-slim AS dist

ARG PKG

WORKDIR /app

# Serveur standalone généré par Next (inclut node_modules allégé + server.js)
COPY --from=build /w/src/$PKG/.next/standalone/ ./

# Assets statiques Next (chunks JS, CSS) — servis par le serveur standalone
COPY --from=build /w/src/$PKG/.next/static/ ./.next/static/

# Fichiers publics (images, fonts, favicon, etc.)
# COPY --from=build /w/src/$PKG/public/ ./public/
# (décommentez la ligne ci-dessus une fois que src/$PKG/public/ existe)

EXPOSE 3000

ENV NODE_ENV=production
ENV PORT=3000
# HOSTNAME permet au serveur standalone Next d'écouter sur 0.0.0.0 dans le conteneur
ENV HOSTNAME=0.0.0.0

CMD ["node", "server.js"]
