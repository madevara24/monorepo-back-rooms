# MBR — monorepo-back-rooms

A self-serve Postgres marketplace. A user picks a database name and a size in GB, and receives a running Postgres instance with a connection string. Flat price: IDR 10,000 per GB per month, provisioned size, billed hourly for proration. One seller, payments mocked, portfolio piece.

The full design and the reasoning behind each choice live in the vault note `Projects/MBR/`. This file is the repo's working summary.

## Deploy contract (read first)

Merging to `main` deploys. `auto_merge: true` in the task pipeline is production deploy authorization, not merge-only permission.

A new deployable stack is not done until:

1. it has its own Dokploy app/compose, or an explicit documented manual exception;
2. `.github/workflows/deploy.yml` has a push-to-main job with a path filter for that stack;
3. unrelated stack changes cannot restart it.

If a new stack lacks a deploy path, the task is blocked and `auto_merge` stays false. A one-time cutover exception must be temporary and linked to a follow-up task; the router's manual gate is `MBR-19`, not the permanent state. Every merge is live only after the stack's health check reports the merged commit.

The stack-to-deploy map and current exceptions are in `DEPLOY.md`.

## Architecture

Two planes, never on the same host.

| Plane | What runs there |
|---|---|
| Control | `control-panel-service` (`cmd/api`, Go), `control-web/` (SvelteKit static), control Postgres 17. Existing VPS, Dokploy, `mbr.madevara.com`. |
| Data | `node-agent-service` (`cmd/node`, `mbr-node`), Docker, one Postgres container per tenant. Separate VPS. |
| Storage | S3 bucket for snapshots, written by the agent. |

Each deployable stack is its own top-level folder and Go module (`control-panel-service/go.mod`, `node-agent-service/go.mod`, tied together by the root `go.work`), with a layer-first interior: `entities` (models), `repository` (pgx/Docker), `usecase` (business logic), `handler` (net/http), and a `container.go` composition root.

The control plane is the source of truth for users, databases, sizes, status, snapshots, and invoices. The data plane holds the bytes. `mbr-node` is the only process that talks to Docker; the API calls it over mTLS.

Tenants connect at `<slug>.db.mbr.madevara.com:<port>`. v1 publishes one host port per container; the SNI proxy is a later chapter. TLS is required: `hostssl` only, `sslmode=require` floor.

## One database's life

1. `POST /databases` writes a `databases` row in `provisioning` and a job.
2. The worker calls the agent with the wanted state.
3. The agent creates a loopback ext4 volume sized to the plan, a container with CPU and memory limits, a generated password, the TLS certificate, and a free host port.
4. The agent reports `running`; the API shows the connection string once.
5. Hourly: record `size_gb`. Daily: `pg_dump -Fc` to S3.
6. Delete takes a final snapshot, stops the container, frees the port, removes the volume.

## Stack

- Go standard library `net/http` (no framework), pgx, sqlc, goose. Jobs: a `jobs` table with `FOR UPDATE SKIP LOCKED`, not Redis.
- Auth: email and password, argon2id, httpOnly `SameSite=Lax` session cookie. The API is under `/api` on the same host.
- Frontend: SvelteKit 2 (Svelte 5), TypeScript, Tailwind 4, `adapter-static`. No runtime server.
- Control Postgres 17. One container per tenant. Provisioned size, enforced by the volume, not measured.
- Snapshots: `pg_dump -Fc`; restore always creates a new database.

## Commands

```bash
make dev     # control Postgres + api + node + web
make test    # go vet + go test (both modules) + svelte-check
make build   # go build ./control-panel-service/cmd/api ./node-agent-service/cmd/node + npm run build in control-web/
```

## Pipeline contract

Work arrives as a PM-vault task note; the note body is the prompt, and a git-ignored copy sits at the repo root. While working:

- Append to the copy's `## Work Log` as you go.
- Commit with an imperative message on the task branch.
- Push, open a PR (or update the existing one), and put its URL in the copy's `pr_url`.
- Set the copy's `status: done`. `done` with an empty `pr_url` is treated as blocked.
- If you cannot push or open a PR, set `status: blocked` and record the exact error.

Leave `auto_merge` alone; the pipeline sets it, and the merge gate waits for CI. Rebase the task branch onto `main` when resuming after a block; the runner does not reconcile it for you.

## Deploy details

See the deploy contract above and `DEPLOY.md` for the stack map and path filters. A merge is not live until the stack's health check reports the merge commit.

## Conventions

- Each change gets a test; run the checks before committing.
- One home per fact: behavior lives in code, not restated in docs.
- Keep the control and data planes separate; a tenant must not be able to starve the API.
- The tenant volume is fixed-size. Alert at 80% and 90%, and keep a reserve above the plan size: Postgres ENOSPC can force crash recovery.
- Do not add a real payment gateway, PITR, replicas, or multiple nodes without a task.
