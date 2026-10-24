# Deploy map

Merging to `main` deploys. `auto_merge: true` in the task pipeline is production deploy authorization.

| Stack | Deploy target | Auto on `main` | Path filter |
|---|---|---|---|
| `control-panel-service`, `control-web` | Dokploy app `mbr` | yes | `control-panel-service/**`, `control-web/**`, `dokploy-compose.yml` |
| `connection-router` | SSH container deploy of `mbr-router` on KKB (no Dokploy app) | yes | `connection-router/**`, `dokploy-compose.router.yml` |
| `node-agent-service` | Manual binary on each data VPS (systemd) | no; explicit manual exception | n/a; every change includes the rebuild/restart runbook |

## New stack checklist

- Add the stack folder and its own build artifact.
- Add its Dokploy app/compose, or write the manual exception in this file.
- Add a push-to-main deploy job with a path filter for that stack; do not let unrelated changes restart it.
- Add the row above.
- Verify the stack health check reports the merged commit.
- If the first deploy requires a manual cutover, file the auto-deploy follow-up in the same task. `auto_merge` stays false until that follow-up exists.

## Demo mode

The demo reset is opt-in on both planes.

- Set `DEMO_MODE=true` on `mbr-api`, plus `DEMO_ADMIN_EMAIL` and `DEMO_ADMIN_PASSWORD`. The API seeds that one admin on boot and then deletes every other account at each boundary. The password lives in Dokploy, never in the repo.
- Set `AGENT_DEMO_MODE=true` on `mbr-node`. Without it, the agent does not serve `POST /v1/demo/reset`.
- Do not enable either flag on a data-plane host with non-demo tenants. The agent reset removes every container labelled `mbr.tenant=true`.
- The control plane resets on the wall clock at `:00` and `:30` UTC, and catches up on boot if a boundary was missed. A failed data-plane cleanup keeps maintenance on and does not truncate the control rows.
- The demo seed marks 1 GB and 5 GB purchasable and leaves 10 GB and 20 GB visible as sold out. Each account may hold two non-deleted databases.
- Do not back up the demo control database. A restore would undo the reset.
- MBR containers and the router rotate `json-file` logs at `10m x 3`; tenant containers carry the same cap. The agent journal needs a size cap and persistent storage on the data host. There is no central log store for MBR in v1.
- The reviewed `node-agent-service` must be built and restarted on the data host. The old pre-refactor binary predates the bulk reset endpoint.

## Current exceptions

- `connection-router`: there is no `mbr-router` Dokploy app or secret. `deploy-router` in `.github/workflows/deploy.yml` builds `mbr-router:$GITHUB_SHA` on the runner, ships it to KKB with `docker save | gzip` over SSH, and runs it directly with `docker run` — see `connection-router/README.md`. This replaced the `MBR-19` manual `workflow_dispatch`-only gate now that the router is live and cut over to `:5432`.
- `node-agent-service`: no CI-built artifact; the binary is installed and restarted on each data VPS by hand. This is a documented exception, not a precedent for new stacks.
