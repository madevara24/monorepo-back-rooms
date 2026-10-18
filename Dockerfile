# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS build
RUN apk add --no-cache git ca-certificates
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN commit="$(git rev-parse HEAD 2>/dev/null || echo unknown)" && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w -X main.commit=${commit}" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
