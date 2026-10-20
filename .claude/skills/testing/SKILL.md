---
name: testing
description: Run the repo's Go and frontend checks before committing or opening a PR. Use when verifying a change.
---
# Testing

```bash
make test    # go vet + go test (both modules) + svelte-check
make build   # go build ./control-panel-service/cmd/api ./node-agent-service/cmd/node + npm run build in control-web/
```

Individual layers:

```bash
go test ./control-panel-service/... ./node-agent-service/...
cd control-panel-service && golangci-lint run
cd node-agent-service && golangci-lint run
cd control-web && npm run check && npm run build
```

- Read the output: every package must pass; a skipped test is not a pass.
- New behavior gets a test. Test observable behavior and boundaries, not wiring or copies.
- A failure outside the task's scope: record it in the Work Log and stop; don't fix unrelated code.
