# connection-router

A protocol-aware TCP router: it reads just enough of a client's TLS or
Postgres startup handshake to pick a tenant by SNI, then forwards every
other byte between the client and that tenant. It never terminates
TLS, never holds a certificate or key, and never sees a password: the
tenant's own Postgres still presents its wildcard certificate and
enforces `hostssl`.

It replaces nginx's `stream { ssl_preread ... }` on the shared `:5432`.
nginx's `ssl_preread` only works when a client's very first bytes are a
TLS ClientHello (`sslnegotiation=direct`); a client using Postgres's
normal negotiation sends a plaintext `SSLRequest` first, which
`ssl_preread` can't parse, so nginx falls through to its deny default
and the client gets `EOF`. This router understands `SSLRequest` (and
`GSSENCRequest`, `CancelRequest`, and a plaintext `StartupMessage`)
well enough to route the common case too.

See `AGENTS.md` at the repo root for where this fits in MBR's overall
architecture, and `Projects/MBR/Connection Routing Design.md` in the
vault for the design rationale (this is "design C").

## Handshake

The router reads a client's first bytes and branches:

| First bytes | Meaning | Router behavior |
|---|---|---|
| `0x16` | TLS ClientHello (`sslnegotiation=direct`) | Parse SNI, route, forward the ClientHello to the tenant as-is (the tenant also expects direct negotiation), splice. |
| length=8, code=`80877103` | `SSLRequest` | Reply `S`, read the ClientHello, parse SNI, route, dial the tenant, send the tenant its own 8-byte `SSLRequest`, require `S` back, forward the buffered ClientHello, splice. |
| length=8, code=`80877104` | `GSSENCRequest` | Reply `N` (GSS encryption is never offered) and loop to read the client's next message. |
| length=16, code=`80877102` | `CancelRequest` | Close. **Unsupported, deliberately**: a cancel request carries no SNI and arrives on a brand-new connection, so the router has no way to know which tenant it's for. A client's cancel attempt fails fast instead of hanging; the query it meant to cancel just runs to completion. Routing cancellation is out of scope for this router. |
| plausible StartupMessage protocol version | plaintext `StartupMessage` | Send a Postgres `ErrorResponse` (`FATAL`, `08P01`) and close, so the client prints a real error instead of just seeing the connection drop. |
| anything else | garbage | Close, no reply. |

SNI is read from the ClientHello's `server_name` extension (extension
type 0, `host_name` entries only), tolerating a ClientHello fragmented
across multiple TLS records, capped at 16 KiB total, under a 10s
handshake timeout. The hostname is lowercased and a trailing dot is
stripped, then matched against the route map **exactly** — no
wildcard or suffix matching. A missing SNI or an unknown host closes
the connection with no reply, same as any other case the router
won't route.

Once routed, both directions are spliced with `io.Copy` and
half-closed independently; dial timeout is 5s, idle timeout (no bytes
either direction) is 1h, and the router caps itself at 4096 concurrent
connections (further connections are accepted and immediately closed).

## Route map

The router reads the same file control-panel-service's reconciler
already writes (`control-panel-service/internal/usecase.Render`):
`/etc/mbr/routes/mbr-routes.map`, one route per line,
`<public hostname> <tenant host>:<port>;`. It watches the file with
`fsnotify` (on the directory, since the reconciler's `WriteAtomic`
writes via temp-file-then-rename, which the directory sees as a
rename/create, not a write on the file itself) and swaps in a new,
immutable map on every change. A parse error on reload is logged and
the previous good map is kept — the router never serves a partially
applied map. `SIGHUP` also forces a reload, in case a bind mount's
change events are ever lost across a remount.

## Config

| Env var | Default | Purpose |
|---|---|---|
| `ROUTER_ADDR` | `:15432` | The Postgres-facing data path inside the container. |
| `ROUTER_HEALTH_ADDR` | `:9099` | `GET /healthz`'s bind address inside the container. Bind to all interfaces here — Docker's port publishing routes to a container's external interface, not its loopback, so binding this to `127.0.0.1` would make a published port unreachable. Loopback-only exposure is enforced by the **host-side** publish directive in `dokploy-compose.router.yml` (`127.0.0.1:9099:9099`), not by this bind address. |
| `MBR_ROUTES_PATH` | `/etc/mbr/routes/mbr-routes.map` | The route map to read and watch. |

`GET /healthz` reports `{"status","commit","routes_loaded","negotiations"}`,
where `negotiations` is a live breakdown of every handshake outcome
(`direct_tls`, `ssl_request`, `gssenc_declined`, `cancel_closed`,
`plaintext_rejected`, `unknown_closed`, `missing_sni`, `unknown_host`,
`dial_error`, `active_connections`) — useful for confirming, from the
outside, that clients are actually negotiating the way this router
expects.

## Local dev

`connection-router` is deliberately not part of the root `make dev`:
it routes to real tenant hosts by SNI, which local dev has none of,
and its default route map path is production-only and
permission-restricted. To run it standalone against a fake backend:

```bash
mkdir -p /tmp/mbr-routes
echo 'example.db.mbr.madevara.com 127.0.0.1:5555;' > /tmp/mbr-routes/mbr-routes.map
MBR_ROUTES_PATH=/tmp/mbr-routes/mbr-routes.map ROUTER_ADDR=:15432 ROUTER_HEALTH_ADDR=:9099 \
  go run ./cmd/router
```

`go vet ./...` and `go test ./...` need no external services: every
test either runs in-memory or spins up its own `net.Listener` as a
fake backend.

## Production

`connection-router` deploys on the same VPS as
`control-panel-service`/`control-web`, but it is **not** a Dokploy app:
it's a plain `mbr-router` Docker container, run and restarted directly
over SSH, specifically so a routine control-plane deploy never restarts
the live `:5432` data path. It bind-mounts
`/etc/mbr/routes:/etc/mbr/routes:ro` — read-only; it is never the
writer — and runs as `65532:65532` (distroless's `nonroot`, baked into
`connection-router/Dockerfile`), matching the map directory's existing
owner. No new host user, and no change to who writes the map.

Deploying it is `.github/workflows/deploy.yml`'s `deploy-router` job,
gated by a `changes` job (`dorny/paths-filter` over
`connection-router/**` and `dokploy-compose.router.yml`) so it only runs
on a router-touching push to `main`, or any manual `workflow_dispatch`
(which forces it regardless of path). The job:

1. Builds `mbr-router:$GITHUB_SHA` on the GitHub runner from
   `connection-router/Dockerfile`.
2. Streams it to the VPS with `docker save mbr-router:$GITHUB_SHA |
   gzip | ssh ... docker load` — there's no registry in between.
3. Replaces the running container: `docker rm -f mbr-router`, then
   `docker run -d --name mbr-router --restart unless-stopped -p
   5432:15432 -p 127.0.0.1:9099:9099 -v
   /etc/mbr/routes:/etc/mbr/routes:ro mbr-router:$GITHUB_SHA`.
4. Polls `http://127.0.0.1:9099/healthz` over SSH until `commit`
   matches `$GITHUB_SHA`.

`dokploy-compose.router.yml` still documents the same container's
image build and run shape (ports, mount, restart policy) for anyone
reproducing the deploy by hand; the workflow does not call Dokploy for
this stack. See the migration runbook below for how this container was
first cut over from nginx.

### Secrets

The router's deploy reuses the secrets `control-panel-service`'s
deploy already uses: `VPS_SSH_KEY`, `VPS_KNOWN_HOST`, `VPS_USER`,
`VPS_HOST`. It needs no Dokploy app, API key, or compose-id secret of
its own.

## Migration runbook

This router replaces nginx's `stream { ssl_preread ... }` on `:5432`
without a flag day: it runs side by side with nginx on a different
port first, gets verified, and only then takes over `:5432`.

1. **Stand up on `:5433` first.** Build and run the `mbr-router`
   container (per `dokploy-compose.router.yml`'s build/run shape) with
   its port mapping temporarily changed to `"5433:15432"` (instead of
   `"5432:15432"`), using the live route map. Leave nginx and the real
   `:5432` untouched.
2. **Verify against `:5433`:**
   - Standard `psql` with no `sslnegotiation` parameter — this is the
     case nginx's `ssl_preread` could never handle, and the reason
     this router exists.
   - The old `psql ... sslnegotiation=direct` form — must still work,
     unchanged.
   - `openssl s_client -connect <host>:5433 -starttls postgres
     -servername <tenant host>` — confirms the full TLS handshake
     completes and returns the tenant's real certificate.
   - Raw `SSLRequest` and `GSSENCRequest` probes (e.g. a short Python
     or `nc` script sending the 8-byte messages directly) — confirms
     the single-byte `S`/`N` replies.
   - A default DBeaver connection (DBeaver's pgJDBC default is
     `SSLRequest`-based, equivalent to `sslmode=require`).
   - Unknown hosts, a plaintext connection, and a ClientHello with no
     SNI should all be refused (closed, or an `ErrorResponse` for the
     plaintext case) — never silently misrouted.
   - `GET /healthz`'s `negotiations` counters should move as each of
     the above runs.
3. **Cut `:5432` over.** Stop nginx's `stream` systemd unit (leave its
   config and unit **installed** — don't remove them, that's the
   rollback path), then point the container's port mapping at
   `"5432:15432"` and re-run it (`docker rm -f mbr-router` + `docker
   run ...`, as `.github/workflows/deploy.yml`'s `deploy-router` job
   does) so Docker's port publishing binds host `:5432`. Keep nginx
   disabled, not uninstalled, for at least a week.
4. **Deploy the client string change** (dropping
   `sslnegotiation=direct` / `sslNegotiation=direct` from
   `ConnectionString` / `JDBCConnectionString` — this is a normal
   `control-panel-service` deploy, unrelated to the router's own
   deploy). Old connection strings that still carry the direct-mode
   parameter keep working: the router's `0x16`-first-byte path handles
   them exactly as before.
5. **Confirm refusal behavior in production**: an unknown host, a
   plaintext connection, and a missing-SNI ClientHello should all be
   refused against the live `:5432`, and `GET /healthz`'s
   `negotiations` counters should reflect real traffic.

### Rollback

nginx's `stream` config and systemd unit stay installed specifically
for this: if the router needs to come back out, stop the `mbr-router`
container (`docker stop mbr-router` or `docker rm -f mbr-router`) and
re-enable + start nginx's `stream` unit. Because nginx's `ssl_preread`
setup never understood
indirect negotiation, rolling back also means clients need
`sslnegotiation=direct` again — if the client string change (step 4
above) has already shipped, that specific rollback combination breaks
non-direct clients until the router comes back. Prefer rolling back
before shipping step 4, or re-adding `sslnegotiation=direct` to the
connection strings as part of the rollback.

## Out of scope

Connection pooling, TLS termination, routing `CancelRequest` to the
right tenant, any change to `node-agent-service`, and any change to
the `:5432` contract itself (one shared port, routed by SNI) are all
out of scope for this router.
