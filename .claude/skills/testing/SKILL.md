---
name: testing
description: Run the repo's Go and frontend checks before committing or opening a PR. Use when verifying a change.
---
# Testing

```bash
make test    # go vet + go test + svelte-check
make build   # go build ./cmd/api ./cmd/node + npm run build in web/
```

Individual layers:

```bash
go test ./...
golangci-lint run
cd web && npm run check && npm run build
```

- Read the output: every package must pass; a skipped test is not a pass.
- New behavior gets a test. Test observable behavior and boundaries, not wiring or copies.
- A failure outside the task's scope: record it in the Work Log and stop; don't fix unrelated code.
