# Contributing to Datacow

Thanks for your interest in Datacow! Contributions are welcome — bug reports,
feature ideas, docs, and code.

> **Note on this project's origins:** Datacow is an experiment in AI-driven
> development — historically every line was produced by Claude under human
> direction. Human-written contributions are very welcome and do not need to
> follow that constraint. PRs are reviewed normally.

## Getting started

```bash
git clone https://github.com/polesen/datacow
cd datacow
go build ./cmd
```

You need **Go 1.25+** (see `go.mod`). Test databases run via Docker Compose:

```bash
docker compose -f docker-compose.dev.yml up -d
```

## Development workflow

1. **Fork** the repo and create a branch off `main`.
2. Make your change. Follow the conventions in [`CLAUDE.md`](CLAUDE.md) — it is
   the source of truth for architecture, layering, naming, and SQL-safety rules.
3. **Write tests first** (this project follows TDD). See the testing-by-layer
   guidance in `CLAUDE.md`.
4. Run the checks below until green.
5. Open a pull request against `main` with a clear description.

## Before you open a PR

```bash
go build ./...
make lint
staticcheck ./...
make test
```

CI runs the same checks against PostgreSQL and MySQL. PRs from forks run CI
without access to repository secrets — that is expected and safe.

## Ground rules

- **SQL safety is non-negotiable.** Always use parameterized queries; never
  interpolate user input, column names, or table names into SQL. Dynamic
  identifiers must be validated against the known schema. See the "SQL Security"
  section of `CLAUDE.md`.
- **No business logic in the TUI or API layers** — it belongs in `internal/core`.
- **New user-visible TUI actions need a keybinding and a help entry.**
- Keep commits focused and write descriptive commit messages.

## Releasing (maintainers)

Releases are cut by pushing a semver tag — everything else is automated.

```bash
git tag v0.1.1
git push origin v0.1.1
```

On a `v*` tag, the [release workflow](.github/workflows/release.yml) automatically:

1. Builds a reproducible source tarball and attests its SLSA build provenance.
2. Publishes a GitHub Release with auto-generated notes.
3. **For stable tags only** — regenerates the Homebrew formula and pushes it to
   [`polesen/homebrew-tap`](https://github.com/polesen/homebrew-tap), so
   `brew install polesen/tap/datacow` picks up the new version.

Prereleases (e.g. `v0.2.0-rc1`) publish a Release but **skip** the tap bump, so
they never reach `brew` users — handy for a dry run.

Notes:

- The formula is rendered by [`scripts/gen-formula.sh`](scripts/gen-formula.sh).
  To change how datacow is packaged, edit that script — never hand-edit the tap,
  which is overwritten on every release.
- The tap push needs the `HOMEBREW_TAP_GITHUB_TOKEN` secret (a fine-grained PAT
  with `Contents: write` on the tap). If it expires, the Release still publishes
  but the "Update Homebrew tap" step fails — rotate the PAT and re-run that job.
- Verify a release end to end:

  ```bash
  brew install polesen/tap/datacow && datacow --version
  gh attestation verify datacow-<version>.tar.gz --repo polesen/datacow
  ```

## Reporting bugs / requesting features

Use the issue templates. For anything security-related, **do not open a public
issue** — see [`SECURITY.md`](SECURITY.md).

By contributing, you agree that your contributions are licensed under the
[MIT License](LICENSE).
