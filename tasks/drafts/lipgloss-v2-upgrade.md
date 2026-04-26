# Draft: Lipgloss v2 Upgrade

## Motivation

Lipgloss v2 (currently `v2.0.0-beta.2`, import path `charm.land/lipgloss/v2`) adds native
`BorderTitle` support and a cleaner adaptive-color API. These would eliminate two workarounds
that are currently baked into datacow:

1. **Manual border-title construction** — `renderPanel()` in `internal/tui/app.go` builds
   the top border line by hand (splicing title text between corner char and dashes) because
   v1 has no `BorderTitle` API. v2 makes this a one-liner on the style.
2. **`AdaptiveColor` struct** — v1's background detection is unreliable on some terminals.
   v2 replaces the struct with an explicit `LightDark(hasDark)` function that takes real I/O.

## What v2 Adds (relevant to datacow)

| Feature | Impact |
|---|---|
| `style.BorderTitle(" text ")` | Removes `renderPanel()` manual border hack entirely |
| `lipgloss.HasDarkBackground(stdin, stdout)` | Explicit, reliable dark-mode detection |
| `lipgloss.LightDark(hasDark)(light, dark)` | Replaces `AdaptiveColor{}` struct |
| `BorderForegroundBlend(colors...)` | Gradient borders on active panes (cosmetic) |
| `Blend1D` / `Blend2D` | Color gradient utilities for header fills etc. (cosmetic) |
| `Style` is a pure value | No more global-renderer coupling at package init |

## Breaking Changes (migration cost)

- **Import path**: `github.com/charmbracelet/lipgloss` → `charm.land/lipgloss/v2` — touches
  every file that imports lipgloss
- **`AdaptiveColor` struct removed**: all 6 uses in `internal/tui/style/style.go` need to
  change to `LightDark()` calls — requires detecting background at startup and threading
  the result into `style.go` (currently the style vars are package-level, initialised at
  import time)
- **`HasDarkBackground()` signature**: now takes `(io.Reader, io.Writer)` — must be called
  from `main.go` or `tui.New()`, not at init time
- **`WithWhitespaceForeground` / `WithWhitespaceBackground`** → `WithWhitespaceStyle(style)`
  — check if datacow uses these (probably not)
- **`renderer.NewStyle()` removed** — datacow doesn't use a custom renderer, so no impact

## Files That Would Change

- `internal/tui/app.go` — remove `renderPanel()`, use `style.PanelActive.BorderTitle(...)`
- `internal/tui/style/style.go` — rewrite all `AdaptiveColor{}` vars using `LightDark()`
- `cmd/main.go` or `internal/tui/app.go` — detect background at startup, pass to style init
- All files with `"github.com/charmbracelet/lipgloss"` imports — mechanical path change

## What Still Needs to Be Checked

### 1. Bubble Tea compatibility — CRITICAL
Bubble Tea also has a v2 in progress (`github.com/charmbracelet/bubbletea` → `charm.land/bubbletea/v2`).
It is not confirmed whether:
- Bubble Tea v1 works with lipgloss v2 (different module paths, may have interface conflicts)
- Or whether upgrading lipgloss v2 forces a Bubble Tea v2 upgrade at the same time

If they must move together, the scope doubles. Bubble Tea v2 has its own breaking changes
(command model, message routing). **This is the most important unknown before committing to
the upgrade.**

Action: check `charm.land/bubbletea/v2` go.mod to see what lipgloss version it requires,
and check whether `charm.land/lipgloss/v2` go.mod lists bubbletea as a dependency.

### 2. Bubbles compatibility
`github.com/charmbracelet/bubbles` (spinner, textinput) is used in datacow. Does bubbles
have a v2? Does it depend on lipgloss v1 or v2? If bubbles still requires lipgloss v1,
we'd have two lipgloss versions in the module graph.

### 3. `BorderTitle` API details
The context7 docs showed `style.BorderTitle(" text ")` but didn't show:
- Whether title alignment is configurable (left / center / right)
- Whether title style (foreground color, bold) is separate from border style
- Whether the title truncates gracefully when the panel is narrow

Check the v2 README or source for `BorderTitle` / `BorderTitleStyle` / `BorderTitlePosition`.

### 4. `LightDark` + package-level style vars
`internal/tui/style/style.go` currently defines all colors as package-level `var` blocks,
which means they're evaluated at import time. `LightDark()` requires knowing the terminal
background, which isn't available until runtime. Need a design for threading the dark/light
decision into the style package — options:
- `style.Init(hasDark bool)` called from `tui.New()`
- Replace package-level vars with functions `style.PanelActive() lipgloss.Style`
- Pass styles as arguments (too invasive)

### 5. v2 stability
`v2.0.0-beta.2` as of April 2026. Watch for stable release. API may still shift,
particularly around `BorderTitle` and the color system.

## Recommendation

Hold until:
1. Bubble Tea + Bubbles v2 compatibility is confirmed (or they go stable together)
2. `BorderTitle` API is verified to cover our use case (title style, position, truncation)
3. v2 reaches stable or at least a later beta that signals API freeze

The manual `renderPanel()` workaround is self-contained and correct. There is no urgent
reason to migrate before the ecosystem stabilises.
