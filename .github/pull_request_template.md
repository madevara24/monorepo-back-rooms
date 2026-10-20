## Deploy

- [ ] Merging to `main` deploys. I checked the stack's path filter in `.github/workflows/deploy.yml`.
- [ ] New stack: its deploy job, path filter, compose/app, and `DEPLOY.md` row are in this PR.
- [ ] Manual cutover exception: linked follow-up task; `auto_merge` is not true.
- [ ] After merge, the stack health check reports the merged commit.

## Checks

- [ ] `go vet` / `go test` for changed modules
- [ ] `npm run check` / `npm run build` for `control-web`
- [ ] The browser click-through for UI changes
