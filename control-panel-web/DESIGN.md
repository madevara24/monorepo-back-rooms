# Night Signal — `control-web` design

A calm infrastructure console, not a fake terminal. Dark default, a working light mode, and real state instead of decoration.

## Dials

| Dial | Value | Meaning |
|---|---|---|
| ENERGY | 2 | Quiet surfaces, one accent color, no gradients or glow. |
| RHYTHM | 2 | Regular grid, dense tables, predictable spacing scale. |
| MOTION | 1 | Hover/focus transitions only. No loops, no auto-playing anything. |

## Tokens

Defined as CSS custom properties on `:root` (dark, the default) and overridden under `[data-theme='light']`, then exposed to Tailwind v4 via `@theme inline` in `src/routes/layout.css`.

| Role | Dark | Light |
|---|---|---|
| `bg` | `#0E1116` | `#F3F0E8` |
| `surface` | `#151B22` | `#FAF7EF` |
| `text` | `#E6E8E3` | `#171A1F` |
| `muted` | `#A3A9A6` | `#5A6168` |
| `line` | `#29313A` | `#D8D3C7` |
| `accent` | `#7DD3B0` | `#0B6B55` |
| `on-accent` | `#0E1116` | `#F3F0E8` |
| `warning` | `#F0C674` | `#8A5A00` |
| `error` | `#F2A6A0` | `#A3261B` |

Theme is chosen by `[data-theme="dark"|"light"]` on `<html>`. Default is dark; `prefers-color-scheme` is read once, on first load with no stored preference, to pick the initial value — never consulted again once a user has toggled (the toggle's choice is persisted to `localStorage` under `mbr-theme`). An inline script in `app.html` sets the attribute before first paint, so there is no flash of the wrong theme.

## Type

- **Archivo** (`@fontsource/archivo`) for all UI text and display: headings, body, labels, buttons.
- **IBM Plex Mono** (`@fontsource/ibm-plex-mono`) only for machine values: host, port, database name, user, connection strings, JDBC URLs, node/job IDs. Never as decoration or for ordinary body copy.

## Identity motif: the status rail

A slim horizontal rail, not a hero graphic. It states real facts as text plus a marker glyph (`●` filled for reachable/good, `○` outline for unreachable/unknown, never animated or glowing) followed by a mono value:

```
● API reachable    12 databases    340 GB provisioned    api.mbr.madevara.com
```

- On the signed-out home page, only what is publicly knowable is shown: API reachability (`/healthz`) and the API's mono endpoint. No invented counts.
- Signed in, the rail (home, and the slim strip on dashboard/admin) shows real counts scoped to what that view already loads: a user's own database count and provisioned GB on dashboard; the instance-wide totals already present in the admin overview on admin. No new endpoints are added to produce these numbers.

## Components (`src/lib/components/`)

All local, Svelte 5 runes, styled with the Tailwind theme tokens above — no new UI framework.

- `Button.svelte` — `variant`: `primary | secondary | danger | ghost`; `size`: `sm | md`. 44px min height at `md`.
- `Input.svelte` — single-line text/number/password/email input, token-styled border/focus ring. `mono` prop switches the value font for machine fields.
- `Field.svelte` — label + input/select slot + optional hint/error line. Owns the `<label>`/`for` wiring.
- `Status.svelte` — `state` prop (`running | provisioning | failed | deleting | queued | open | paid | reachable | unreachable | fresh | stale`, etc.) → marker + label text, colored by token (accent/warning/error/muted). Never a pulsing dot.
- `Table.svelte` — thin wrapper that applies the dense-table treatment (compact row height, `line`-colored dividers, sticky header on scroll) to a slotted `<table>`.
- `Panel.svelte` — bordered surface block with optional title, used for page sections and the connection panel.
- `Dialog.svelte` — native `<dialog>`-backed modal for confirmations (replaces `window.confirm`), focus-trapped, closes on `Escape` and backdrop click.
- `CopyField.svelte` — label + readonly mono value + copy button + inline "Copied" status; used for every machine value shown once (password, connection string, JDBC URL, etc.).
- `EmptyState.svelte` — centered muted message for zero-row tables.
- `Toast.svelte` (+ `src/lib/toast.svelte.ts` store) — transient top-level notifications for action results (delete/retry/pay failures and successes).
- `StatusRail.svelte` — the identity motif described above; `dense` prop renders the slim app-shell strip variant.
- `ThemeToggle.svelte` — persisted dark/light switch, keyboard operable.
- `TopBar.svelte` — brand, nav links, theme toggle, user menu; collapses to a mobile menu under the `sm` breakpoint.

## Pages

- **Home** (`/`) — status rail, the real one-sentence proposition (self-serve Postgres, flat IDR/GB price), a database card labeled "Sample" (not live data), the IDR 10,000/GB/month price table, and the five-step lifecycle from `AGENTS.md` ("One database's life"). No fake terminal, stats, testimonials, gradients, glass, or decorative grid.
- **Dashboard** (`/dashboard`) — `TopBar` + status strip, create form, database ledger (dense `Table`, the database is the row), one-time connection `Panel` with Host/Port/Database/User/Password/connection string/JDBC URL as `CopyField`s and a `Status` for provisioning/running/failed. Existing polling and running-state behavior is preserved as-is.
- **Admin** (`/admin`) — `TopBar` + status strip, dense tables for overview, nodes, databases, users, jobs; create-for-user form; force-delete and retry via `Dialog` confirmation. No invented metrics — every number shown already comes from `/api/admin/*`.
- **Billing** (`/billing`) — invoice ledger `Table` with the mock-pay action, real `/api/billing/invoices` data.
- **Login / signup** — unchanged behavior, restyled with `Field`/`Input`/`Button`.

## Responsive & accessibility

- No horizontal overflow at any width; tables scroll within their own `Panel`, not the page.
- 44px minimum touch targets on interactive controls.
- Visible focus ring (`outline` using the `accent` token) on every focusable element, keyboard-reachable mobile menu and dialogs.
- AA contrast checked for both themes at the token level (text/bg and accent/on-accent pairs above 4.5:1).
- Verified at 200% browser zoom: no clipped controls, no overlap.
