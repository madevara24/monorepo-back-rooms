# monorepo-back-rooms

MBR — a self-serve Postgres marketplace. See [AGENTS.md](AGENTS.md) for the architecture.

## Layout

One top-level folder per deployable stack, each its own Go module, tied
together by the root `go.work`:

- `control-panel-service/` — control-plane Go API (`cmd/api`). Layer-first
  interior: `entities` (models), `repository` (pgx queries and goose
  migration runner), `usecase` (auth, jobs, routes), `handler` (HTTP),
  `pkg` (password/session/credentials primitives), `migrations` (embedded
  SQL), and `internal/container.go` (composition root).
- `node-agent-service/` — data-plane agent (`cmd/node`, `mbr-node`). Same
  layering: `entities`, `repository` (Docker/volume adapters), `usecase`
  (provision/delete/reconcile/heartbeat), `handler` (HTTP, its own
  `/healthz`), `internal/container.go`.
- `connection-router/` — the SNI-routing TCP proxy in front of the shared
  tenant port `:5432` (`cmd/router`). See
  [`connection-router/README.md`](connection-router/README.md) for the
  handshake it implements, its config, and the migration/rollback runbook.
- `control-web/` — SvelteKit frontend (static build). Routes: `/`,
  `/signup`, `/login`, `/dashboard`, `/billing`.
- `compose.yaml` — local control Postgres.

## Prerequisites

- Go 1.26+
- Node 22+
- Docker (for the local Postgres container)

## Commands

```bash
make dev     # starts control Postgres, the API, the node agent, and the web dev server
make test    # go vet + go test (both modules) + svelte-check
make build   # go build ./control-panel-service/cmd/api ./node-agent-service/cmd/node + npm run build in control-web/
```

Run each service on its own, if needed:

```bash
docker compose up -d control-db   # Postgres 17 on :5432

go run ./control-panel-service/cmd/api    # :8080, GET /healthz
go run ./node-agent-service/cmd/node      # :8081, GET /healthz

cd control-web && npm install && npm run dev
```

The web dev server proxies `/healthz` and `/api` to the API at
`http://localhost:8080` (see `control-web/vite.config.ts`), so `cmd/api`
must be running for the home page and the signup/login/dashboard/billing
pages to work. In production, deploy `control-web`'s static build behind
the same host as the API so `/healthz` and `/api` resolve same-origin,
same as the proxy does in dev.

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

- `mbr-api` — built from the repo root with
  `dockerfile: control-panel-service/Dockerfile` (the build context stays
  at the repo root because `go.work` ties `control-panel-service` to
  `node-agent-service`), port 8080, handles `/api/*` and `/healthz`
  (priority 10).
- `mbr-web` — built from `control-web/Dockerfile`, port 80, serves
  everything else on the host (priority 1).
- `mbr-control-db` — `postgres:17` with a named volume, no published host
  port, only reachable from `mbr-api` on `dokploy-network`.

The data plane, S3, and snapshots are out of scope for this cutover.

`control-panel-service/Dockerfile` derives the build's commit SHA by
running `git rev-parse HEAD` inside the build stage, which assumes
Dokploy's build context includes `.git` (true for a normal git clone,
which is how Dokploy builds compose apps). If a deploy's `/healthz` ever
reports `"commit":"unknown"` or the `Wait for the live commit to match`
step in `deploy.yml` times out while logging a `last seen commit` that
isn't the dispatched SHA, that assumption has broken; pass the correct
SHA explicitly with `--build-arg COMMIT_SHA=<sha>` (wired into the
Dockerfile as a fallback) until the build context is fixed.

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
| `DOKPLOY_MBR_ROUTER_COMPOSE_ID` | The Dokploy compose app id for the separate `mbr-router` app (see [Production (connection router)](#production-connection-router)). Only used by the `deploy-router` job. |

### Deploying

`deploy.yml` runs on every push to `main`, and can also be triggered
manually:

1. A `gate` job runs the same checks as `ci.yml` (`go vet`, `go test`, the
   web build).
2. A `deploy` job SSHes into the VPS and calls the Dokploy API
   (`compose.deploy`) for `DOKPLOY_MBR_COMPOSE_ID`, then polls
   `https://mbr.madevara.com/healthz` for up to 300s until its `commit`
   field equals the deployed commit SHA. This job always runs; it never
   touches the router.
3. A `deploy-router` job exists but only runs when the workflow is
   dispatched manually with its `deploy_router` input checked. It
   deploys the separate `mbr-router` app (`DOKPLOY_MBR_ROUTER_COMPOSE_ID`)
   and polls its loopback-only `/healthz` over SSH. See
   [`connection-router/README.md`](connection-router/README.md#migration-runbook)
   for why this is deliberately not part of the normal push-to-main path.

Merging to `main` deploys `mbr-api`/`mbr-web` automatically; the router
is never deployed by a merge. Deploys run one at a time
(`concurrency: group: deploy`), so a merge queued behind an in-flight
deploy waits instead of racing it. To deploy without a merge, open the
Actions tab and run `Deploy` manually.

## Production (connection router)

`connection-router` deploys on the same VPS and Dokploy instance as
`mbr-api`/`mbr-web`, but as its own, separate Dokploy compose app
(`dokploy-compose.router.yml`), so a routine control-plane deploy never
restarts the live `:5432` data path. Full details — the handshake it
implements, its config, the Dokploy app setup, and the migration and
rollback runbook for cutting it over from nginx — live in
[`connection-router/README.md`](connection-router/README.md).

## Production (data plane)

The data plane runs `mbr-node` as a plain binary under systemd on its own
VPS; there is no CI-built artifact or container image for it. Build and
install it manually after every change:

```bash
git pull
go build -o /usr/local/bin/mbr-node ./node-agent-service/cmd/node
systemctl restart mbr-node
```

The `go build` source path moved from `./cmd/node` to
`./node-agent-service/cmd/node` when the repo split into per-stack Go
modules; the binary's name, env vars, and systemd unit are unchanged.
The unit (e.g. `/etc/systemd/system/mbr-node.service`) runs the binary
with the same env it has always needed:

```
AGENT_ADDR, AGENT_DATA_DIR, AGENT_NODE_ID, AGENT_HOST_ADDRESS, AGENT_IMAGE,
AGENT_PORT_MIN, AGENT_PORT_MAX, AGENT_TLS_CERT, AGENT_TLS_KEY,
AGENT_CLIENT_CA, AGENT_TENANT_TLS_CERT, AGENT_TENANT_TLS_KEY,
CONTROL_URL, AGENT_CONTROL_TOKEN
```
