# monorepo-back-rooms

MBR — a self-serve Postgres marketplace. See [AGENTS.md](AGENTS.md) for the architecture.

## Layout

- `cmd/api` — control-plane Go API.
- `cmd/node` — data-plane agent (`mbr-node`).
- `internal/` — shared Go packages: `store` (goose migrations and queries),
  `auth` (argon2id and sessions), `api` (HTTP handlers), `healthz`.
- `web/` — SvelteKit frontend (static build). Routes: `/`, `/signup`,
  `/login`, `/dashboard`, `/billing`.
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

The web dev server proxies `/healthz` and `/api` to the API at
`http://localhost:8080` (see `web/vite.config.ts`), so `cmd/api` must be
running for the home page and the signup/login/dashboard/billing pages to
work. In production, deploy `web`'s static build behind the same host as
the API so `/healthz` and `/api` resolve same-origin, same as the proxy
does in dev.

`cmd/api` applies its goose migrations on startup against `DATABASE_URL`
(default `postgres://mbr:mbr@localhost:5432/mbr?sslmode=disable`, matching
`compose.yaml`). This slice's control-plane API covers accounts, sessions,
listing databases, and billing; it returns `503` for database provisioning
and teardown and `501` for snapshots, since it runs without the data-plane
VPS or S3 (see `AGENTS.md`).

## Production (control plane)

The control plane runs on the existing VPS under Dokploy, as the compose
app `mbr` (`dokploy-compose.yml`), on the external `dokploy-network`, with
Traefik routing `mbr.madevara.com`:

- `mbr-api` — built from the repo root `Dockerfile`, port 8080, handles
  `/api/*` and `/healthz` (priority 10).
- `mbr-web` — built from `web/Dockerfile`, port 80, serves everything else
  on the host (priority 1).
- `mbr-control-db` — `postgres:17` with a named volume, no published host
  port, only reachable from `mbr-api` on `dokploy-network`.

The data plane, S3, and snapshots are out of scope for this cutover.

`Dockerfile` derives the build's commit SHA by running `git rev-parse
HEAD` inside the build stage, which assumes Dokploy's build context
includes `.git` (true for a normal git clone, which is how Dokploy builds
compose apps). If a deploy's `/healthz` ever reports `"commit":"unknown"`
or the `Wait for the live commit to match` step in `deploy.yml` times out
while logging a `last seen commit` that isn't the dispatched SHA, that
assumption has broken; pass the correct SHA explicitly with
`--build-arg COMMIT_SHA=<sha>` (wired into the Dockerfile as a fallback)
until the build context is fixed.

### Production env vars

Set these on the Dokploy `mbr` app (never committed; `.env` is
git-ignored):

| Variable | Set by | Purpose |
|---|---|---|
| `DATABASE_URL` | Dokploy app env | `mbr-api`'s connection string to `mbr-control-db`. |
| `POSTGRES_PASSWORD` | Dokploy app env | Password for the `mbr-control-db` superuser. |
| `API_COOKIE_SECURE` | hardcoded `true` in `dokploy-compose.yml` | Marks the session cookie `Secure`; always true behind TLS in production. |

Repo secrets, for `.github/workflows/deploy.yml`:

| Secret | Purpose |
|---|---|
| `VPS_SSH_KEY` | Private key for SSHing into the VPS. |
| `VPS_KNOWN_HOST` | The VPS's known-hosts entry, to pin `ssh`. |
| `VPS_USER` / `VPS_HOST` | SSH target. |
| `DOKPLOY_KEY` | Dokploy API key, used from the VPS to call its own API. |
| `DOKPLOY_MBR_COMPOSE_ID` | The Dokploy compose app id for `mbr`. |

### Deploying

`deploy.yml` is `workflow_dispatch` only until the first cutover is
verified:

1. A `gate` job runs the same checks as `ci.yml` (`go vet`, `go test`, the
   web build).
2. A `deploy` job SSHes into the VPS and calls the Dokploy API
   (`compose.deploy`) for `DOKPLOY_MBR_COMPOSE_ID`, then polls
   `https://mbr.madevara.com/healthz` for up to 300s until its `commit`
   field equals the dispatched commit SHA.

To deploy: open the Actions tab, run `Deploy` manually, and watch it go
green. Once a manual dispatch has gone green, add a `push: branches:
[main]` trigger to `deploy.yml` to make deploys automatic.
