# MBR — monorepo-back-rooms

A self-serve Postgres marketplace. A user picks a database name and a size in GB, and receives a running Postgres instance with a connection string. Flat price: IDR 10,000 per GB per month, provisioned size, billed hourly for proration. One seller, payments mocked, portfolio piece.

The full design and the reasoning behind each choice live in the vault note `Projects/MBR/`. This file is the repo's working summary.

## Architecture

Two planes, never on the same host.

| Plane | What runs there |
|---|---|
| Control | `cmd/api` (Go), `web/` (SvelteKit static), control Postgres 17. Existing VPS, Dokploy, `mbr.madevara.com`. |
| Data | `cmd/node` (`mbr-node`), Docker, one Postgres container per tenant. Separate VPS. |
| Storage | S3 bucket for snapshots, written by the agent. |

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
make test    # go vet + go test + svelte-check
make build   # go build ./cmd/api ./cmd/node + npm run build in web/
```

The scaffold task creates these targets. Until it lands, the repo is only this tooling.

## Pipeline contract

Work arrives as a PM-vault task note; the note body is the prompt, and a git-ignored copy sits at the repo root. While working:

- Append to the copy's `## Work Log` as you go.
- Commit with an imperative message on the task branch.
- Push, open a PR (or update the existing one), and put its URL in the copy's `pr_url`.
- Set the copy's `status: done`. `done` with an empty `pr_url` is treated as blocked.
- If you cannot push or open a PR, set `status: blocked` and record the exact error.

Leave `auto_merge` alone; the pipeline sets it, and the merge gate waits for CI. Rebase the task branch onto `main` when resuming after a block; the runner does not reconcile it for you.

## Deploy

`main` is the deploy branch. The deploy workflow is the production path; after MBR-15 it triggers on push to `main`. A merge is not live until `/healthz` reports the merge commit. `auto_merge: true` in the task pipeline is production deploy authorization, not just merge permission. If the deploy trigger is missing, fixing it is part of the task, not an ops afterthought.

## Conventions

- Each change gets a test; run the checks before committing.
- One home per fact: behavior lives in code, not restated in docs.
- Keep the control and data planes separate; a tenant must not be able to starve the API.
- The tenant volume is fixed-size. Alert at 80% and 90%, and keep a reserve above the plan size: Postgres ENOSPC can force crash recovery.
- Do not add a real payment gateway, PITR, replicas, or multiple nodes without a task.
