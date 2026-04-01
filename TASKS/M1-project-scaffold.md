# M1 — Project Scaffold

## Goal
Set up the Go project structure so all future milestones have a clean foundation to build on.

## Acceptance Criteria
- [ ] `go mod init` with module name `github.com/beetio/datacow`
- [ ] Directory structure matches the layout in `CLAUDE.md`
- [ ] `cmd/main.go` builds and prints version when run with `--version`
- [ ] `Makefile` with targets: `build`, `test`, `run`, `lint`
- [ ] `.gitignore` appropriate for Go projects
- [ ] `docker-compose.dev.yml` in root for spinning up Postgres + MySQL test databases
- [ ] `go test ./...` passes (no tests yet, but must not error)
- [ ] `golangci-lint` runs clean (add `.golangci.yml` config)

## What to Create

```
go.mod
go.sum
Makefile
.gitignore
.golangci.yml
docker-compose.dev.yml
cmd/
  main.go
internal/
  core/
    db/
      client.go       ← just the interface, no implementation yet
    schema/
      .gitkeep
    dataset/
      .gitkeep
    export/
      .gitkeep
    config/
      .gitkeep
  tui/
    .gitkeep
  api/
    .gitkeep
  web/
    .gitkeep
```

## Notes
- Use Cobra for CLI in `cmd/main.go`. Wire up `--version` and `--connection-string` flags even if they do nothing yet.
- `core/db/client.go` should define the `Client` interface with placeholder methods — see M2 for the full interface.
- Module name: `github.com/beetio/datacow`

## Verify
```bash
go build -o datacow ./cmd && ./datacow --version
go test ./...
make lint
```
