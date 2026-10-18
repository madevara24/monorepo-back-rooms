# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS build
RUN apk add --no-cache git ca-certificates
WORKDIR /src

# COMMIT_SHA overrides the .git-derived commit, for build contexts that
# don't carry .git (e.g. a shallow export). Pass it with
# --build-arg COMMIT_SHA=<sha> if git rev-parse ever reports "unknown".
ARG COMMIT_SHA=""

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN commit="${COMMIT_SHA:-$(git rev-parse HEAD 2>/dev/null || echo unknown)}" && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w -X main.commit=${commit}" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
