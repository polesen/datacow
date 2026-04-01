# M8 — HTTP API Server

## Goal
`datacow serve` starts an HTTP server that exposes the core library as a REST API. The web app (M9) will be served from the same binary.

## Depends On
M3 (dataset layer)

## Acceptance Criteria
- [ ] `datacow serve --connection-string=... --port=8080` starts the server
- [ ] Health endpoint: `GET /api/health` → `{"status":"ok"}`
- [ ] Datasources: `GET /api/datasources` → list of configured datasources
- [ ] Datasets: `GET /api/datasources/:id/datasets` → list of datasets (auto-discovered tables + custom)
- [ ] Query: `GET /api/datasources/:id/datasets/:name/rows?page=1&pageSize=50&filter[status]=active&sort=name&dir=asc`
- [ ] Schema: `GET /api/datasources/:id/datasets/:name/schema` → columns + FK info
- [ ] Export: `GET /api/datasources/:id/datasets/:name/export?format=csv` → file download
- [ ] CORS headers set correctly for web app requests
- [ ] Errors return JSON: `{"error": "message"}`
- [ ] Request logging (method, path, duration, status)

## Notes
- Use `github.com/go-chi/chi/v5` for routing — lightweight, idiomatic
- No auth in this milestone — auth is a later milestone
- The `--connection-string` flag registers a single datasource with id `default`
- Future: multiple datasources via `datasources.yaml`
- All handler logic delegates to core — handlers should be thin

## Verify
```bash
go run ./cmd serve --connection-string="postgres://..." --port=8080

curl http://localhost:8080/api/health
curl http://localhost:8080/api/datasources/default/datasets
curl "http://localhost:8080/api/datasources/default/datasets/orders/rows?page=1&pageSize=10"
```
