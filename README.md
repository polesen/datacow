# Datacow

Zero-config terminal database explorer. Connect with a single connection string and immediately navigate tables, drill through foreign key relationships, filter, sort, and export — no SQL required.

Like [k9s](https://github.com/derailed/k9s) or [lazygit](https://github.com/jesseduffield/lazygit), but for databases.

---

> **Experiment in progress**
>
> A deliberate exploration of how far vibe coding can go with current AI tooling. Every line of code, architecture, tests, and documentation were produced by Claude (Anthropic) — no code written by hand. The guidance comes from years of hands-on experience across languages, architecture, and testing.
>
> The tool is real and usable. The experiment is ongoing.

---

## Features

- **Zero-config** — point at a connection string, auto-discovers all tables and views
- **Keyboard-driven TUI** — navigate entirely without a mouse
- **FK drill-down** — follow foreign key relationships between tables interactively
- **Filter and sort** — filter rows by column value, sort by any column
- **Export** — save to CSV or Excel directly from the browser
- **Query log** — see every SQL query executed, with timing
- **Cell viewer** — inspect long values and save cell contents to file
- **Schema explorer** — browse the full schema tree (tables, columns, types)
- **Multi-datasource** — switch between multiple databases in one session
- **Custom datasets** — define named views as saved SQL queries via YAML config
- **PostgreSQL and MySQL** — full support for both; SQLite, MSSQL, Oracle planned

---

## Usage

### Zero-config (quickest way to start)

```bash
datacow --connection-string="postgres://user:pass@localhost/mydb"
datacow --connection-string="mysql://user:pass@localhost/mydb"
```

All tables and views are auto-discovered as datasets. No configuration required.

### With a config file

```yaml
# datacow.yaml
datasources:
  - name: production
    connection_string: postgres://user:pass@prod-host/mydb
  - name: analytics
    connection_string: mysql://user:pass@analytics-host/reports

datasets:
  - name: recent_orders
    datasource: production
    query: "SELECT * FROM orders WHERE created_at > now() - interval '7 days'"
```

```bash
datacow --config datacow.yaml
```

### Keybindings

| Key | Action |
|-----|--------|
| `↑` / `↓` / `j` / `k` | Move cursor |
| `Enter` | Open dataset / drill into FK |
| `f` | Filter rows |
| `s` | Sort by column |
| `e` | Export (CSV / Excel) |
| `v` | Open cell viewer |
| `l` | Toggle query log |
| `Backspace` | Go back |
| `q` / `Ctrl+C` | Quit |

---

## Installation

### Homebrew (macOS / Linux)

```bash
brew install polesen/tap/datacow
```

Upgrade later with `brew upgrade datacow`.

> Distributed through Homebrew, which installs without triggering macOS
> Gatekeeper notarization prompts. Release binaries are built in CI and
> published with SHA256 checksums and a SLSA build-provenance attestation.

### From source

```bash
git clone https://github.com/polesen/datacow
cd datacow
go build -o datacow ./cmd
```

Go 1.25 or later required.

---

## Architecture

Three layers, one binary — core library used by both TUI and HTTP server (planned):

```
TUI app          HTTP server (planned)
    \               /
     [Core library]       ← all business logic
            |
       DB drivers
```

The TUI does not go through an HTTP API — it uses the core library directly. This keeps it fast, standalone, and zero-dependency.

```
/cmd                     # Entry point
/internal
  /core
    /db                  # DB connection + driver abstraction
    /schema              # Schema introspection
    /dataset             # Dataset resolution + query execution
    /export              # CSV, Excel export
    /config              # YAML config loading
  /tui                   # Bubble Tea TUI
    /views               # Panel components
    /keys                # Keybinding registry
```

---

## Status

Early development. Core TUI is functional and usable against PostgreSQL and MySQL. No stable release yet.

**Implemented:** TUI, PostgreSQL + MySQL drivers, filter/sort/export, FK drill-down, query log, cell viewer, schema explorer, multi-datasource, YAML config + custom datasets.

**Planned:** HTTP API, web app, natural language filtering via LLM.

---

## Contributing

Contributions are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md). For security
issues, please follow [SECURITY.md](SECURITY.md) and report privately.

## License

[MIT](LICENSE) © Per Olesen.
