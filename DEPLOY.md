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

## Current exceptions

- `connection-router`: there is no `mbr-router` Dokploy app or secret. `deploy-router` in `.github/workflows/deploy.yml` builds `mbr-router:$GITHUB_SHA` on the runner, ships it to KKB with `docker save | gzip` over SSH, and runs it directly with `docker run` — see `connection-router/README.md`. This replaced the `MBR-19` manual `workflow_dispatch`-only gate now that the router is live and cut over to `:5432`.
- `node-agent-service`: no CI-built artifact; the binary is installed and restarted on each data VPS by hand. This is a documented exception, not a precedent for new stacks.
