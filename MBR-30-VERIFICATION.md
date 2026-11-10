# MBR-30 final integration verification

Date: 2026-11-10 (Asia/Jakarta)

This unit changes no product code: it re-runs the rename checks and the
repo-wide build/test/docker verification. The command transcripts below are
the evidence for that run.

The old-name literals are omitted from this file so the file does not match
its own exception-filtered sweep. The runner's regex was used unchanged.

## Results

| Check | Result |
| --- | --- |
| Repo-wide old-name sweep, exception-filtered | PASS, 0 unexpected matches |
| DEPLOY.md exception count | PASS, exactly 1 |
| `make test` | PASS, exit 0 (temporary local port override, see below) |
| `go vet` / `go test` (runner command) | PASS, exit 0 |
| `npm run check` | PASS, 0 errors and 0 warnings |
| `make build` | PASS, exit 0 |
| Docker build: `mbr-30-router` | PASS, then removed |
| Docker build: `mbr-30-web` | PASS, then removed |
| Docker build: `mbr-30-api` | PASS, then removed |
| `dokploy-compose.yml` config | PASS, exit 0 |
| `dokploy-compose.router.yml` config | PASS, exit 0 |
| Post-cleanup containers matching `mbr-30` | 0 |
| Post-cleanup images matching `mbr-30-*` | 0 |

## Old-name sweep

```console
$ bad=$(git grep -nE "$OLD_NAME_REGEX" | grep -v '^DEPLOY.md:' | grep -v 'docker rm -f <old-router> <new-router>' || true)
$ if [ -z "$bad" ]; then echo 'unexpected matches: 0'; else printf '%s\n' "$bad"; fi
unexpected matches: 0
$ git grep -nE "$OLD_NAME_REGEX" DEPLOY.md | wc -l
1
```

Post-cleanup state, after all test containers and images were removed:

```console
$ docker ps -a --filter name=mbr-30 --format '{{.ID}} {{.Names}} {{.Image}}'
$ docker images --filter reference='mbr-30-*' --format '{{.Repository}}:{{.Tag}} {{.ID}}'
```

Both commands printed no rows.

## make test

`make test` starts a compose Postgres on host port 5432. On this host the live
router container owns 5432, so for this verification run only the compose host
port and test DSN were moved to 15433, and the compose project was named
`mbr-30-maketest`. Both files were restored immediately afterwards. The
container, network, and volume were removed with `docker compose ... down -v`.

Temporary diff:

```diff
 compose.yaml
-      - "5432:5432"
+      - "15433:5432"

 Makefile
-       DATABASE_URL_TEST=postgres://mbr:mbr@localhost:5432/mbr?sslmode=disable go test ./control-panel-service/... ./node-agent-service/... ./router-service/...
+       DATABASE_URL_TEST=postgres://mbr:mbr@localhost:15433/mbr?sslmode=disable go test ./control-panel-service/... ./node-agent-service/... ./router-service/...
```

Command and output:

```console
$ COMPOSE_PROJECT_NAME=mbr-30-maketest make test
docker compose up -d control-db
 Network mbr-30-maketest_default Creating 
 Network mbr-30-maketest_default Creating 
 Volume mbr-30-maketest_control-db-data Creating 
 Volume mbr-30-maketest_control-db-data Creating 
 Volume mbr-30-maketest_control-db-data Created 
 Volume mbr-30-maketest_control-db-data Created 
 Network mbr-30-maketest_default Created 
 Network mbr-30-maketest_default Created 
 Container mbr-30-maketest-control-db-1 Creating 
 Container mbr-30-maketest-control-db-1 Created 
 Container mbr-30-maketest-control-db-1 Starting 
 Container mbr-30-maketest-control-db-1 Started 
go vet ./control-panel-service/... ./node-agent-service/... ./router-service/...
DATABASE_URL_TEST=postgres://mbr:mbr@localhost:15433/mbr?sslmode=disable go test ./control-panel-service/... ./node-agent-service/... ./router-service/...
?   	github.com/madevara24/monorepo-back-rooms/control-panel-service/cmd/api	[no test files]
?   	github.com/madevara24/monorepo-back-rooms/control-panel-service/config	[no test files]
?   	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal	[no test files]
?   	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities	[no test files]
ok  	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/handler	87.026s
ok  	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/integration	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/pkg/credentials	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/pkg/password	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/pkg/session	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository	23.660s
?   	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository/storetest	[no test files]
ok  	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/usecase	16.760s
?   	github.com/madevara24/monorepo-back-rooms/control-panel-service/migrations	[no test files]
?   	github.com/madevara24/monorepo-back-rooms/node-agent-service/cmd/node	[no test files]
ok  	github.com/madevara24/monorepo-back-rooms/node-agent-service/config	(cached)
?   	github.com/madevara24/monorepo-back-rooms/node-agent-service/internal	[no test files]
?   	github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/entities	[no test files]
ok  	github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/handler	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/repository	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/usecase	(cached)
?   	github.com/madevara24/monorepo-back-rooms/router-service/cmd/router	[no test files]
?   	github.com/madevara24/monorepo-back-rooms/router-service/config	[no test files]
?   	github.com/madevara24/monorepo-back-rooms/router-service/internal	[no test files]
ok  	github.com/madevara24/monorepo-back-rooms/router-service/internal/handler	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/router-service/internal/pgwire	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/router-service/internal/proxy	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/router-service/internal/routemap	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/router-service/internal/sni	(cached)
cd control-panel-web && npm run check

> control-panel-web@0.0.1 check
> svelte-kit sync && svelte-check --tsconfig ./tsconfig.json

Loading svelte-check in workspace: /home/obsidian/repos/monorepo-back-rooms/control-panel-web
Getting Svelte diagnostics...

svelte-check found 0 errors and 0 warnings
make test exit: 0
```

The `mbr-30-maketest-control-db-1` container, its network, and its volume were
then removed. `make test` in the committed tree still expects host port 5432;
this host cannot provide it while the production router is running. The
runner's verify command uses the direct `go vet`/`go test` commands instead,
which are included below.

## Runner verify command

The runner's verify command was executed verbatim. Full output:

```console
?   	github.com/madevara24/monorepo-back-rooms/control-panel-service/cmd/api	[no test files]
?   	github.com/madevara24/monorepo-back-rooms/control-panel-service/config	[no test files]
?   	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal	[no test files]
?   	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities	[no test files]
ok  	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/handler	0.163s
ok  	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/integration	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/pkg/credentials	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/pkg/password	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/pkg/session	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository	0.219s
?   	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository/storetest	[no test files]
ok  	github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/usecase	0.070s
?   	github.com/madevara24/monorepo-back-rooms/control-panel-service/migrations	[no test files]
?   	github.com/madevara24/monorepo-back-rooms/node-agent-service/cmd/node	[no test files]
ok  	github.com/madevara24/monorepo-back-rooms/node-agent-service/config	(cached)
?   	github.com/madevara24/monorepo-back-rooms/node-agent-service/internal	[no test files]
?   	github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/entities	[no test files]
ok  	github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/handler	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/repository	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/usecase	(cached)
?   	github.com/madevara24/monorepo-back-rooms/router-service/cmd/router	[no test files]
?   	github.com/madevara24/monorepo-back-rooms/router-service/config	[no test files]
?   	github.com/madevara24/monorepo-back-rooms/router-service/internal	[no test files]
ok  	github.com/madevara24/monorepo-back-rooms/router-service/internal/handler	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/router-service/internal/pgwire	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/router-service/internal/proxy	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/router-service/internal/routemap	(cached)
ok  	github.com/madevara24/monorepo-back-rooms/router-service/internal/sni	(cached)

> control-panel-web@0.0.1 check
> svelte-kit sync && svelte-check --tsconfig ./tsconfig.json

Loading svelte-check in workspace: /home/obsidian/repos/monorepo-back-rooms/control-panel-web
Getting Svelte diagnostics...

svelte-check found 0 errors and 0 warnings
go build -o bin/api ./control-panel-service/cmd/api
go build -o bin/node ./node-agent-service/cmd/node
go build -o bin/router ./router-service/cmd/router
cd control-panel-web && npm run build

> control-panel-web@0.0.1 build
> vite build

vite v6.4.4 building SSR bundle for production...
transforming...
✓ 189 modules transformed.
rendering chunks...
vite v6.4.4 building for production...
transforming...
✓ 211 modules transformed.
rendering chunks...
computing gzip size...
.svelte-kit/output/client/_app/version.json                                                            0.03 kB │ gzip:  0.05 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-vietnamese-600-normal.anCjMumA.woff2           5.19 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-vietnamese-500-normal.C8zxqsMH.woff      5.62 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-vietnamese-400-normal.DDuiU_S-.woff      5.62 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-vietnamese-500-normal.k0drYPZA.woff2           5.66 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-vietnamese-400-normal.BF4g3tCF.woff2           5.68 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-vietnamese-700-normal.nuAQAQa7.woff2           5.69 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-cyrillic-ext-500-normal.BIfNGwUT.woff    5.77 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-cyrillic-ext-400-normal.DMdlQ8Kv.woff    5.82 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-vietnamese-400-normal.BulugwFq.woff2     5.87 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-vietnamese-500-normal.DZ4AoWbu.woff2     6.04 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-cyrillic-ext-400-normal.xuaO2J-f.woff2   6.91 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-cyrillic-ext-500-normal.BqneJy0T.woff2   6.97 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-cyrillic-400-normal.CEL4l2ZJ.woff        7.20 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-cyrillic-500-normal.Ael50iVv.woff        7.21 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-vietnamese-600-normal.C8xBVdfP.woff            7.38 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-vietnamese-500-normal.7W3lD2kQ.woff            7.66 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-vietnamese-700-normal.Bz2vkKqq.woff            7.67 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-vietnamese-400-normal.FGOLYM1a.woff            7.68 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-cyrillic-400-normal.BSMlKf0J.woff2       8.36 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-cyrillic-500-normal.Bq9vWWag.woff2       8.46 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-latin-ext-400-normal.D3D2R8hC.woff      11.77 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-latin-ext-500-normal.CZ70TYgx.woff      11.77 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-latin-ext-600-normal.Dm61iqcg.woff2           12.38 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-latin-ext-400-normal.B5tPZzqW.woff2           12.93 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-latin-400-normal.CvHOgSBP.woff          13.14 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-latin-500-normal.CB9ihrfo.woff          13.16 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-latin-ext-500-normal.RFhg4CpD.woff2           13.16 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-latin-ext-700-normal.DD_nkw6b.woff2           13.27 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-latin-ext-400-normal.BmRBH3aV.woff2     13.35 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-latin-ext-500-normal.CAhNIIs5.woff2     13.43 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-latin-600-normal.3BBy0ZsW.woff2               13.82 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-latin-700-normal.D9cHMWGj.woff2               14.51 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-latin-500-normal.CGugC8dq.woff2               14.60 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-latin-400-normal.C81ewxNO.woff2               14.70 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-latin-400-normal.DMJ8VG8y.woff2         14.71 kB
.svelte-kit/output/client/_app/immutable/assets/ibm-plex-mono-latin-500-normal.DSY6xOcd.woff2         14.89 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-latin-ext-600-normal.D7uErJuz.woff            17.72 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-latin-ext-400-normal.DrmqtLj6.woff            17.98 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-latin-600-normal.DwYieO8P.woff                18.20 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-latin-ext-500-normal.D7mXw3Kz.woff            18.30 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-latin-ext-700-normal.CgyfnKjw.woff            18.40 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-latin-700-normal.lT5TFe7H.woff                18.80 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-latin-400-normal.Bl602Mgc.woff                18.97 kB
.svelte-kit/output/client/_app/immutable/assets/archivo-latin-500-normal.pkzA7jw9.woff                18.99 kB
.svelte-kit/output/client/.vite/manifest.json                                                         29.23 kB │ gzip:  2.88 kB
.svelte-kit/output/client/_app/immutable/assets/EmptyState.BKKCKzyY.css                                0.30 kB │ gzip:  0.18 kB
.svelte-kit/output/client/_app/immutable/assets/0.CgmsPQed.css                                        23.71 kB │ gzip:  5.29 kB
.svelte-kit/output/client/_app/immutable/chunks/DQSor5WS.js                                            0.04 kB │ gzip:  0.06 kB
.svelte-kit/output/client/_app/immutable/chunks/BEPfpWa7.js                                            0.04 kB │ gzip:  0.06 kB
.svelte-kit/output/client/_app/immutable/entry/start.DjM5kFFM.js                                       0.12 kB │ gzip:  0.11 kB
.svelte-kit/output/client/_app/immutable/chunks/CABb__52.js                                            0.19 kB │ gzip:  0.17 kB
.svelte-kit/output/client/_app/immutable/chunks/B0GYmqM2.js                                            0.23 kB │ gzip:  0.16 kB
.svelte-kit/output/client/_app/immutable/chunks/DOqcwCOw.js                                            0.27 kB │ gzip:  0.22 kB
.svelte-kit/output/client/_app/immutable/chunks/Dnc2W2Bs.js                                            0.30 kB │ gzip:  0.23 kB
.svelte-kit/output/client/_app/immutable/chunks/B2dp0cSQ.js                                            0.40 kB │ gzip:  0.30 kB
.svelte-kit/output/client/_app/immutable/chunks/BoSuJXMo.js                                            0.45 kB │ gzip:  0.27 kB
.svelte-kit/output/client/_app/immutable/chunks/CO5-8xXn.js                                            0.47 kB │ gzip:  0.33 kB
.svelte-kit/output/client/_app/immutable/chunks/DoeGbexP.js                                            0.47 kB │ gzip:  0.31 kB
.svelte-kit/output/client/_app/immutable/nodes/1.96OMfdcz.js                                           0.48 kB │ gzip:  0.32 kB
.svelte-kit/output/client/_app/immutable/chunks/C5oq8MkA.js                                            0.51 kB │ gzip:  0.33 kB
.svelte-kit/output/client/_app/immutable/chunks/3Q6qIvMf.js                                            0.60 kB │ gzip:  0.37 kB
.svelte-kit/output/client/_app/immutable/chunks/Da2-dhBp.js                                            0.64 kB │ gzip:  0.39 kB
.svelte-kit/output/client/_app/immutable/chunks/6qC1AO8u.js                                            0.65 kB │ gzip:  0.40 kB
.svelte-kit/output/client/_app/immutable/chunks/B-OI_isb.js                                            0.76 kB │ gzip:  0.43 kB
.svelte-kit/output/client/_app/immutable/nodes/2.lnpMdfNe.js                                           0.77 kB │ gzip:  0.46 kB
.svelte-kit/output/client/_app/immutable/chunks/Dw4XAxH_.js                                            1.01 kB │ gzip:  0.60 kB
.svelte-kit/output/client/_app/immutable/nodes/4.DXFHaFsC.js                                           1.11 kB │ gzip:  0.63 kB
.svelte-kit/output/client/_app/immutable/chunks/Dbe9cOsu.js                                            1.17 kB │ gzip:  0.67 kB
.svelte-kit/output/client/_app/immutable/chunks/bkI0RePC.js                                            1.17 kB │ gzip:  0.63 kB
.svelte-kit/output/client/_app/immutable/chunks/_PTLRBAp.js                                            1.26 kB │ gzip:  0.70 kB
.svelte-kit/output/client/_app/immutable/chunks/u6Z9pJwd.js                                            1.31 kB │ gzip:  0.73 kB
.svelte-kit/output/client/_app/immutable/chunks/BhVHoQlX.js                                            1.33 kB │ gzip:  0.74 kB
.svelte-kit/output/client/_app/immutable/chunks/wbPk3Yxo.js                                            1.37 kB │ gzip:  0.66 kB
.svelte-kit/output/client/_app/immutable/chunks/K4psa0hX.js                                            1.54 kB │ gzip:  0.88 kB
.svelte-kit/output/client/_app/immutable/chunks/CxpSGMYY.js                                            2.24 kB │ gzip:  0.99 kB
.svelte-kit/output/client/_app/immutable/chunks/C2pmke5H.js                                            2.41 kB │ gzip:  1.22 kB
.svelte-kit/output/client/_app/immutable/nodes/12.BQxWppLK.js                                          2.49 kB │ gzip:  1.06 kB
.svelte-kit/output/client/_app/immutable/nodes/5.zrWY--AF.js                                           2.52 kB │ gzip:  1.23 kB
.svelte-kit/output/client/_app/immutable/chunks/CrxtRynJ.js                                            2.54 kB │ gzip:  1.12 kB
.svelte-kit/output/client/_app/immutable/nodes/6.BbztHBv0.js                                           2.55 kB │ gzip:  1.25 kB
.svelte-kit/output/client/_app/immutable/nodes/9.BEFyKlbo.js                                           2.68 kB │ gzip:  1.44 kB
.svelte-kit/output/client/_app/immutable/chunks/DyXaXfGn.js                                            4.50 kB │ gzip:  2.09 kB
.svelte-kit/output/client/_app/immutable/nodes/11.B7rYwmjx.js                                          4.67 kB │ gzip:  2.11 kB
.svelte-kit/output/client/_app/immutable/nodes/7.B29mhqD0.js                                           4.94 kB │ gzip:  2.29 kB
.svelte-kit/output/client/_app/immutable/chunks/C_rMxmmP.js                                            5.01 kB │ gzip:  2.33 kB
.svelte-kit/output/client/_app/immutable/chunks/Bdw0D7HT.js                                            6.51 kB │ gzip:  2.93 kB
.svelte-kit/output/client/_app/immutable/nodes/3.BMGWHmxM.js                                           7.17 kB │ gzip:  2.69 kB
.svelte-kit/output/client/_app/immutable/chunks/BSQoCAVc.js                                            7.61 kB │ gzip:  3.12 kB
.svelte-kit/output/client/_app/immutable/nodes/0.Box6OYL1.js                                           8.52 kB │ gzip:  3.68 kB
.svelte-kit/output/client/_app/immutable/entry/app.CSNdJ6-X.js                                        10.14 kB │ gzip:  3.93 kB
.svelte-kit/output/client/_app/immutable/nodes/10.BlnaBhE1.js                                         10.15 kB │ gzip:  4.37 kB
.svelte-kit/output/client/_app/immutable/nodes/8.JR59MMAX.js                                          19.82 kB │ gzip:  6.75 kB
.svelte-kit/output/client/_app/immutable/chunks/nHnzuCq3.js                                           23.13 kB │ gzip:  8.97 kB
.svelte-kit/output/client/_app/immutable/chunks/D0VY7UT-.js                                           28.67 kB │ gzip: 10.93 kB
✓ built in 4.54s
.svelte-kit/output/server/_app/immutable/assets/archivo-vietnamese-600-normal.anCjMumA.woff2            5.19 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-vietnamese-500-normal.C8zxqsMH.woff       5.62 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-vietnamese-400-normal.DDuiU_S-.woff       5.62 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-vietnamese-500-normal.k0drYPZA.woff2            5.66 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-vietnamese-400-normal.BF4g3tCF.woff2            5.68 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-vietnamese-700-normal.nuAQAQa7.woff2            5.69 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-cyrillic-ext-500-normal.BIfNGwUT.woff     5.77 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-cyrillic-ext-400-normal.DMdlQ8Kv.woff     5.82 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-vietnamese-400-normal.BulugwFq.woff2      5.87 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-vietnamese-500-normal.DZ4AoWbu.woff2      6.04 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-cyrillic-ext-400-normal.xuaO2J-f.woff2    6.91 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-cyrillic-ext-500-normal.BqneJy0T.woff2    6.97 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-cyrillic-400-normal.CEL4l2ZJ.woff         7.20 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-cyrillic-500-normal.Ael50iVv.woff         7.21 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-vietnamese-600-normal.C8xBVdfP.woff             7.38 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-vietnamese-500-normal.7W3lD2kQ.woff             7.66 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-vietnamese-700-normal.Bz2vkKqq.woff             7.67 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-vietnamese-400-normal.FGOLYM1a.woff             7.68 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-cyrillic-400-normal.BSMlKf0J.woff2        8.36 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-cyrillic-500-normal.Bq9vWWag.woff2        8.46 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-latin-ext-500-normal.CZ70TYgx.woff       11.77 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-latin-ext-400-normal.D3D2R8hC.woff       11.77 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-latin-ext-600-normal.Dm61iqcg.woff2            12.38 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-latin-ext-400-normal.B5tPZzqW.woff2            12.93 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-latin-400-normal.CvHOgSBP.woff           13.14 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-latin-500-normal.CB9ihrfo.woff           13.16 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-latin-ext-500-normal.RFhg4CpD.woff2            13.16 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-latin-ext-700-normal.DD_nkw6b.woff2            13.27 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-latin-ext-400-normal.BmRBH3aV.woff2      13.35 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-latin-ext-500-normal.CAhNIIs5.woff2      13.43 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-latin-600-normal.3BBy0ZsW.woff2                13.82 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-latin-700-normal.D9cHMWGj.woff2                14.51 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-latin-500-normal.CGugC8dq.woff2                14.60 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-latin-400-normal.C81ewxNO.woff2                14.70 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-latin-400-normal.DMJ8VG8y.woff2          14.71 kB
.svelte-kit/output/server/_app/immutable/assets/ibm-plex-mono-latin-500-normal.DSY6xOcd.woff2          14.89 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-latin-ext-600-normal.D7uErJuz.woff             17.72 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-latin-ext-400-normal.DrmqtLj6.woff             17.98 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-latin-600-normal.DwYieO8P.woff                 18.20 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-latin-ext-500-normal.D7mXw3Kz.woff             18.30 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-latin-ext-700-normal.CgyfnKjw.woff             18.40 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-latin-700-normal.lT5TFe7H.woff                 18.80 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-latin-400-normal.Bl602Mgc.woff                 18.97 kB
.svelte-kit/output/server/_app/immutable/assets/archivo-latin-500-normal.pkzA7jw9.woff                 18.99 kB
.svelte-kit/output/server/.vite/manifest.json                                                          24.19 kB
.svelte-kit/output/server/_app/immutable/assets/Table.BKKCKzyY.css                                      0.30 kB
.svelte-kit/output/server/_app/immutable/assets/_layout.B0sbU6Iv.css                                   23.60 kB
.svelte-kit/output/server/entries/pages/_layout.ts.js                                                   0.05 kB
.svelte-kit/output/server/chunks/authGuard.js                                                           0.09 kB
.svelte-kit/output/server/chunks/shared-server.js                                                       0.21 kB
.svelte-kit/output/server/env.js                                                                        0.23 kB
.svelte-kit/output/server/chunks/toast.svelte.js                                                        0.38 kB
.svelte-kit/output/server/internal.js                                                                   0.41 kB
.svelte-kit/output/server/chunks/server.js                                                              0.62 kB
.svelte-kit/output/server/chunks/state.svelte.js                                                        0.65 kB
.svelte-kit/output/server/entries/pages/(site)/(app)/billing/_page.svelte.js                            0.67 kB
.svelte-kit/output/server/chunks/stores.js                                                              0.71 kB
.svelte-kit/output/server/entries/fallbacks/error.svelte.js                                             0.78 kB
.svelte-kit/output/server/chunks/session.svelte.js                                                      0.83 kB
.svelte-kit/output/server/entries/pages/(auth)/_layout.svelte.js                                        0.86 kB
.svelte-kit/output/server/chunks/Status.js                                                              1.01 kB
.svelte-kit/output/server/chunks/SkipLink.js                                                            1.02 kB
.svelte-kit/output/server/chunks/utils2.js                                                              1.09 kB
.svelte-kit/output/server/chunks/Button.js                                                              1.12 kB
.svelte-kit/output/server/chunks/context.js                                                             1.38 kB
.svelte-kit/output/server/chunks/Input.js                                                               1.54 kB
.svelte-kit/output/server/entries/pages/(site)/(app)/_layout.svelte.js                                  2.00 kB
.svelte-kit/output/server/entries/pages/(site)/demo/_page.svelte.js                                     2.52 kB
.svelte-kit/output/server/entries/pages/(auth)/login/_page.svelte.js                                    2.66 kB
.svelte-kit/output/server/entries/pages/(auth)/signup/_page.svelte.js                                   2.70 kB
.svelte-kit/output/server/chunks/ConfirmDialog.js                                                       3.02 kB
.svelte-kit/output/server/entries/pages/(site)/_page.svelte.js                                          3.10 kB
.svelte-kit/output/server/entries/pages/(site)/(app)/dashboard/_page.svelte.js                          3.52 kB
.svelte-kit/output/server/chunks/internal.js                                                            3.53 kB
.svelte-kit/output/server/chunks/tour.svelte.js                                                         4.02 kB
.svelte-kit/output/server/entries/pages/(site)/about/_page.svelte.js                                    5.16 kB
.svelte-kit/output/server/chunks/exports.js                                                             5.54 kB
.svelte-kit/output/server/entries/pages/(site)/_layout.svelte.js                                        6.29 kB
.svelte-kit/output/server/entries/pages/_layout.svelte.js                                               7.35 kB
.svelte-kit/output/server/entries/pages/(site)/(app)/admin/_page.svelte.js                             10.27 kB
.svelte-kit/output/server/chunks/root.js                                                               32.84 kB
.svelte-kit/output/server/chunks/utils.js                                                              34.22 kB
.svelte-kit/output/server/remote-entry.js                                                              36.81 kB
.svelte-kit/output/server/chunks/index.js                                                             104.30 kB
.svelte-kit/output/server/index.js                                                                    132.29 kB
✓ built in 15.57s

Run npm run preview to preview your production build locally.

> Using @sveltejs/adapter-static
  Wrote site to "build"
  ✔ done
#0 building with "default" instance using docker driver

#1 [internal] load build definition from Dockerfile
#1 transferring dockerfile: 1.52kB 0.0s done
#1 DONE 0.1s

#2 resolve image config for docker-image://docker.io/docker/dockerfile:1
#2 DONE 1.1s

#3 docker-image://docker.io/docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e
#3 CACHED

#4 [internal] load metadata for docker.io/library/golang:1.26-alpine
#4 ...

#5 [internal] load metadata for gcr.io/distroless/static-debian12:nonroot
#5 DONE 0.9s

#4 [internal] load metadata for docker.io/library/golang:1.26-alpine
#4 DONE 1.0s

#6 [internal] load .dockerignore
#6 transferring context: 133B done
#6 DONE 0.1s

#7 [stage-1 1/2] FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
#7 DONE 0.0s

#8 [build  1/10] FROM docker.io/library/golang:1.26-alpine@sha256:87c65a688975c0b1d4585d204874c45ca78354670e3a5f11368c5a492303d696
#8 DONE 0.0s

#9 [internal] load build context
#9 transferring context: 149.17kB 0.5s done
#9 DONE 0.6s

#10 [build  3/10] WORKDIR /src
#10 CACHED

#11 [build  4/10] COPY go.work go.work.sum ./
#11 CACHED

#12 [build  5/10] COPY router-service/go.mod router-service/go.sum router-service/
#12 CACHED

#13 [build  6/10] COPY control-panel-service/go.mod control-panel-service/go.sum control-panel-service/
#13 CACHED

#14 [build  7/10] COPY node-agent-service/go.mod node-agent-service/go.sum node-agent-service/
#14 CACHED

#15 [build  2/10] RUN apk add --no-cache git ca-certificates
#15 CACHED

#16 [build  8/10] RUN go -C router-service mod download
#16 CACHED

#17 [build  9/10] COPY . .
#17 DONE 1.4s

#18 [build 10/10] RUN commit="$(git rev-parse HEAD 2>/dev/null || echo unknown)" &&     CGO_ENABLED=0 GOOS=linux go -C router-service build -trimpath -ldflags "-s -w -X main.commit=${commit}" -o /out/router ./cmd/router
#18 DONE 51.0s

#19 [stage-1 2/2] COPY --from=build /out/router /router
#19 CACHED

#20 exporting to image
#20 exporting layers done
#20 writing image sha256:31279322ec1eccd206102c736d2aab3d3d79739226433969d3e29fab039a6474 0.0s done
#20 naming to docker.io/library/mbr-30-router
#20 naming to docker.io/library/mbr-30-router 0.2s done
#20 DONE 0.2s
#0 building with "default" instance using docker driver

#1 [internal] load build definition from Dockerfile
#1 transferring dockerfile: 391B done
#1 DONE 0.0s

#2 resolve image config for docker-image://docker.io/docker/dockerfile:1
#2 DONE 0.4s

#3 docker-image://docker.io/docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e
#3 CACHED

#4 [internal] load metadata for docker.io/library/nginx:1.27-alpine
#4 ...

#5 [internal] load metadata for docker.io/library/node:22-alpine
#5 DONE 0.7s

#4 [internal] load metadata for docker.io/library/nginx:1.27-alpine
#4 DONE 0.7s

#6 [internal] load .dockerignore
#6 transferring context: 97B done
#6 DONE 0.0s

#7 [stage-1 1/3] FROM docker.io/library/nginx:1.27-alpine@sha256:65645c7bb6a0661892a8b03b89d0743208a18dd2f3f17a54ef4b76fb8e2f2a10
#7 DONE 0.0s

#8 [build 1/6] FROM docker.io/library/node:22-alpine@sha256:0e1754758c2df4cfa3e68d59e2e1b7374bb7fd9ea6328eaaacf15cc934af8202
#8 DONE 0.0s

#9 [internal] load build context
#9 transferring context: 3.42kB 0.0s done
#9 DONE 0.0s

#10 [build 2/6] WORKDIR /src
#10 CACHED

#11 [build 6/6] RUN npm run build
#11 CACHED

#12 [build 4/6] RUN npm ci
#12 CACHED

#13 [stage-1 2/3] COPY --from=build /src/build /usr/share/nginx/html
#13 CACHED

#14 [build 3/6] COPY package.json package-lock.json ./
#14 CACHED

#15 [build 5/6] COPY . .
#15 CACHED

#16 [stage-1 3/3] COPY nginx.conf /etc/nginx/conf.d/default.conf
#16 CACHED

#17 exporting to image
#17 exporting layers done
#17 writing image sha256:03a199333773df428079508aed3e82ddea467eb4d5fa5f0ea2508b3678d7a3f0 done
#17 naming to docker.io/library/mbr-30-web 0.0s done
#17 DONE 0.0s
#0 building with "default" instance using docker driver

#1 [internal] load build definition from Dockerfile
#1 transferring dockerfile: 1.32kB done
#1 DONE 0.0s

#2 resolve image config for docker-image://docker.io/docker/dockerfile:1
#2 DONE 0.3s

#3 docker-image://docker.io/docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e
#3 CACHED

#4 [internal] load metadata for gcr.io/distroless/static-debian12:nonroot
#4 DONE 0.4s

#5 [internal] load metadata for docker.io/library/golang:1.26-alpine
#5 DONE 0.5s

#6 [internal] load .dockerignore
#6 transferring context: 133B 0.0s done
#6 DONE 0.1s

#7 [build  1/10] FROM docker.io/library/golang:1.26-alpine@sha256:87c65a688975c0b1d4585d204874c45ca78354670e3a5f11368c5a492303d696
#7 DONE 0.0s

#8 [stage-1 1/2] FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
#8 DONE 0.0s

#9 [internal] load build context
#9 transferring context: 123.59kB 0.4s done
#9 DONE 0.4s

#10 [build  3/10] WORKDIR /src
#10 CACHED

#11 [build  4/10] COPY go.work go.work.sum ./
#11 CACHED

#12 [build  5/10] COPY control-panel-service/go.mod control-panel-service/go.sum control-panel-service/
#12 CACHED

#13 [build  6/10] COPY node-agent-service/go.mod node-agent-service/go.sum node-agent-service/
#13 CACHED

#14 [build  7/10] COPY router-service/go.mod router-service/go.sum router-service/
#14 CACHED

#15 [build  2/10] RUN apk add --no-cache git ca-certificates
#15 CACHED

#16 [build  8/10] RUN go -C control-panel-service mod download
#16 CACHED

#17 [build  9/10] COPY . .
#17 DONE 1.0s

#18 [build 10/10] RUN commit="$(git rev-parse HEAD 2>/dev/null || echo unknown)" &&     CGO_ENABLED=0 GOOS=linux go -C control-panel-service build -trimpath -ldflags "-s -w -X main.commit=${commit}" -o /out/api ./cmd/api
#18 DONE 40.3s

#19 [stage-1 2/2] COPY --from=build /out/api /api
#19 CACHED

#20 exporting to image
#20 exporting layers done
#20 writing image sha256:62492744b02164b34df9826cc2f963f53beb8557137536780427c2e2d8019590 done
#20 naming to docker.io/library/mbr-30-api done
#20 DONE 0.0s
time="2026-11-10T09:48:58+07:00" level=warning msg="The \"AGENT_CLIENT_CERT_B64\" variable is not set. Defaulting to a blank string."
time="2026-11-10T09:48:58+07:00" level=warning msg="The \"AGENT_CLIENT_KEY_B64\" variable is not set. Defaulting to a blank string."
time="2026-11-10T09:48:58+07:00" level=warning msg="The \"DATABASE_URL\" variable is not set. Defaulting to a blank string."
time="2026-11-10T09:48:58+07:00" level=warning msg="The \"CREDENTIAL_KEY\" variable is not set. Defaulting to a blank string."
time="2026-11-10T09:48:58+07:00" level=warning msg="The \"AGENT_CA_B64\" variable is not set. Defaulting to a blank string."
time="2026-11-10T09:48:58+07:00" level=warning msg="The \"PUBLIC_DOKPLOY_URL\" variable is not set. Defaulting to a blank string."
time="2026-11-10T09:48:58+07:00" level=warning msg="The \"POSTGRES_PASSWORD\" variable is not set. Defaulting to a blank string."
Untagged: mbr-30-router:latest
Deleted: sha256:31279322ec1eccd206102c736d2aab3d3d79739226433969d3e29fab039a6474
Untagged: mbr-30-web:latest
Deleted: sha256:03a199333773df428079508aed3e82ddea467eb4d5fa5f0ea2508b3678d7a3f0
Untagged: mbr-30-api:latest
Deleted: sha256:62492744b02164b34df9826cc2f963f53beb8557137536780427c2e2d8019590
verify exit: 0
```
