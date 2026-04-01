# Datacow — Product Definition

> Living document. Updated iteratively through discovery sessions.

---

## Problem

Existing database exploration tools either target developers with SQL knowledge, or require significant upfront setup and configuration before non-technical users can get any value from them.

**The gap:** There is no tool that lets you connect to *any* SQL database with nothing more than a connection string, and immediately navigate and visualize data — without needing to know SQL or configure anything.

---

## Vision

Like k9s or lazygit — but for databases.

A zero-config database explorer that works the moment you point it at a connection string. Navigate tables, drill through relationships, filter and export — without writing SQL or configuring anything upfront. Ask questions in plain English and get answers directly from your data. Available as a world-class TUI for terminal users and a clean web app for everyone else.

## Users

### Non-technical users (web app)
- **Roles:** Sales, Project Manager, Support
- **Current behavior:** Ask developers for reports, queries, or data insights
- **Goal:** See data directly without needing a developer intermediary

### Terminal users (TUI app)
- **Roles:** Anyone comfortable in a terminal — most likely developers, but not exclusively
- **Same core functionality** as the web app (datasources, datasets, drill-down, download)
- **Additional capabilities:**
  - Scriptable — output can be piped, used in automation and shell scripts
  - Shell completions — context-aware tab completion (datasources, datasets, columns, filters)
  - Terminal-native UX (keyboard-driven, no mouse required)

## Key Concepts

### Datasource
A connection to a database. Defined by a connection string. In zero-config mode, passed directly on the command line. In persistent mode, defined in `datasources.yaml`.

### Dataset
A named view into data. Can be:
- A plain table (select all rows/columns) — auto-discovered from schema
- A saved SQL query — the query *is* the configuration of the dataset

In zero-config mode, all tables and views are automatically datasets. In configured mode, custom datasets are defined in `datasets.yaml`.

---

## Core Use Cases

### 0. Zero-config entry point (key promise)

```
datacow --connection-string=...
```

With nothing else configured, the tool auto-discovers the schema and gives you:
- All tables and views listed as datasets
- Navigate into any dataset to see its contents (paged)
- Filter and sort
- Foreign key relationships auto-detected — drill down from parent to child
- Export to CSV, Excel, etc.

No YAML, no setup, no SQL knowledge required.

YAML config (datasources, datasets, users) is the *optional* layer for persistent, shared, or access-controlled setups.

---

### 1. Non-technical user — web app journey

1. **Select a datasource** — choose which database to work with
2. **Browse datasets** — recently used / frequently used shown first; full list available
3. **Act on a dataset:**
   - **Download** — export directly to CSV or Excel, no need to open
   - **Browse** — open a paged table view with sorting and filtering
   - **Drill down** — click a foreign key cell to navigate to related records

### 2. Drill-down navigation (key differentiator)
- Navigating via a foreign key *appends* the child dataset below the current view
- The parent remains visible above — scroll up to return context
- Multiple levels of drill-down are possible, building a vertical "trail"
- This makes relational data explorable without knowing SQL or understanding schema

## Configuration & Setup

All configuration is done manually via YAML files — no admin UI required to get started. This keeps the initial setup simple and config version-controllable.

### Config files (draft structure)
- `datasources.yaml` — database connection strings and metadata
- `datasets.yaml` — named views: either a table reference or a saved SQL query
- `users.yaml` — users, their roles, and dataset assignments

### Auth & Access Control (v1)
- **Admin role** — full access to all datasources and datasets
- **Viewer role** — access only to explicitly assigned datasets (assigned via `users.yaml`)
- **API keys** — for TUI and scripting use (no interactive login required)
- OAuth/SSO deferred to a later version

## Database Support

**Must support (v1):**
- PostgreSQL
- MySQL

**Should support:**
- SQLite
- Microsoft SQL Server
- Oracle

**Design principle:** Any database with a reasonably standard SQL dialect should be addable via a driver. Go's `database/sql` interface provides the same abstraction as JDBC — a standard interface with pluggable drivers.

---

## Platform & Technology

**Language: Go**

Rationale:
- Single binary, no runtime — download and run on Mac, Linux, Windows
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Charm) for world-class TUI
- `database/sql` + pluggable drivers for all target databases
- Fast compilation, easy cross-platform distribution (Homebrew, direct download, etc.)
- Strong CLI ecosystem: Cobra (commands + completions), Lip Gloss (TUI styling)

### Architecture

Three layers, one binary:

```
TUI app          HTTP server
    \               /
     \             /
      [Core library]       ← all logic lives here
            |
       DB drivers
```

- **Core library** — schema introspection, dataset resolution, FK detection, filtering, export. All business logic lives here.
- **TUI** — uses core directly. Works standalone with just a connection string. No server required.
- **`datacow serve`** — starts an HTTP server that uses core and also serves the embedded web app. Exposes a full REST API for third-party integrations.
- **Web app** — embedded in the binary, talks to the HTTP API.

The TUI does *not* go through the HTTP API — this keeps it fast, zero-dependency, and true to the zero-config promise.

## MVP (v0.1)

**Goal:** Something you can show someone and say "this is Datacow."

**Scope:**
- TUI only (no web app, no HTTP server)
- Connect to PostgreSQL or MySQL via connection string
- Auto-discover all tables
- Navigate into a table and see its data, paged and readable
- Basic sorting and filtering

**Success criterion:** A developer runs `datacow --connection-string=...`, browses a table, and says "oh, this is useful."

Everything else — web app, datasets, YAML config, auth, export, drill-down — comes after.

---

## Boundaries

**Out of scope for v1:**
- Writing data (INSERT, UPDATE, DELETE) — read-only for now, editing is a future feature
- Charting or graphical visualizations — tabular data only
- Arbitrary SQL query editor — users navigate and filter, not write SQL
- Admin UI for managing datasources/datasets/users — YAML config only

**Planned for later:**
- In-place data editing

## AI Integration

AI removes the need to know SQL or understand the schema — which is exactly Datacow's core promise. LLM provider is pluggable (Anthropic, OpenAI, Ollama for local, etc.), configured via YAML or environment variable.

### Priority features

**1. Natural language filtering**
Type plain English to filter data: *"show orders from last month over $500"* → translated to SQL WHERE clauses automatically. Replaces a complex filter UI, especially valuable for non-technical users.

**2. Natural language → dataset**
Describe what you want: *"customers who haven't ordered in 90 days"* → AI generates the SQL query → developer reviews and saves it as a named dataset. Dataset creation without writing SQL.

**3. Data Q&A / chat**
Ask questions about the data currently in view: *"why are there so many failed orders on Tuesdays?"* → AI answers in plain English using the visible data as context. A co-pilot for non-technical users.

### Architecture note
The core library will include an LLM abstraction layer — a standard interface with pluggable provider implementations. Schema context (table names, column names, types, FK relationships) is passed automatically as context to the LLM.

### Deferred AI features
- Schema descriptions (auto-generate human-readable column descriptions)
- Anomaly / insight surfacing (proactive observations when opening a dataset)

---

