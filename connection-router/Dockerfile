# syntax=docker/dockerfile:1
#
# Build context is the repo root (see dokploy-compose.router.yml:
# `dockerfile: connection-router/Dockerfile`), because go.work ties
# this module to control-panel-service and node-agent-service, and all
# three must be present for `go build` to resolve the workspace.
FROM golang:1.26-alpine AS build
RUN apk add --no-cache git ca-certificates
WORKDIR /src

# COMMIT_SHA overrides the .git-derived commit, for build contexts that
# don't carry .git (e.g. a shallow export). Pass it with
# --build-arg COMMIT_SHA=<sha> if git rev-parse ever reports "unknown".
ARG COMMIT_SHA=""

COPY go.work go.work.sum ./
COPY connection-router/go.mod connection-router/go.sum connection-router/
COPY control-panel-service/go.mod control-panel-service/go.sum control-panel-service/
COPY node-agent-service/go.mod node-agent-service/go.sum node-agent-service/
RUN go -C connection-router mod download

COPY . .
RUN commit="${COMMIT_SHA:-$(git rev-parse HEAD 2>/dev/null || echo unknown)}" && \
    CGO_ENABLED=0 GOOS=linux go -C connection-router build -trimpath -ldflags "-s -w -X main.commit=${commit}" -o /out/router ./cmd/router

# The router listens on :15432, an unprivileged port, so it needs no
# NET_BIND_SERVICE capability; USER 65532:65532 (distroless's "nonuser")
# matches the host map's existing owner (see dokploy-compose.router.yml).
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/router /router
EXPOSE 15432
USER 65532:65532
ENTRYPOINT ["/router"]
