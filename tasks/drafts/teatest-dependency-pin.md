# Draft: Un-pin charmbracelet/x/exp/teatest from pseudo-version

## Background

`go.mod` pins `github.com/charmbracelet/x/exp/teatest` to a pseudo-version
(`v0.0.0-20260413165052-6921c759c913`) because the package has never had a
tagged release of its own.

`exp/teatest` is its own Go module — it has its own `go.mod`
(`module github.com/charmbracelet/x/exp/teatest`), separate from the root
`github.com/charmbracelet/x` module. With no tags of its own, Dependabot's
grouped `go_modules` update tried bumping it against the root module's
`v0.1.0` tag — which doesn't contain `exp/teatest` (it was split out into its
own module) — and failed the whole grouped update with
`dependency_file_not_resolvable`. It's now `ignore`d in
`.github/dependabot.yml`.

## Two paths forward

1. **Wait for a tagged `exp/teatest` release** (e.g. `exp/teatest/vX.Y.Z`).
   Once one exists, bump to it, drop the Dependabot `ignore` rule, and confirm
   the `teatest`-based app integration tests (see `internal/tui/views` TDD
   guidance in `CLAUDE.md`) still pass.

2. **Move to `exp/teatest/v2`** as part of the Bubble Tea v2 migration already
   tracked in `tasks/drafts/lipgloss-v2-upgrade.md`. `exp/teatest/v2` requires
   `charm.land/bubbletea/v2` (currently `v2.0.0-rc.1`) — same ecosystem-wide
   v2 move, same "hold until stable" reasoning as that draft.

## Recommendation

Hold — same as `lipgloss-v2-upgrade.md`. Revisit both together once Bubble Tea
v2 reaches a stable release; that's the natural point to pick up
`exp/teatest/v2` too. Until then, the pseudo-version pin plus the Dependabot
`ignore` is a reasonable, low-maintenance state.
