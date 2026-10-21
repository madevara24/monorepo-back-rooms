# Deploy map

Merging to `main` deploys. `auto_merge: true` in the task pipeline is production deploy authorization.

| Stack | Deploy target | Auto on `main` | Path filter |
|---|---|---|---|
| `control-panel-service`, `control-web` | Dokploy app `mbr` | yes | `control-panel-service/**`, `control-web/**`, `dokploy-compose.yml` |
| `connection-router` | SSH container deploy of `mbr-router` on KKB (no Dokploy app) | yes | `connection-router/**`, `dokploy-compose.router.yml` |
| `node-agent-service` | CI publishes a signed GitHub Release and registers it with the control plane; installing it onto each data VPS (systemd) is still manual until MBR-23's updater ships | release publish only; install is an explicit manual exception | `node-agent-service/**` |
| `agent-release` | Not deployed: a shared Go library (manifest signing/verification) imported by `node-agent-service`'s CI signing step and `control-panel-service`'s release catalog. No service runs it directly | n/a | n/a |

## New stack checklist

- Add the stack folder and its own build artifact.
- Add its Dokploy app/compose, or write the manual exception in this file.
- Add a push-to-main deploy job with a path filter for that stack; do not let unrelated changes restart it.
- Add the row above.
- Verify the stack health check reports the merged commit.
- If the first deploy requires a manual cutover, file the auto-deploy follow-up in the same task. `auto_merge` stays false until that follow-up exists.

## Current exceptions

- `connection-router`: there is no `mbr-router` Dokploy app or secret. `deploy-router` in `.github/workflows/deploy.yml` builds `mbr-router:$GITHUB_SHA` on the runner, ships it to KKB with `docker save | gzip` over SSH, and runs it directly with `docker run` — see `connection-router/README.md`. This replaced the `MBR-19` manual `workflow_dispatch`-only gate now that the router is live and cut over to `:5432`.
- `node-agent-service`: CI builds `mbr-node` once per push to main (job `release-node` in `.github/workflows/deploy.yml`), signs the manifest with the ed25519 private key in the `MBR_NODE_RELEASE_SIGNING_KEY` repo secret, publishes a GitHub Release, and registers the release with the control plane's `POST /api/releases` (the manifest's own signature is the only auth that endpoint checks). The DB VPS never compiles. Installing a published release onto a data VPS and restarting `mbr-node` is still a manual step — that automation is MBR-23 (blocked by this task). **MBR_NODE_RELEASE_SIGNING_KEY is not yet set as a repo secret**: until it is, `release-node` fails fast with an explicit error naming the secret, rather than silently skipping signing or inventing a throwaway key. To turn this on: generate the real keypair with `go run ./agent-release/cmd/genkey`, add the printed private key as the `MBR_NODE_RELEASE_SIGNING_KEY` GitHub Actions repo secret, and set the printed public key as `MBR_RELEASE_PUBLIC_KEY_B64` (control plane env) and `AGENT_RELEASE_PUBLIC_KEY_B64` (every node-agent-service host's env).
