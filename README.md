# monorepo-back-rooms

MBR — a self-serve Postgres marketplace. See [AGENTS.md](AGENTS.md) for the architecture.

## Layout

- `cmd/api` — control-plane Go API.
- `cmd/node` — data-plane agent (`mbr-node`).
- `internal/` — shared Go packages.
- `web/` — SvelteKit frontend (static build).
- `compose.yaml` — local control Postgres.

## Prerequisites

- Go 1.26+
- Node 22+
- Docker (for the local Postgres container)

## Commands

```bash
make dev     # starts control Postgres, the API, the node agent, and the web dev server
make test    # go vet + go test + svelte-check
make build   # go build ./cmd/api ./cmd/node + npm run build in web/
```

Run each service on its own, if needed:

```bash
docker compose up -d control-db   # Postgres 17 on :5432

go run ./cmd/api                  # :8080, GET /healthz
go run ./cmd/node                 # :8081, GET /healthz

cd web && npm install && npm run dev
```

The web dev server proxies `/healthz` to the API at `http://localhost:8080`
(see `web/vite.config.ts`), so `cmd/api` must be running for the home page to
show `ok`. In production, deploy `web`'s static build behind the same host as
the API so `/healthz` resolves same-origin, same as the proxy does in dev.
