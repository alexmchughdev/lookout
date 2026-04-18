# syntax=docker/dockerfile:1

# ── build stage ───────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags "-s -w -X github.com/alexmchughdev/lookout/cmd.version=${VERSION}" \
    -o /out/lookout .

# ── runtime stage ─────────────────────────────────────────────────────────────
FROM debian:bookworm-slim

# Chromium for chromedp; fonts + CA bundle so screenshots look right and
# HTTPS just works. --no-install-recommends keeps the image lean (~380 MB).
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      chromium \
      ca-certificates \
      fonts-liberation \
      fonts-noto-color-emoji \
      tini \
 && rm -rf /var/lib/apt/lists/*

COPY --from=build /out/lookout /usr/local/bin/lookout

# Working directory for user-mounted specs / reports.
WORKDIR /work

# `tini` handles PID 1 signalling so Ctrl-C and `docker stop` terminate the
# chromium subprocess cleanly instead of leaving zombies.
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/lookout"]
CMD ["--help"]
