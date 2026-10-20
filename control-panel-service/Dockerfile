# syntax=docker/dockerfile:1
#
# Build context is the repo root (see dokploy-compose.yml and
# compose.yaml: `dockerfile: control-panel-service/Dockerfile`), because
# go.work ties this module to node-agent-service and both must be
# present for `go build` to resolve the workspace.
FROM golang:1.26-alpine AS build
RUN apk add --no-cache git ca-certificates
WORKDIR /src

# COMMIT_SHA overrides the .git-derived commit, for build contexts that
# don't carry .git (e.g. a shallow export). Pass it with
# --build-arg COMMIT_SHA=<sha> if git rev-parse ever reports "unknown".
ARG COMMIT_SHA=""

COPY go.work go.work.sum ./
COPY control-panel-service/go.mod control-panel-service/go.sum control-panel-service/
COPY node-agent-service/go.mod node-agent-service/go.sum node-agent-service/
RUN go -C control-panel-service mod download

COPY . .
RUN commit="${COMMIT_SHA:-$(git rev-parse HEAD 2>/dev/null || echo unknown)}" && \
    CGO_ENABLED=0 GOOS=linux go -C control-panel-service build -trimpath -ldflags "-s -w -X main.commit=${commit}" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
