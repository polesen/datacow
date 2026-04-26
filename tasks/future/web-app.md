# M9 — Web App

## Goal
A clean web app served from the same binary as the HTTP API. Implements the same table browsing, filtering, sorting, export, and FK drill-down as the TUI, but in a browser.

## Depends On
M8 (HTTP API)

## Decide First
Before starting this milestone, choose the frontend framework and document it in `CLAUDE.md`.

**Recommendation:** Svelte + Vite
- Lightweight bundle, fast HMR
- Simple component model, no virtual DOM overhead
- Vite dev server proxies API calls to Go server during development
- `go:embed` for production build

**Dev iteration cycle:**
1. `make dev-api` — starts Go API server on port 8080
2. `make dev-web` — starts Vite dev server on port 5173, proxies `/api/*` to 8080
3. Edit `.svelte` file → save → browser hot-reloads instantly
4. `make build` — runs `npm run build`, then `go build` (embeds `web/dist/` into binary)

Switch between embedded and dev mode via `DATACOW_DEV=true` env var:
- Dev mode: Go serves from `web/dist/` on disk (or proxies to Vite)
- Production: Go serves from `go:embed` assets

## Acceptance Criteria
- [ ] Web app project set up in `/web/` (Svelte + Vite)
- [ ] `Makefile` targets: `dev-web`, `build-web`
- [ ] Go embeds `web/dist/` via `go:embed`, served at `/` by the HTTP server
- [ ] Dev mode serves from disk / proxies to Vite
- [ ] Pages:
  - Dataset list (tables, recently used shown first)
  - Row browser (paged table, sort, filter)
  - FK drill-down (same append-below model as TUI)
  - Export button (CSV / Excel download)
- [ ] Responsive enough for laptop screens (not mobile-first)
- [ ] Clean, minimal design — functional over decorative

## Notes
- The web app is a consumer of the M8 API — all data comes via `fetch('/api/...')`
- No separate auth yet — that comes in a later milestone
- Design direction: functional, dense, keyboard-friendly (similar feel to the TUI)

## Verify
```bash
make dev-api   # terminal 1
make dev-web   # terminal 2
# Open http://localhost:5173
# Browse tables, filter, drill down via FK, export CSV
```
