# Multi-stage build: compile a static binary, then package it alone on scratch.
# The builder always runs on the build machine's own architecture and
# cross-compiles via TARGETOS/TARGETARCH, so building for a platform other
# than the build machine's does not drag the Go toolchain through emulation.
FROM --platform=$BUILDPLATFORM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/aws-ddns ./cmd/aws-ddns
# Pre-created writable state directory (owned by the runtime user) — named
# volumes initialised from the image inherit this ownership.
RUN mkdir -p /out/state

# Minimal runtime: only the binary and the CA bundle (HTTPS to the discovery
# endpoints and AWS). No shell, no package manager, no exposed ports. No USER
# directive — the container engine (or the deployment's own `user:` setting)
# decides who runs it, so a bind-mounted data folder works without ownership
# preparation.
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/aws-ddns /aws-ddns
COPY --from=build /out/state /var/lib/aws-ddns
ENTRYPOINT ["/aws-ddns"]
