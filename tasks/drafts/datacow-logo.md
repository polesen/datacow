# Datacow Logo & Branding

Datacow has no logo yet. This task creates one — a single visual identity used across the TUI,
the future web app, and the README. The brief is: lean into the name. A cow + data motif,
playful or a little weird, but never offensive. It must work at favicon scale (16×16) and
as ASCII art in a terminal.

## Deliverables

Three artefacts, all expressing the same logo concept:

1. **SVG master** — real vector geometry (paths, shapes), not a `<image>` tag wrapping a
   bitmap. Single colour or limited palette so it works on light and dark backgrounds.
   Lives at `assets/logo.svg`.
2. **Favicon set** — generated from the SVG: `favicon.ico` (16/32/48 multi-size), plus
   `favicon-32.png`, `favicon-180.png` (Apple touch). Lives in `assets/favicon/`.
   The favicon must be legible at 16×16 — a full ASCII cow won't survive that. Design
   from the favicon outward, not the splash inward.
3. **ASCII art** — terminal-safe, displayed as a startup splash in the TUI. Plain ASCII
   preferred; Unicode box-drawing acceptable if it renders cleanly in common terminals.
   Lives at `internal/tui/views/splash.go` as a `const Splash = ...` (multi-line string).

## Design Constraints

- **Theme**: cow + data. The name does the work — don't be subtle. Some directions to
  explore (pick one, don't ship all of them):
  - A cow with database-cylinder spots
  - A cow whose udder is a stack of database cylinders
  - A "happy cow at a terminal" vibe
  - A cow silhouette with a tiny cursor blink
- **Tone**: fun, slightly weird is welcome. Not edgy, not crude, not political. No bodily
  fluids, no anthropomorphised distress. If in doubt, leaner and friendlier.
- **Scalability**: the SVG must read clearly at 1024px, 64px, and 16px. Test all three.
  Tiny details disappear at favicon size — the silhouette has to carry the identity.
- **Palette**: monochrome-friendly. The SVG should look correct rendered in a single
  foreground colour (so it can inherit `currentColor` in HTML and adapt to dark mode).
  If a colour version is added, keep it to 2–3 colours max.
- **ASCII**: between 6 and 12 rows tall, no wider than 60 columns (must fit in a narrow
  terminal). No characters that require a specific font (`█` is fine, emoji is not).
  Test in a terminal at 80 columns before committing.

## TUI Splash Behaviour

The splash shows during the initial schema cache load — replacing or complementing the
current spinner-only screen. Behaviour:

- On startup, before the schema cache is ready, render the ASCII logo centred in the
  available area with the version string and a small "loading…" line beneath it.
- When `schemaCacheReadyMsg` arrives, transition to the normal split view immediately —
  no artificial delay, no fade. The splash is a load screen, not a brand moment users
  have to wait through.
- If startup is fast enough that the splash would flash for <200ms, that's fine. Don't
  add a minimum display time.
- The error screen (`screenError`) keeps its current rendering; no splash there.
- Multi-datasource picker (`screenDatasourcePicker`) keeps its current rendering; no
  splash there either. The splash is specifically for the "connecting + loading schema"
  window, which is the only blocking phase in single-datasource mode.

Render integration point: `App.renderContent()` in `internal/tui/app.go`, gated on
`a.cacheLoading && a.screen == screenSplit`. The current spinner-only path inside the
split-view render needs to be replaced with the splash view for the pre-ready state.

## Files Touched

| File | Change |
|---|---|
| `assets/logo.svg` | New — SVG master |
| `assets/favicon/favicon.ico` | New — multi-size ICO |
| `assets/favicon/favicon-32.png` | New |
| `assets/favicon/favicon-180.png` | New |
| `internal/tui/views/splash.go` | New — `SplashView` model + `Splash` constant |
| `internal/tui/views/splash_test.go` | New — view unit tests |
| `internal/tui/app.go` | Wire splash into the pre-ready render path |
| `README.md` | Add logo at top (referencing `assets/logo.svg`) |

## Acceptance Criteria

- `assets/logo.svg` opens in a browser and renders as vector geometry — no `<image>` tag,
  no embedded base64 bitmap.
- The SVG, rendered at 16×16, is recognisable as the same mark as the 1024px version
  (silhouette survives downscaling).
- `assets/favicon/favicon.ico` contains 16/32/48 sizes (`identify favicon.ico` shows
  three frames) and the 16×16 frame is legible.
- Running `datacow --connection-string=...` shows the ASCII splash centred during schema
  load, then disappears once the schema is ready. Confirmed by manual test.
- The splash never appears on the error screen, the datasource picker, or after the
  schema cache is ready.
- View unit tests cover: splash renders the ASCII art, version string, and loading line;
  splash width/height fit inside a 24-row × 80-column window without truncation.
- README shows the logo near the top.

## What NOT to Change

- The existing spinner — keep `spinner.MiniDot` available; it may still be used inside
  the loading-line beneath the ASCII art. Don't rip it out.
- The error screen, datasource picker, help overlay, query log, or any non-startup view.
- Keybindings — the splash is non-interactive; any keypress just gets queued for the
  underlying app as normal. No new bindings.
- Lipgloss/Bubble Tea versions — work with what's installed.
- Colour scheme of the TUI overall. The splash inherits the existing foreground colour;
  do not introduce a new palette.

## Definition of Done

See [definition-of-done.md](../definition-of-done.md). All gates must pass.

Manual:
1. Start `datacow` against a small DB — splash visible during load, then split view.
2. Start `datacow` against a large DB (or one with many tables) — splash visible longer,
   still transitions cleanly when ready.
3. Open `assets/logo.svg` in a browser at full size, then resize to 16px — silhouette
   still readable.
4. Open `assets/favicon/favicon.ico` in a browser tab via a local HTML file — appears
   in the tab.
