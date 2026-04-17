# Sample Database Schema Overview

**Theme:** SaaS Analytics Platform for multi-tenant organizations

**Tables:** 79 | **Rows:** ~4,800 | **JSON Snapshots:** 10 (250KB each)

---

## Core Tables (Multi-Tenant Foundation)

### Organizations & Users

| Table | Rows | Purpose |
|-------|------|---------|
| `organizations` | 10 | Tenants: Acme Corp, TechStartup Inc, Healthcare Net, etc. |
| `users` | 100 | Users distributed across orgs (1:10 ratio) |
| `teams` | 50 | Department-level grouping within orgs (FK: `organization_id`, `team_lead_id`) |
| `roles` | 5 | Org-scoped roles (admin, editor, viewer) with permission JSON |
| `user_team_memberships` | ~50 | M2M junction with `joined_at`, `role_id` |

**Sample Data:**
- Orgs: enterprise, pro, starter plans
- Users: distributed across all orgs
- Teams: team lead assigned from available users

---

## Workspaces & Projects

| Table | Rows | Purpose |
|-------|------|---------|
| `workspaces` | ~20 | User-created workspaces within orgs (FK: `organization_id`, `owner_id`) |
| `workspace_members` | — | Members invited to specific workspaces (EMPTY—test empty views) |
| `projects` | 20 | Projects within workspaces (FK: `workspace_id`, `created_by_id`) |

**Relationships:**
- Organization → Workspace (1:N)
- Workspace → Project (1:N)
- User → Workspace (M:M via `workspace_members`)

---

## Events & Analytics

### Tracking

| Table | Rows | Purpose |
|-------|------|---------|
| `events` | 500 | Raw events: page_view, click, submit, error, conversion, signup |
| `page_views` | 200 | Detailed page view events (FK: `event_id`, `user_id`, `organization_id`) |
| `conversions` | 100 | Conversion events with amount and currency |

### Analytics Aggregates

| Table | Rows | Purpose |
|-------|------|---------|
| `analytics_events` | 100 | JSON-payload analytics events (small JSON) |
| `analytics_snapshots` | 10 | **Large JSON (~250KB each)** — test large cell rendering |
| `metrics` | 25 | Computed metrics: gauge, counter, histogram (FK: `workspace_id`) |
| `reports` | 15 | Custom reports with config JSON (FK: `workspace_id`, `created_by_id`) |
| `report_results` | — | Query results from reports (EMPTY—test empty views) |

**Notable:**
- `analytics_snapshots.data_payload` contains ~250KB JSON for testing large cells
- `metrics.metric_value` and `metrics.unit` for flexible metric types

---

## Content Management

| Table | Rows | Purpose |
|-------|------|---------|
| `documents` | 50 | Workspace documents with view count, publish status (FK: `workspace_id`, `created_by_id`) |
| `articles` | 30 | Blog-style articles with slug, excerpt, featured image (FK: `workspace_id`, `author_id`) |
| `templates` | 3 | System and org templates (FK: `organization_id`) |
| `pages` | — | Static pages (EMPTY—test empty views) |

**Relationships:**
- Workspace → Document (1:N)
- Workspace → Article (1:N)
- Organization → Template (1:N)

---

## Products & Orders

### Catalog

| Table | Rows | Purpose |
|-------|------|---------|
| `products` | 50 | Products with SKU, category, status (FK: `organization_id`) |
| `product_variants` | 100 | Variants with attributes JSON (FK: `product_id`) |
| `product_pricing` | — | Regional pricing per variant (FK: `variant_id`) |
| `product_images` | 80 | Product images with alt text, sort order (FK: `product_id`) |
| `product_reviews` | — | Customer reviews (EMPTY—test empty views) |

### Orders

| Table | Rows | Purpose |
|-------|------|---------|
| `orders` | 150 | Orders with status, total_amount (FK: `organization_id`, `user_id`) |
| `order_items` | 300 | Order line items (FK: `order_id`, `variant_id`) |
| `payments` | 100 | Payment records per order (FK: `order_id`) |
| `shipments` | — | Shipment tracking (EMPTY—test empty views) |

**Relationships:**
- Product → Variant (1:N)
- Variant → Stock Level (1:N)
- Order → OrderItem (1:N)
- OrderItem → Variant (M:1)

---

## Billing & Subscriptions

| Table | Rows | Purpose |
|-------|------|---------|
| `plans` | 3 | Pricing plans: Starter, Professional, Enterprise |
| `subscriptions` | 50 | Active/paused/cancelled subscriptions (FK: `plan_id`, `user_id`) |
| `invoices` | 100 | Invoices with status (draft, sent, paid, overdue) (FK: `subscription_id`) |
| `invoice_items` | — | Line items per invoice (EMPTY—test empty views) |

**Sample Data:**
- Plans: $29.99 (month), $99.99 (month), $499.99 (month)
- Subscriptions: distributed across organizations

---

## Inventory Management

| Table | Rows | Purpose |
|-------|------|---------|
| `warehouses` | 10 | Physical warehouses with capacity (FK: `organization_id`) |
| `stock_levels` | 200 | Current stock per variant per warehouse (FK: `warehouse_id`, `variant_id`) |
| `stock_movements` | 300 | Stock in/out/adjustment/return history (FK: `warehouse_id`, `variant_id`) |
| `stock_forecasts` | — | Predicted stock levels (EMPTY—test empty views) |

**Relationships:**
- Warehouse → StockLevel (1:N)
- Variant → StockLevel (1:N, composite FK)
- StockLevel → StockMovement (1:N)

---

## Configuration & Integration

### Settings

| Table | Rows | Purpose |
|-------|------|---------|
| `settings` | 100 | Org-scoped settings: email_enabled, sso_enabled, timezone, currency, etc. (FK: `organization_id`) |
| `feature_flags` | 4 | Global feature flags: dark_mode, new_dashboard, export_csv, ai_assistant |
| `integrations` | 23 | Third-party integrations: Slack, Salesforce, Stripe, GitHub, etc. (FK: `organization_id`) |
| `webhooks` | 20 | Event webhooks for integrations with proper FK refs (FK: `integration_id`) |

### API & Data Connections

| Table | Rows | Purpose |
|-------|------|---------|
| `api_keys` | 30 | API keys per user with scopes JSON (FK: `user_id`) |
| `data_connections` | 15 | DB connections: postgres, mysql, mongodb with SSH tunnel support (FK: `organization_id`) |
| `sync_jobs` | 30 | Scheduled sync jobs for data connections (FK: `connection_id`) |
| `sync_job_runs` | — | Historical runs per sync job (EMPTY—test empty views) |

**Wide Tables (Many Columns):**
- `data_connections`: 27 columns including SSL certs, SSH keys, pool settings, custom options

---

## Audit & Monitoring

### Logging

| Table | Rows | Purpose |
|-------|------|---------|
| `audit_logs` | 200 | Action audit trail: create, update, delete, view, export (FK: `user_id`) |
| `api_logs` | 100 | API request/response logging with status code, duration (FK: `user_id`) |
| `error_logs` | 50 | Application errors with stack traces (FK: `organization_id`) |
| `change_logs` | — | Schema change history (EMPTY—test empty views) |

### Monitoring

| Table | Rows | Purpose |
|-------|------|---------|
| `performance_metrics` | 200 | CPU, memory, disk, latency metrics with tags JSON (FK: `organization_id`) |
| `uptime_checks` | 100 | Service health checks (status: up, down, degraded) |
| `alerts` | 50 | Critical alerts with severity (info, warning, critical) |
| `alert_assignments` | — | Alert assignments to users (EMPTY—test empty views) |

---

## Collaboration

| Table | Rows | Purpose |
|-------|------|---------|
| `comments` | 100 | Comments on resources with nested replies (FK: `parent_comment_id`) |
| `mentions` | 30 | @mentions in comments (FK: `mentioned_user_id`) |
| `notifications` | 150 | User notifications: comment, mention, assignment, review_request (FK: `actor_user_id`) |
| `watches` | — | Resource watches (EMPTY—test empty views) |

**Relationships:**
- Comment → ChildComment (self-referential via `parent_comment_id`)
- Comment → Mention (1:N)
- User → Notification (1:N, M:1 via `actor_user_id`)

---

## Reporting & Dashboards

| Table | Rows | Purpose |
|-------|------|---------|
| `dashboards` | 20 | Custom dashboards (FK: `workspace_id`, `created_by_id`) |
| `visualizations` | 50 | Charts: line, bar, pie, scatter, table, gauge, heatmap (FK: `dashboard_id`) |
| `scheduled_reports` | 10 | Email schedules for reports (cron format) (FK: `report_id`) |
| `export_jobs` | — | CSV/Excel export history (EMPTY—test empty views) |

---

## Custom Fields & Metadata

| Table | Rows | Purpose |
|-------|------|---------|
| `custom_field_definitions` | 30 | Dynamic schema: text, number, date, select, checkbox (FK: `organization_id`) |
| `custom_field_values` | — | Values for custom fields (references definitions) |
| `entity_metadata` | — | Flexible metadata JSON for any entity (FK: `organization_id`) |

---

## File Storage

| Table | Rows | Purpose |
|-------|------|---------|
| `files` | 100 | Uploaded files with MIME type, size (FK: `workspace_id`, `uploaded_by_id`) |
| `file_versions` | — | Version history per file (FK: `file_id`) |
| `file_permissions` | — | Granular file permissions (FK: `file_id`, `user_id`) |
| `file_shares` | — | Public shares with expiration (EMPTY—test empty views) |

---

## Usage Tracking

| Table | Rows | Purpose |
|-------|------|---------|
| `feature_usage` | 200 | Feature usage counters: export, api_call, filter, report, visualization (FK: `user_id`) |
| `usage_quotas` | 20 | Monthly/daily quotas per org (FK: `organization_id`) |
| `usage_billing` | — | Metered usage for billing period |

---

## Support & Feedback

| Table | Rows | Purpose |
|-------|------|---------|
| `support_tickets` | 50 | Support tickets with status, priority (FK: `assigned_to_id`) |
| `ticket_messages` | — | Conversation history per ticket (FK: `ticket_id`) |
| `user_feedback` | 80 | NPS and feedback with rating 1-5 (FK: `user_id`) |
| `help_articles` | — | Knowledge base articles (EMPTY—test empty views) |

---

## Performance Profiling

| Table | Rows | Purpose |
|-------|------|---------|
| `query_performance` | 150 | Query execution time, rows affected, hash (FK: `organization_id`) |
| `endpoint_performance` | 150 | API endpoint latency (GET, POST, PUT, DELETE) |
| `resource_usage` | — | CPU/memory/disk per resource (FK: `organization_id`) |
| `optimization_suggestions` | — | Auto-generated optimization hints (EMPTY—test empty views) |

---

## Key Patterns

### Foreign Key Relationships: 125+

**Multi-level nesting:**
```
Organization → Workspace → Project
Organization → User → Team (M:M via UserTeamMembership)
Product → ProductVariant → StockLevel (per Warehouse)
Organization → DataConnection → SyncJob → SyncJobRun
Order → OrderItem → ProductVariant → Product
```

**Self-referential:**
```
Comment → Comment (parent_comment_id)
```

**M:M junctions:**
```
User ↔ Team (via user_team_memberships)
```

### Empty Tables (7 total)

For testing empty view rendering:
- `workspace_members`
- `pages`
- `report_results`
- `change_logs`
- `sync_job_runs`
- `watches`
- `export_jobs`
- `help_articles`
- `alert_assignments`
- `optimization_suggestions`
- `file_shares`
- `stock_forecasts`
- `ticket_messages`
- `custom_field_values`
- `invoice_items`
- `file_permissions`
- `file_versions`
- `resource_usage`
- `product_reviews`
- `shipments`
- `pricing`
- `entity_metadata`
- `usage_billing`

### Large JSON Fields

**250KB JSON (~10 rows):**
- `analytics_snapshots.data_payload` — test large cell rendering in TUI

**Small-to-medium JSON (23 fields):**
- `custom_field_definitions.validation_rules`
- `integrations.config`, `.credentials`
- `webhooks.headers`
- `settings.value` (some)
- `roles.permissions`
- `notifications.metadata`
- etc.

### Wide Tables (Many Columns)

- `data_connections`: 27 columns (host, port, SSL, SSH, pool settings, custom options)
- Most standard tables: 8-15 columns

---

## Drill-Down Navigation Test Cases

The schema supports extensive FK navigation:

1. **Organization → Users → Teams → Projects → Documents**
2. **Warehouse → StockLevels → Variants → Products → Orders**
3. **Integration → Webhooks** (23 integrations, 20 webhooks)
4. **Dashboard → Visualizations** (20 dashboards, 50 charts)
5. **Order → OrderItems → Variants → ProductPricing** (regional pricing)
6. **Comment → Mentions → Users** (nested mentions)
7. **Ticket → Messages → Users** (support conversation)

---

## Sample Data Characteristics

- **Dates:** Realistic timestamps within ±365 days of today
- **Organization Distribution:** Users and resources evenly distributed across 10 orgs
- **Status Values:** active/paused/cancelled, open/closed, draft/published, etc.
- **Amounts:** Realistic pricing ($10-$1000 range for conversions/orders)
- **Emails:** Realistic format with domains (acme.com, corp.io, example.com)
- **Slugs:** URL-safe identifiers (lowercase, hyphens)

---

## Initialization

Both PostgreSQL and MySQL load automatically on container startup:

```bash
# PostgreSQL
POSTGRES_DB: datacow
# Init: /docker-entrypoint-initdb.d/01-init.sql

# MySQL  
MYSQL_DATABASE: datacow
# Init: /docker-entrypoint-initdb.d/01-init.sql
```

Generated from: `scripts/generate-sample-db.py`

---

## Quick Queries

### User Activity
```sql
SELECT u.email, COUNT(e.id) as event_count
FROM users u
LEFT JOIN events e ON u.id = e.user_id
GROUP BY u.id, u.email
ORDER BY event_count DESC;
```

### Revenue by Organization
```sql
SELECT o.name, SUM(c.amount) as total_revenue
FROM organizations o
LEFT JOIN conversions c ON o.id = c.organization_id
GROUP BY o.id, o.name
ORDER BY total_revenue DESC;
```

### Popular Articles
```sql
SELECT a.title, a.view_count, u.email as author
FROM articles a
LEFT JOIN users u ON a.author_id = u.id
ORDER BY a.view_count DESC
LIMIT 10;
```

### Active Subscriptions by Plan
```sql
SELECT p.name, COUNT(s.id) as subscriber_count
FROM plans p
LEFT JOIN subscriptions s ON p.id = s.plan_id
WHERE s.status = 'active'
GROUP BY p.id, p.name;
```

