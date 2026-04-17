#!/usr/bin/env python3
"""
Generate a themed sample database with 100 tables, FK relationships,
and realistic data for testing Datacow.

Theme: SaaS analytics platform with multi-tenant organizations, users, events, and data.
"""

import json
import random
import string
from datetime import datetime, timedelta
from typing import List, Tuple

# Minimal data generation - avoid using heavy libraries if possible
def random_string(length: int = 10) -> str:
    return ''.join(random.choices(string.ascii_lowercase, k=length))

def random_email(name: str = None) -> str:
    if name is None:
        name = random_string(8)
    domain = random.choice(['acme.com', 'example.com', 'corp.io'])
    return f"{name.lower()}.{random_string(6)}@{domain}"

def random_date(days_back: int = 365) -> str:
    date = datetime.now() - timedelta(days=random.randint(0, days_back))
    return date.strftime('%Y-%m-%d %H:%M:%S')

def random_json_small() -> str:
    """Small JSON for metadata, configs, etc."""
    data = {
        "version": random.choice(["1.0", "2.0", "3.0"]),
        "enabled": random.choice([True, False]),
        "tags": [random_string(5) for _ in range(random.randint(1, 3))],
        "priority": random.randint(1, 5)
    }
    return json.dumps(data).replace("'", "''")

def random_json_large() -> str:
    """Large JSON for testing large cell values."""
    # Create ~50KB JSON
    data = {
        "id": random_string(20),
        "created_at": datetime.now().isoformat(),
        "metadata": {
            f"key_{i}": {
                "value": random_string(100),
                "nested": {
                    "deep_data": random_string(200),
                    "array": [random.randint(0, 1000) for _ in range(50)]
                }
            }
            for i in range(50)
        },
        "events": [
            {
                "timestamp": (datetime.now() - timedelta(hours=h)).isoformat(),
                "type": random.choice(["view", "click", "submit", "error"]),
                "details": random_string(500)
            }
            for h in range(100)
        ]
    }
    return json.dumps(data).replace("'", "''")

def random_json_huge() -> str:
    """Huge JSON (~250KB) for testing large content."""
    # Create ~250KB JSON - use simpler structure to avoid quote escaping issues
    payload = "x" * 250000  # Simple 250KB string
    data = {
        "id": random_string(20),
        "timestamp": datetime.now().isoformat(),
        "payload": payload,
        "metadata": {
            "size": len(payload),
            "type": "snapshot"
        }
    }
    # Proper escaping for SQL
    json_str = json.dumps(data)
    # Escape single quotes for SQL
    return json_str.replace("'", "''").replace("\\", "\\\\")

def random_text_large() -> str:
    """Large TEXT content."""
    paragraphs = []
    for _ in range(100):
        paragraphs.append(" ".join(random_string(8) for _ in range(50)))
    return ("\\n".join(paragraphs)).replace("'", "''")

class DatabaseGenerator:
    def __init__(self):
        self.org_ids = list(range(1, 11))  # 10 orgs
        self.user_ids = list(range(1, 101))  # 100 users
        self.team_ids = list(range(1, 51))  # 50 teams
        self.product_ids = list(range(1, 26))  # 25 products
        self.integration_ids = []  # Track created integration IDs
        self.table_count = 0

    def generate_postgres(self) -> str:
        """Generate PostgreSQL init script."""
        sql = "-- Datacow Sample Database (PostgreSQL)\n"
        sql += "-- Theme: SaaS Analytics Platform\n"
        sql += "-- 100+ tables with FK relationships, large JSON, and empty tables\n\n"
        sql += self.generate_core_tables('postgres')
        sql += self.generate_multi_tenant_tables('postgres')
        sql += self.generate_events_tables('postgres')
        sql += self.generate_content_tables('postgres')
        sql += self.generate_analytics_tables('postgres')
        sql += self.generate_configuration_tables('postgres')
        sql += self.generate_audit_tables('postgres')
        sql += self.generate_integration_tables('postgres')
        sql += self.generate_collaboration_tables('postgres')
        sql += self.generate_reporting_tables('postgres')
        sql += self.generate_products_tables('postgres')
        sql += self.generate_orders_tables('postgres')
        sql += self.generate_billing_tables('postgres')
        sql += self.generate_inventory_tables('postgres')
        sql += self.generate_monitoring_tables('postgres')
        sql += self.generate_custom_fields_tables('postgres')
        sql += self.generate_file_storage_tables('postgres')
        sql += self.generate_usage_tables('postgres')
        sql += self.generate_support_tables('postgres')
        sql += self.generate_performance_tables('postgres')
        return sql

    def generate_mysql(self) -> str:
        """Generate MySQL init script."""
        sql = "-- Datacow Sample Database (MySQL)\n"
        sql += "-- Theme: SaaS Analytics Platform\n"
        sql += "-- 100+ tables with FK relationships, large JSON, and empty tables\n\n"
        sql += self.generate_core_tables('mysql')
        sql += self.generate_multi_tenant_tables('mysql')
        sql += self.generate_events_tables('mysql')
        sql += self.generate_content_tables('mysql')
        sql += self.generate_analytics_tables('mysql')
        sql += self.generate_configuration_tables('mysql')
        sql += self.generate_audit_tables('mysql')
        sql += self.generate_integration_tables('mysql')
        sql += self.generate_collaboration_tables('mysql')
        sql += self.generate_reporting_tables('mysql')
        sql += self.generate_products_tables('mysql')
        sql += self.generate_orders_tables('mysql')
        sql += self.generate_billing_tables('mysql')
        sql += self.generate_inventory_tables('mysql')
        sql += self.generate_monitoring_tables('mysql')
        sql += self.generate_custom_fields_tables('mysql')
        sql += self.generate_file_storage_tables('mysql')
        sql += self.generate_usage_tables('mysql')
        sql += self.generate_support_tables('mysql')
        sql += self.generate_performance_tables('mysql')
        return sql

    def make_id_col(self, dialect: str) -> str:
        if dialect == 'postgres':
            return "id BIGSERIAL PRIMARY KEY"
        return "id BIGINT PRIMARY KEY AUTO_INCREMENT"

    def make_json_col(self, dialect: str) -> str:
        if dialect == 'postgres':
            return "JSONB"
        return "JSON"

    def generate_core_tables(self, dialect: str) -> str:
        sql = "-- Core: Organizations, Users, Teams, Roles\n\n"

        # Organizations
        sql += f"""CREATE TABLE organizations (
  {self.make_id_col(dialect)},
  name VARCHAR(255) NOT NULL,
  slug VARCHAR(100) UNIQUE NOT NULL,
  description TEXT,
  status VARCHAR(50) DEFAULT 'active',
  plan VARCHAR(50) DEFAULT 'free',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO organizations (name, slug, description, plan) VALUES
('Acme Corp', 'acme-corp', 'Enterprise analytics platform', 'enterprise'),
('TechStartup Inc', 'techstartup', 'Growing data company', 'pro'),
('Global Systems', 'global-sys', 'International organization', 'enterprise'),
('Local Services', 'local-svc', 'Regional business analytics', 'starter'),
('Digital Ventures', 'digital-ven', 'Innovation lab', 'pro'),
('Finance First', 'finance-first', 'Financial services firm', 'enterprise'),
('Retail Plus', 'retail-plus', 'Multi-chain retailer', 'enterprise'),
('Media Group', 'media-group', 'Content and publishing', 'pro'),
('Healthcare Net', 'healthcare-net', 'Healthcare provider', 'enterprise'),
('Education Hub', 'educ-hub', 'Educational institution', 'starter');
"""

        # Users
        sql += f"""CREATE TABLE users (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  email VARCHAR(255) UNIQUE NOT NULL,
  first_name VARCHAR(100),
  last_name VARCHAR(100),
  full_name VARCHAR(255),
  avatar_url TEXT,
  status VARCHAR(50) DEFAULT 'active',
  last_login TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        # Insert users
        for user_id in range(1, 101):
            org_id = self.org_ids[(user_id - 1) % len(self.org_ids)]
            email = random_email()
            sql += f"INSERT INTO users (organization_id, email, first_name, last_name, full_name, status) VALUES ({org_id}, '{email}', '{random_string(6)}', '{random_string(8)}', '{random_string(6)} {random_string(8)}', 'active');\n"

        # Teams
        sql += f"""CREATE TABLE teams (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  team_lead_id BIGINT,
  status VARCHAR(50) DEFAULT 'active',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (team_lead_id) REFERENCES users(id)
);
"""
        for team_id in range(1, 51):
            org_id = self.org_ids[(team_id - 1) % len(self.org_ids)]
            lead_id = random.choice(self.user_ids)
            sql += f"INSERT INTO teams (organization_id, name, description, team_lead_id) VALUES ({org_id}, 'Team {random_string(8)}', 'Team for {random_string(10)}', {lead_id});\n"

        # Roles
        sql += f"""CREATE TABLE roles (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  name VARCHAR(100) NOT NULL,
  description TEXT,
  permissions {self.make_json_col(dialect)},
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(organization_id, name),
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
INSERT INTO roles (organization_id, name, description, permissions) VALUES
(1, 'admin', 'Full access', '{{"read": true, "write": true, "delete": true}}'),
(1, 'editor', 'Edit content', '{{"read": true, "write": true}}'),
(1, 'viewer', 'Read only', '{{"read": true}}'),
(2, 'admin', 'Full access', '{{"read": true, "write": true, "delete": true}}'),
(2, 'member', 'Team member', '{{"read": true, "write": true}}');
"""

        # User Team Membership (M2M with data)
        sql += f"""CREATE TABLE user_team_memberships (
  {self.make_id_col(dialect)},
  user_id BIGINT NOT NULL,
  team_id BIGINT NOT NULL,
  role_id BIGINT,
  joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(user_id, team_id),
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (team_id) REFERENCES teams(id),
  FOREIGN KEY (role_id) REFERENCES roles(id)
);
"""
        for user_id in self.user_ids[:50]:
            for team_id in random.sample(self.team_ids, k=random.randint(1, 3)):
                role_id = random.randint(1, 5)
                sql += f"INSERT INTO user_team_memberships (user_id, team_id, role_id) VALUES ({user_id}, {team_id}, {role_id});\n"

        self.table_count += 5
        return sql

    def generate_multi_tenant_tables(self, dialect: str) -> str:
        sql = "-- Multi-tenant: Workspaces, Projects, Permissions\n\n"

        # Workspaces
        sql += f"""CREATE TABLE workspaces (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  owner_id BIGINT,
  color VARCHAR(7),
  is_default BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (owner_id) REFERENCES users(id)
);
"""
        for org_id in self.org_ids:
            for i in range(random.randint(2, 5)):
                owner_id = random.choice(self.user_ids)
                sql += f"INSERT INTO workspaces (organization_id, name, owner_id, color) VALUES ({org_id}, 'Workspace {random_string(6)}', {owner_id}, '#{random.randint(0, 0xFFFFFF):06x}');\n"

        # Workspace Members
        sql += f"""CREATE TABLE workspace_members (
  {self.make_id_col(dialect)},
  workspace_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  role VARCHAR(50) DEFAULT 'member',
  invited_by_id BIGINT,
  joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(workspace_id, user_id),
  FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (invited_by_id) REFERENCES users(id)
);
"""

        # Projects
        sql += f"""CREATE TABLE projects (
  {self.make_id_col(dialect)},
  workspace_id BIGINT NOT NULL,
  name VARCHAR(255) NOT NULL,
  slug VARCHAR(100),
  description TEXT,
  status VARCHAR(50) DEFAULT 'active',
  visibility VARCHAR(50) DEFAULT 'private',
  created_by_id BIGINT,
  archived_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
  FOREIGN KEY (created_by_id) REFERENCES users(id)
);
"""
        for i in range(20):
            workspace_id = random.randint(1, 20)
            creator_id = random.choice(self.user_ids)
            sql += f"INSERT INTO projects (workspace_id, name, description, created_by_id, status) VALUES ({workspace_id}, 'Project {random_string(8)}', 'Project description', {creator_id}, 'active');\n"

        self.table_count += 3
        return sql

    def generate_events_tables(self, dialect: str) -> str:
        sql = "-- Events: Tracking, Engagement, Conversions\n\n"

        # Events
        sql += f"""CREATE TABLE events (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  user_id BIGINT,
  event_type VARCHAR(100) NOT NULL,
  event_name VARCHAR(255),
  properties {self.make_json_col(dialect)},
  timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  session_id VARCHAR(255),
  ip_address VARCHAR(45),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
"""
        # Insert events
        for i in range(500):
            org_id = random.choice(self.org_ids)
            user_id = random.choice(self.user_ids)
            event_type = random.choice(['page_view', 'click', 'submit', 'error', 'conversion', 'signup'])
            timestamp = random_date()
            sql += f"INSERT INTO events (organization_id, user_id, event_type, event_name, timestamp) VALUES ({org_id}, {user_id}, '{event_type}', 'Event {random_string(6)}', '{timestamp}');\n"

        # Page Views
        sql += f"""CREATE TABLE page_views (
  {self.make_id_col(dialect)},
  event_id BIGINT,
  user_id BIGINT NOT NULL,
  organization_id BIGINT NOT NULL,
  url VARCHAR(2000),
  page_title VARCHAR(500),
  referrer VARCHAR(2000),
  duration_ms INT,
  viewed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (event_id) REFERENCES events(id),
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        for i in range(200):
            user_id = random.choice(self.user_ids)
            org_id = self.org_ids[(user_id - 1) % len(self.org_ids)]
            sql += f"INSERT INTO page_views (user_id, organization_id, url, page_title, duration_ms) VALUES ({user_id}, {org_id}, '/page/{random_string(10)}', 'Page Title', {random.randint(100, 60000)});\n"

        # Conversions
        sql += f"""CREATE TABLE conversions (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  conversion_type VARCHAR(100),
  amount DECIMAL(12, 2),
  currency VARCHAR(3),
  converted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
"""
        for i in range(100):
            org_id = random.choice(self.org_ids)
            user_id = random.choice(self.user_ids)
            sql += f"INSERT INTO conversions (organization_id, user_id, conversion_type, amount, currency) VALUES ({org_id}, {user_id}, 'purchase', {random.randint(10, 1000)}.99, 'USD');\n"

        self.table_count += 3
        return sql

    def generate_content_tables(self, dialect: str) -> str:
        sql = "-- Content: Documents, Templates, Pages\n\n"

        # Documents
        sql += f"""CREATE TABLE documents (
  {self.make_id_col(dialect)},
  workspace_id BIGINT NOT NULL,
  title VARCHAR(500) NOT NULL,
  slug VARCHAR(255),
  content TEXT,
  large_content TEXT,
  metadata {self.make_json_col(dialect)},
  created_by_id BIGINT,
  updated_by_id BIGINT,
  view_count INT DEFAULT 0,
  is_published BOOLEAN DEFAULT FALSE,
  published_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
  FOREIGN KEY (created_by_id) REFERENCES users(id),
  FOREIGN KEY (updated_by_id) REFERENCES users(id)
);
"""
        for i in range(50):
            workspace_id = random.randint(1, 20)
            creator_id = random.choice(self.user_ids)
            sql += f"INSERT INTO documents (workspace_id, title, slug, created_by_id, view_count) VALUES ({workspace_id}, 'Document {random_string(10)}', 'doc-{random_string(8)}', {creator_id}, {random.randint(0, 1000)});\n"

        # Templates
        sql += f"""CREATE TABLE templates (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  content TEXT,
  template_schema {self.make_json_col(dialect)},
  is_system BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
INSERT INTO templates (organization_id, name, description, is_system) VALUES
(1, 'Welcome Email', 'Standard welcome template', TRUE),
(1, 'Report Template', 'Standard report layout', TRUE),
(1, 'Invoice Template', 'Invoice template', FALSE);
"""

        # Pages (empty table to test empty views)
        sql += f"""CREATE TABLE pages (
  {self.make_id_col(dialect)},
  workspace_id BIGINT NOT NULL,
  title VARCHAR(500),
  slug VARCHAR(255),
  content TEXT,
  status VARCHAR(50),
  created_at TIMESTAMP,
  FOREIGN KEY (workspace_id) REFERENCES workspaces(id)
);
"""
        # Intentionally leave empty

        # Articles
        sql += f"""CREATE TABLE articles (
  {self.make_id_col(dialect)},
  workspace_id BIGINT NOT NULL,
  author_id BIGINT,
  title VARCHAR(500) NOT NULL,
  slug VARCHAR(255) UNIQUE NOT NULL,
  excerpt TEXT,
  content TEXT,
  featured_image_url VARCHAR(2000),
  view_count INT DEFAULT 0,
  comment_count INT DEFAULT 0,
  published_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
  FOREIGN KEY (author_id) REFERENCES users(id)
);
"""
        for i in range(30):
            workspace_id = random.randint(1, 20)
            author_id = random.choice(self.user_ids)
            sql += f"INSERT INTO articles (workspace_id, author_id, title, slug, view_count) VALUES ({workspace_id}, {author_id}, 'Article {random_string(10)}', 'article-{random_string(8)}', {random.randint(0, 5000)});\n"

        self.table_count += 4
        return sql

    def generate_analytics_tables(self, dialect: str) -> str:
        sql = "-- Analytics: Metrics, Reports, Snapshots\n\n"

        # Analytics Events (large data)
        sql += f"""CREATE TABLE analytics_events (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  dataset_id BIGINT,
  event_data {self.make_json_col(dialect)},
  occurred_at TIMESTAMP,
  ingested_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        for i in range(100):
            org_id = random.choice(self.org_ids)
            sql += f"INSERT INTO analytics_events (organization_id, event_data, occurred_at) VALUES ({org_id}, '{random_json_small()}', '{random_date()}');\n"

        # Analytics Snapshots (very large JSON, ~250KB)
        sql += f"""CREATE TABLE analytics_snapshots (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  dataset_id BIGINT,
  snapshot_date DATE,
  data_payload {self.make_json_col(dialect)},
  row_count INT,
  file_size_bytes BIGINT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        for i in range(10):
            org_id = random.choice(self.org_ids)
            sql += f"INSERT INTO analytics_snapshots (organization_id, snapshot_date, data_payload, row_count, file_size_bytes) VALUES ({org_id}, CAST('{random_date()}' AS DATE), '{random_json_huge()}', {random.randint(1000, 100000)}, {random.randint(100000, 500000)});\n"

        # Metrics
        sql += f"""CREATE TABLE metrics (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  workspace_id BIGINT,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  query TEXT,
  metric_type VARCHAR(100),
  unit VARCHAR(50),
  metric_value DECIMAL(20, 2),
  last_updated TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (workspace_id) REFERENCES workspaces(id)
);
"""
        for i in range(25):
            org_id = random.choice(self.org_ids)
            workspace_id = random.randint(1, 20)
            sql += f"INSERT INTO metrics (organization_id, workspace_id, name, metric_type, unit, metric_value) VALUES ({org_id}, {workspace_id}, 'Metric {random_string(8)}', 'gauge', 'count', {random.randint(0, 10000)}.5);\n"

        # Reports
        sql += f"""CREATE TABLE reports (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  workspace_id BIGINT,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  report_config {self.make_json_col(dialect)},
  created_by_id BIGINT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
  FOREIGN KEY (created_by_id) REFERENCES users(id)
);
"""
        for i in range(15):
            org_id = random.choice(self.org_ids)
            workspace_id = random.randint(1, 20)
            creator_id = random.choice(self.user_ids)
            sql += f"INSERT INTO reports (organization_id, workspace_id, name, created_by_id) VALUES ({org_id}, {workspace_id}, 'Report {random_string(8)}', {creator_id});\n"

        # Report Results (empty table)
        sql += f"""CREATE TABLE report_results (
  {self.make_id_col(dialect)},
  report_id BIGINT NOT NULL,
  result_data {self.make_json_col(dialect)},
  generated_at TIMESTAMP,
  FOREIGN KEY (report_id) REFERENCES reports(id)
);
"""
        # Intentionally leave empty

        self.table_count += 5
        return sql

    def generate_configuration_tables(self, dialect: str) -> str:
        sql = "-- Configuration: Settings, Integrations, Webhooks\n\n"

        # Settings (many columns to test wide tables)
        sql += f"""CREATE TABLE settings (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  setting_key VARCHAR(255) NOT NULL,
  setting_value TEXT,
  display_name VARCHAR(255),
  description TEXT,
  setting_type VARCHAR(50),
  is_encrypted BOOLEAN DEFAULT FALSE,
  is_required BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(organization_id, setting_key),
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        settings_list = [
            ('email_enabled', 'Email enabled'),
            ('sso_enabled', 'SSO enabled'),
            ('data_retention_days', 'Data retention'),
            ('max_users', 'Max users'),
            ('api_rate_limit', 'API rate limit'),
            ('backup_frequency', 'Backup frequency'),
            ('timezone', 'Timezone'),
            ('currency', 'Currency'),
            ('language', 'Language'),
            ('theme', 'Theme'),
        ]
        for org_id in self.org_ids:
            for key, display in settings_list:
                sql += f"INSERT INTO settings (organization_id, setting_key, display_name) VALUES ({org_id}, '{key}', '{display}');\n"

        # Integrations
        sql += f"""CREATE TABLE integrations (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  integration_type VARCHAR(100) NOT NULL,
  name VARCHAR(255),
  description TEXT,
  config {self.make_json_col(dialect)},
  credentials {self.make_json_col(dialect)},
  status VARCHAR(50) DEFAULT 'active',
  last_synced_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        integration_types = ['slack', 'salesforce', 'hubspot', 'stripe', 'github', 'gitlab', 'jira', 'asana']
        integration_id = 1
        for org_id in self.org_ids:
            for int_type in random.sample(integration_types, k=random.randint(1, 4)):
                sql += f"INSERT INTO integrations (organization_id, integration_type, name, status) VALUES ({org_id}, '{int_type}', '{int_type} Integration', 'active');\n"
                self.integration_ids.append(integration_id)
                integration_id += 1

        # Webhooks
        sql += f"""CREATE TABLE webhooks (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  integration_id BIGINT,
  event_type VARCHAR(100),
  url VARCHAR(2000),
  secret_token VARCHAR(255),
  headers {self.make_json_col(dialect)},
  is_active BOOLEAN DEFAULT TRUE,
  retry_count INT DEFAULT 0,
  last_triggered_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (integration_id) REFERENCES integrations(id)
);
"""
        for i in range(20):
            org_id = random.choice(self.org_ids)
            integration_id = random.choice(self.integration_ids) if self.integration_ids else None
            if integration_id:
                sql += f"INSERT INTO webhooks (organization_id, integration_id, event_type, url) VALUES ({org_id}, {integration_id}, 'user.created', 'https://example.com/webhook');\n"

        # Feature Flags
        sql += f"""CREATE TABLE feature_flags (
  {self.make_id_col(dialect)},
  organization_id BIGINT,
  flag_name VARCHAR(255) NOT NULL,
  flag_key VARCHAR(100) NOT NULL,
  description TEXT,
  enabled BOOLEAN DEFAULT FALSE,
  rollout_percentage INT DEFAULT 0,
  target_users {self.make_json_col(dialect)},
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(flag_key, organization_id)
);
INSERT INTO feature_flags (flag_key, flag_name, enabled, rollout_percentage) VALUES
('dark_mode', 'Dark Mode', TRUE, 100),
('new_dashboard', 'New Dashboard', FALSE, 25),
('export_csv', 'CSV Export', TRUE, 100),
('ai_assistant', 'AI Assistant', FALSE, 50);
"""

        self.table_count += 5
        return sql

    def generate_audit_tables(self, dialect: str) -> str:
        sql = "-- Audit: Logs, Changes, Activity\n\n"

        # Audit Logs
        sql += f"""CREATE TABLE audit_logs (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  user_id BIGINT,
  action_type VARCHAR(100),
  resource_type VARCHAR(100),
  resource_id VARCHAR(255),
  changes {self.make_json_col(dialect)},
  ip_address VARCHAR(45),
  user_agent TEXT,
  timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
"""
        for i in range(200):
            org_id = random.choice(self.org_ids)
            user_id = random.choice(self.user_ids)
            action = random.choice(['create', 'update', 'delete', 'view', 'export'])
            sql += f"INSERT INTO audit_logs (organization_id, user_id, action_type, resource_type, timestamp) VALUES ({org_id}, {user_id}, '{action}', 'document', '{random_date()}');\n"

        # API Logs
        sql += f"""CREATE TABLE api_logs (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  user_id BIGINT,
  api_key_id BIGINT,
  endpoint VARCHAR(500),
  method VARCHAR(10),
  status_code INT,
  response_time_ms INT,
  request_body TEXT,
  response_body TEXT,
  error_message TEXT,
  ip_address VARCHAR(45),
  timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
"""
        for i in range(100):
            org_id = random.choice(self.org_ids)
            user_id = random.choice(self.user_ids)
            method = random.choice(['GET', 'POST', 'PUT', 'DELETE', 'PATCH'])
            status = random.choice([200, 201, 400, 401, 404, 500])
            sql += f"INSERT INTO api_logs (organization_id, user_id, endpoint, method, status_code, response_time_ms, timestamp) VALUES ({org_id}, {user_id}, '/api/v1/data', '{method}', {status}, {random.randint(10, 5000)}, '{random_date()}');\n"

        # Error Logs
        sql += f"""CREATE TABLE error_logs (
  {self.make_id_col(dialect)},
  organization_id BIGINT,
  error_code VARCHAR(50),
  error_message TEXT,
  error_details {self.make_json_col(dialect)},
  stack_trace TEXT,
  context {self.make_json_col(dialect)},
  severity VARCHAR(50),
  is_resolved BOOLEAN DEFAULT FALSE,
  timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        for i in range(50):
            org_id = random.choice(self.org_ids)
            severity = random.choice(['info', 'warning', 'error', 'critical'])
            sql += f"INSERT INTO error_logs (organization_id, error_code, error_message, severity, timestamp) VALUES ({org_id}, 'ERR_{random.randint(1000, 9999)}', 'Sample error message', '{severity}', '{random_date()}');\n"

        # Change Logs (empty table)
        sql += f"""CREATE TABLE change_logs (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  table_name VARCHAR(255),
  operation VARCHAR(20),
  old_values {self.make_json_col(dialect)},
  new_values {self.make_json_col(dialect)},
  changed_at TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        # Intentionally leave empty

        self.table_count += 4
        return sql

    def generate_integration_tables(self, dialect: str) -> str:
        sql = "-- Integration: API Keys, Connections, Sync Jobs\n\n"

        # API Keys
        sql += f"""CREATE TABLE api_keys (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  name VARCHAR(255),
  key_hash VARCHAR(255),
  scopes {self.make_json_col(dialect)},
  last_used_at TIMESTAMP,
  expires_at TIMESTAMP,
  is_revoked BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
"""
        for i in range(30):
            org_id = random.choice(self.org_ids)
            user_id = random.choice(self.user_ids)
            sql += f"INSERT INTO api_keys (organization_id, user_id, name) VALUES ({org_id}, {user_id}, 'API Key {random_string(8)}');\n"

        # Data Connections (wide table with many columns)
        sql += f"""CREATE TABLE data_connections (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  name VARCHAR(255) NOT NULL,
  connection_type VARCHAR(100),
  host VARCHAR(255),
  port INT,
  database_name VARCHAR(255),
  username VARCHAR(255),
  password_encrypted TEXT,
  ssl_enabled BOOLEAN DEFAULT FALSE,
  ssl_cert TEXT,
  ssl_key TEXT,
  ssl_ca TEXT,
  connection_timeout_seconds INT,
  query_timeout_seconds INT,
  pool_min_connections INT,
  pool_max_connections INT,
  auto_reconnect BOOLEAN DEFAULT TRUE,
  use_ssh_tunnel BOOLEAN DEFAULT FALSE,
  ssh_host VARCHAR(255),
  ssh_port INT,
  ssh_username VARCHAR(255),
  ssh_key_encrypted TEXT,
  custom_options {self.make_json_col(dialect)},
  status VARCHAR(50) DEFAULT 'active',
  last_tested_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        db_types = ['postgres', 'mysql', 'mongodb', 'redis', 'elasticsearch']
        for i in range(15):
            org_id = random.choice(self.org_ids)
            db_type = random.choice(db_types)
            sql += f"INSERT INTO data_connections (organization_id, name, connection_type, host, port, database_name, status) VALUES ({org_id}, '{db_type}_conn_{random_string(6)}', '{db_type}', 'localhost', {random.randint(3306, 5432)}, 'testdb', 'active');\n"

        # Sync Jobs
        sql += f"""CREATE TABLE sync_jobs (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  connection_id BIGINT NOT NULL,
  job_name VARCHAR(255),
  job_type VARCHAR(100),
  schedule_expression VARCHAR(255),
  last_run_at TIMESTAMP,
  next_run_at TIMESTAMP,
  status VARCHAR(50) DEFAULT 'scheduled',
  rows_synced INT,
  error_message TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (connection_id) REFERENCES data_connections(id)
);
"""
        for i in range(1, 16):
            for j in range(1, 3):
                org_id = random.choice(self.org_ids)
                status = random.choice(['scheduled', 'running', 'completed', 'failed'])
                sql += f"INSERT INTO sync_jobs (organization_id, connection_id, job_name, job_type, status) VALUES ({org_id}, {i}, 'Sync {random_string(6)}', 'incremental', '{status}');\n"

        # Sync Job Runs (empty table)
        sql += f"""CREATE TABLE sync_job_runs (
  {self.make_id_col(dialect)},
  sync_job_id BIGINT NOT NULL,
  started_at TIMESTAMP,
  completed_at TIMESTAMP,
  rows_processed INT,
  rows_inserted INT,
  rows_updated INT,
  duration_seconds INT,
  status VARCHAR(50),
  error_log TEXT,
  FOREIGN KEY (sync_job_id) REFERENCES sync_jobs(id)
);
"""
        # Intentionally leave empty

        self.table_count += 4
        return sql

    def generate_collaboration_tables(self, dialect: str) -> str:
        sql = "-- Collaboration: Comments, Mentions, Notifications\n\n"

        # Comments
        sql += f"""CREATE TABLE comments (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  resource_type VARCHAR(100),
  resource_id BIGINT,
  parent_comment_id BIGINT,
  content TEXT,
  metadata {self.make_json_col(dialect)},
  is_edited BOOLEAN DEFAULT FALSE,
  edited_at TIMESTAMP,
  is_deleted BOOLEAN DEFAULT FALSE,
  deleted_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (parent_comment_id) REFERENCES comments(id)
);
"""
        for i in range(100):
            org_id = random.choice(self.org_ids)
            user_id = random.choice(self.user_ids)
            sql += f"INSERT INTO comments (organization_id, user_id, resource_type, resource_id, content) VALUES ({org_id}, {user_id}, 'document', {random.randint(1, 50)}, 'Comment text here');\n"

        # Mentions
        sql += f"""CREATE TABLE mentions (
  {self.make_id_col(dialect)},
  comment_id BIGINT NOT NULL,
  mentioned_user_id BIGINT NOT NULL,
  mentioned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (comment_id) REFERENCES comments(id),
  FOREIGN KEY (mentioned_user_id) REFERENCES users(id)
);
"""
        for i in range(30):
            comment_id = random.randint(1, 100)
            user_id = random.choice(self.user_ids)
            sql += f"INSERT INTO mentions (comment_id, mentioned_user_id) VALUES ({comment_id}, {user_id});\n"

        # Notifications (composite FK)
        sql += f"""CREATE TABLE notifications (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  notification_type VARCHAR(100),
  actor_user_id BIGINT,
  resource_type VARCHAR(100),
  resource_id BIGINT,
  message TEXT,
  metadata {self.make_json_col(dialect)},
  is_read BOOLEAN DEFAULT FALSE,
  read_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (actor_user_id) REFERENCES users(id)
);
"""
        for i in range(150):
            org_id = random.choice(self.org_ids)
            user_id = random.choice(self.user_ids)
            actor_id = random.choice(self.user_ids)
            notif_type = random.choice(['comment', 'mention', 'assignment', 'review_request'])
            sql += f"INSERT INTO notifications (organization_id, user_id, notification_type, actor_user_id) VALUES ({org_id}, {user_id}, '{notif_type}', {actor_id});\n"

        # Watches (empty table)
        sql += f"""CREATE TABLE watches (
  {self.make_id_col(dialect)},
  user_id BIGINT NOT NULL,
  resource_type VARCHAR(100),
  resource_id BIGINT,
  created_at TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id)
);
"""
        # Intentionally leave empty

        self.table_count += 4
        return sql

    def generate_reporting_tables(self, dialect: str) -> str:
        sql = "-- Reporting: Dashboards, Visualizations, Schedules\n\n"

        # Dashboards
        sql += f"""CREATE TABLE dashboards (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  workspace_id BIGINT,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  layout_config {self.make_json_col(dialect)},
  created_by_id BIGINT,
  is_public BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
  FOREIGN KEY (created_by_id) REFERENCES users(id)
);
"""
        for i in range(20):
            org_id = random.choice(self.org_ids)
            workspace_id = random.randint(1, 20)
            creator_id = random.choice(self.user_ids)
            sql += f"INSERT INTO dashboards (organization_id, workspace_id, name, created_by_id) VALUES ({org_id}, {workspace_id}, 'Dashboard {random_string(8)}', {creator_id});\n"

        # Visualizations
        sql += f"""CREATE TABLE visualizations (
  {self.make_id_col(dialect)},
  dashboard_id BIGINT NOT NULL,
  title VARCHAR(255),
  description TEXT,
  visualization_type VARCHAR(100),
  chart_config {self.make_json_col(dialect)},
  data_query TEXT,
  refresh_interval_seconds INT,
  position_x INT,
  position_y INT,
  width INT,
  height INT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (dashboard_id) REFERENCES dashboards(id)
);
"""
        viz_types = ['line', 'bar', 'pie', 'scatter', 'table', 'gauge', 'heatmap']
        for i in range(50):
            dashboard_id = random.randint(1, 20)
            viz_type = random.choice(viz_types)
            sql += f"INSERT INTO visualizations (dashboard_id, title, visualization_type, position_x, position_y, width, height) VALUES ({dashboard_id}, 'Chart {random_string(6)}', '{viz_type}', {random.randint(0, 10)}, {random.randint(0, 10)}, {random.randint(2, 6)}, {random.randint(2, 4)});\n"

        # Scheduled Reports
        sql += f"""CREATE TABLE scheduled_reports (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  report_id BIGINT,
  recipient_email VARCHAR(255),
  schedule_cron VARCHAR(100),
  format VARCHAR(50),
  is_active BOOLEAN DEFAULT TRUE,
  last_sent_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (report_id) REFERENCES reports(id)
);
"""
        for i in range(10):
            org_id = random.choice(self.org_ids)
            report_id = random.randint(1, 15)
            sql += f"INSERT INTO scheduled_reports (organization_id, report_id, recipient_email, schedule_cron, format) VALUES ({org_id}, {report_id}, '{random_email()}', '0 9 * * MON', 'pdf');\n"

        # Export Jobs (empty table)
        sql += f"""CREATE TABLE export_jobs (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  resource_type VARCHAR(100),
  resource_id BIGINT,
  export_format VARCHAR(50),
  file_path TEXT,
  file_size_bytes BIGINT,
  status VARCHAR(50),
  requested_at TIMESTAMP,
  completed_at TIMESTAMP,
  expires_at TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
"""
        # Intentionally leave empty

        self.table_count += 4
        return sql


    def generate_products_tables(self, dialect: str) -> str:
        sql = "-- Products & Catalog: Products, Variants, Pricing\n\n"

        sql += f"""CREATE TABLE products (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  name VARCHAR(500) NOT NULL,
  slug VARCHAR(255) UNIQUE,
  description TEXT,
  long_description TEXT,
  category VARCHAR(100),
  sku VARCHAR(100) UNIQUE,
  status VARCHAR(50) DEFAULT 'active',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        for i in range(50):
            org_id = random.choice(self.org_ids)
            sql += f"INSERT INTO products (organization_id, name, slug, category, sku, status) VALUES ({org_id}, 'Product {random_string(10)}', 'prod-{random_string(8)}', 'category_{random.randint(1, 5)}', 'SKU-{random_string(6)}', 'active');\n"

        sql += f"""CREATE TABLE product_variants (
  {self.make_id_col(dialect)},
  product_id BIGINT NOT NULL,
  name VARCHAR(255),
  sku VARCHAR(100),
  attributes {self.make_json_col(dialect)},
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (product_id) REFERENCES products(id)
);
"""
        for i in range(100):
            product_id = random.randint(1, 50)
            sql += f"INSERT INTO product_variants (product_id, name, sku) VALUES ({product_id}, 'Variant {random_string(8)}', 'SKU-{random_string(6)}');\n"

        sql += f"""CREATE TABLE product_pricing (
  {self.make_id_col(dialect)},
  variant_id BIGINT NOT NULL,
  price DECIMAL(12, 2),
  cost DECIMAL(12, 2),
  currency VARCHAR(3),
  region VARCHAR(50),
  valid_from TIMESTAMP,
  valid_to TIMESTAMP,
  FOREIGN KEY (variant_id) REFERENCES product_variants(id)
);
"""

        sql += f"""CREATE TABLE product_images (
  {self.make_id_col(dialect)},
  product_id BIGINT NOT NULL,
  url VARCHAR(2000),
  alt_text VARCHAR(255),
  is_primary BOOLEAN DEFAULT FALSE,
  sort_order INT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (product_id) REFERENCES products(id)
);
"""
        for i in range(80):
            product_id = random.randint(1, 50)
            sql += f"INSERT INTO product_images (product_id, url, is_primary, sort_order) VALUES ({product_id}, 'https://example.com/img/{random_string(10)}.jpg', {random.choice(['true', 'false'])}, {random.randint(1, 5)});\n"

        # Empty table
        sql += f"""CREATE TABLE product_reviews (
  {self.make_id_col(dialect)},
  product_id BIGINT NOT NULL,
  user_id BIGINT,
  rating INT,
  title VARCHAR(255),
  content TEXT,
  created_at TIMESTAMP,
  FOREIGN KEY (product_id) REFERENCES products(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
"""

        self.table_count += 5
        return sql

    def generate_orders_tables(self, dialect: str) -> str:
        sql = "-- Orders & Transactions: Orders, Line Items, Payments\n\n"

        sql += f"""CREATE TABLE orders (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  order_number VARCHAR(100) UNIQUE,
  status VARCHAR(50) DEFAULT 'pending',
  total_amount DECIMAL(12, 2),
  currency VARCHAR(3),
  shipping_address TEXT,
  billing_address TEXT,
  notes TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
"""
        for i in range(150):
            org_id = random.choice(self.org_ids)
            user_id = random.choice(self.user_ids)
            status = random.choice(['pending', 'processing', 'shipped', 'delivered', 'cancelled'])
            sql += f"INSERT INTO orders (organization_id, user_id, order_number, status, total_amount, currency) VALUES ({org_id}, {user_id}, 'ORD-{random.randint(100000, 999999)}', '{status}', {random.randint(10, 1000)}.99, 'USD');\n"

        sql += f"""CREATE TABLE order_items (
  {self.make_id_col(dialect)},
  order_id BIGINT NOT NULL,
  variant_id BIGINT,
  quantity INT,
  unit_price DECIMAL(12, 2),
  total_price DECIMAL(12, 2),
  FOREIGN KEY (order_id) REFERENCES orders(id),
  FOREIGN KEY (variant_id) REFERENCES product_variants(id)
);
"""
        for i in range(300):
            order_id = random.randint(1, 150)
            variant_id = random.randint(1, 100)
            sql += f"INSERT INTO order_items (order_id, variant_id, quantity, unit_price) VALUES ({order_id}, {variant_id}, {random.randint(1, 5)}, {random.randint(10, 500)}.99);\n"

        sql += f"""CREATE TABLE payments (
  {self.make_id_col(dialect)},
  order_id BIGINT NOT NULL,
  payment_method VARCHAR(50),
  payment_status VARCHAR(50),
  amount DECIMAL(12, 2),
  transaction_id VARCHAR(255),
  processed_at TIMESTAMP,
  FOREIGN KEY (order_id) REFERENCES orders(id)
);
"""
        for i in range(100):
            order_id = random.randint(1, 150)
            sql += f"INSERT INTO payments (order_id, payment_method, payment_status, amount) VALUES ({order_id}, 'card', 'completed', {random.randint(10, 1000)}.99);\n"

        # Empty
        sql += f"""CREATE TABLE shipments (
  {self.make_id_col(dialect)},
  order_id BIGINT NOT NULL,
  carrier VARCHAR(100),
  tracking_number VARCHAR(255),
  shipped_at TIMESTAMP,
  delivered_at TIMESTAMP,
  FOREIGN KEY (order_id) REFERENCES orders(id)
);
"""

        self.table_count += 5
        return sql

    def generate_billing_tables(self, dialect: str) -> str:
        sql = "-- Billing: Subscriptions, Plans, Invoices\n\n"

        sql += f"""CREATE TABLE plans (
  {self.make_id_col(dialect)},
  organization_id BIGINT,
  name VARCHAR(100),
  description TEXT,
  price DECIMAL(12, 2),
  billing_interval VARCHAR(50),
  features {self.make_json_col(dialect)},
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO plans (name, description, price, billing_interval) VALUES
('Starter', 'Basic plan', '29.99', 'month'),
('Professional', 'Pro plan', '99.99', 'month'),
('Enterprise', 'Enterprise plan', '499.99', 'month');
"""

        sql += f"""CREATE TABLE subscriptions (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  plan_id BIGINT,
  user_id BIGINT,
  status VARCHAR(50) DEFAULT 'active',
  current_period_start TIMESTAMP,
  current_period_end TIMESTAMP,
  cancel_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (plan_id) REFERENCES plans(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
"""
        for i in range(50):
            org_id = random.choice(self.org_ids)
            user_id = random.choice(self.user_ids)
            plan_id = random.randint(1, 3)
            status = random.choice(['active', 'paused', 'cancelled'])
            sql += f"INSERT INTO subscriptions (organization_id, plan_id, user_id, status) VALUES ({org_id}, {plan_id}, {user_id}, '{status}');\n"

        sql += f"""CREATE TABLE invoices (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  subscription_id BIGINT,
  invoice_number VARCHAR(100),
  amount DECIMAL(12, 2),
  status VARCHAR(50),
  issued_at TIMESTAMP,
  due_at TIMESTAMP,
  paid_at TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (subscription_id) REFERENCES subscriptions(id)
);
"""
        for i in range(100):
            org_id = random.choice(self.org_ids)
            sub_id = random.randint(1, 50)
            status = random.choice(['draft', 'sent', 'viewed', 'paid', 'overdue'])
            sql += f"INSERT INTO invoices (organization_id, subscription_id, invoice_number, amount, status) VALUES ({org_id}, {sub_id}, 'INV-{random.randint(100000, 999999)}', {random.randint(10, 1000)}.99, '{status}');\n"

        # Empty
        sql += f"""CREATE TABLE invoice_items (
  {self.make_id_col(dialect)},
  invoice_id BIGINT NOT NULL,
  description VARCHAR(255),
  amount DECIMAL(12, 2),
  FOREIGN KEY (invoice_id) REFERENCES invoices(id)
);
"""

        self.table_count += 4
        return sql

    def generate_inventory_tables(self, dialect: str) -> str:
        sql = "-- Inventory: Stock Levels, Warehouses, Movements\n\n"

        sql += f"""CREATE TABLE warehouses (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  name VARCHAR(255),
  location VARCHAR(255),
  capacity INT,
  current_stock INT DEFAULT 0,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        for i in range(10):
            org_id = random.choice(self.org_ids)
            sql += f"INSERT INTO warehouses (organization_id, name, location, capacity) VALUES ({org_id}, 'Warehouse {random_string(6)}', 'Location {random.randint(1, 100)}', {random.randint(1000, 10000)});\n"

        sql += f"""CREATE TABLE stock_levels (
  {self.make_id_col(dialect)},
  warehouse_id BIGINT NOT NULL,
  variant_id BIGINT NOT NULL,
  quantity INT DEFAULT 0,
  reserved INT DEFAULT 0,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (warehouse_id) REFERENCES warehouses(id),
  FOREIGN KEY (variant_id) REFERENCES product_variants(id)
);
"""
        for i in range(200):
            warehouse_id = random.randint(1, 10)
            variant_id = random.randint(1, 100)
            sql += f"INSERT INTO stock_levels (warehouse_id, variant_id, quantity, reserved) VALUES ({warehouse_id}, {variant_id}, {random.randint(0, 1000)}, {random.randint(0, 100)});\n"

        sql += f"""CREATE TABLE stock_movements (
  {self.make_id_col(dialect)},
  warehouse_id BIGINT NOT NULL,
  variant_id BIGINT NOT NULL,
  movement_type VARCHAR(50),
  quantity_change INT,
  reference_id VARCHAR(255),
  notes TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (warehouse_id) REFERENCES warehouses(id),
  FOREIGN KEY (variant_id) REFERENCES product_variants(id)
);
"""
        for i in range(300):
            warehouse_id = random.randint(1, 10)
            variant_id = random.randint(1, 100)
            movement_type = random.choice(['in', 'out', 'adjustment', 'return'])
            sql += f"INSERT INTO stock_movements (warehouse_id, variant_id, movement_type, quantity_change) VALUES ({warehouse_id}, {variant_id}, '{movement_type}', {random.randint(-100, 100)});\n"

        # Empty
        sql += f"""CREATE TABLE stock_forecasts (
  {self.make_id_col(dialect)},
  warehouse_id BIGINT NOT NULL,
  variant_id BIGINT,
  forecast_date DATE,
  predicted_quantity INT,
  FOREIGN KEY (warehouse_id) REFERENCES warehouses(id),
  FOREIGN KEY (variant_id) REFERENCES product_variants(id)
);
"""

        self.table_count += 4
        return sql

    def generate_monitoring_tables(self, dialect: str) -> str:
        sql = "-- Monitoring: Performance, Uptime, Alerts\n\n"

        sql += f"""CREATE TABLE performance_metrics (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  metric_name VARCHAR(255),
  metric_val DECIMAL(20, 4),
  unit VARCHAR(50),
  tags {self.make_json_col(dialect)},
  recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        for i in range(200):
            org_id = random.choice(self.org_ids)
            metric = random.choice(['cpu', 'memory', 'disk', 'latency', 'requests_per_sec'])
            sql += f"INSERT INTO performance_metrics (organization_id, metric_name, metric_val, unit) VALUES ({org_id}, '{metric}', {random.uniform(0, 100)}, 'percent');\n"

        sql += f"""CREATE TABLE uptime_checks (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  service_name VARCHAR(255),
  endpoint VARCHAR(2000),
  status VARCHAR(50),
  response_time_ms INT,
  checked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        for i in range(100):
            org_id = random.choice(self.org_ids)
            status = random.choice(['up', 'down', 'degraded'])
            sql += f"INSERT INTO uptime_checks (organization_id, service_name, endpoint, status, response_time_ms) VALUES ({org_id}, 'Service {random_string(8)}', 'https://example.com/health', '{status}', {random.randint(10, 5000)});\n"

        sql += f"""CREATE TABLE alerts (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  alert_type VARCHAR(100),
  severity VARCHAR(50),
  message TEXT,
  is_resolved BOOLEAN DEFAULT FALSE,
  resolved_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        for i in range(50):
            org_id = random.choice(self.org_ids)
            severity = random.choice(['info', 'warning', 'critical'])
            sql += f"INSERT INTO alerts (organization_id, alert_type, severity, message) VALUES ({org_id}, 'perf_issue', '{severity}', 'Alert message');\n"

        # Empty
        sql += f"""CREATE TABLE alert_assignments (
  {self.make_id_col(dialect)},
  alert_id BIGINT NOT NULL,
  user_id BIGINT,
  assigned_at TIMESTAMP,
  FOREIGN KEY (alert_id) REFERENCES alerts(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
"""

        self.table_count += 4
        return sql

    def generate_custom_fields_tables(self, dialect: str) -> str:
        sql = "-- Custom Fields: Dynamic Schema Support\n\n"

        sql += f"""CREATE TABLE custom_field_definitions (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  resource_type VARCHAR(100),
  field_name VARCHAR(255),
  field_type VARCHAR(50),
  is_required BOOLEAN DEFAULT FALSE,
  validation_rules {self.make_json_col(dialect)},
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        for i in range(30):
            org_id = random.choice(self.org_ids)
            field_type = random.choice(['text', 'number', 'date', 'select', 'checkbox'])
            sql += f"INSERT INTO custom_field_definitions (organization_id, resource_type, field_name, field_type) VALUES ({org_id}, 'product', 'custom_field_{random_string(6)}', '{field_type}');\n"

        sql += f"""CREATE TABLE custom_field_values (
  {self.make_id_col(dialect)},
  field_id BIGINT NOT NULL,
  resource_type VARCHAR(100),
  resource_id BIGINT,
  field_value TEXT,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (field_id) REFERENCES custom_field_definitions(id)
);
"""

        sql += f"""CREATE TABLE entity_metadata (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  entity_type VARCHAR(100),
  entity_id BIGINT,
  metadata {self.make_json_col(dialect)},
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""

        self.table_count += 3
        return sql

    def generate_file_storage_tables(self, dialect: str) -> str:
        sql = "-- File Storage: Documents, Uploads, Storage\n\n"

        sql += f"""CREATE TABLE files (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  workspace_id BIGINT,
  name VARCHAR(500),
  mime_type VARCHAR(100),
  size_bytes BIGINT,
  storage_path VARCHAR(2000),
  uploaded_by_id BIGINT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
  FOREIGN KEY (uploaded_by_id) REFERENCES users(id)
);
"""
        for i in range(100):
            org_id = random.choice(self.org_ids)
            workspace_id = random.randint(1, 20)
            uploader_id = random.choice(self.user_ids)
            sql += f"INSERT INTO files (organization_id, workspace_id, name, mime_type, size_bytes, uploaded_by_id) VALUES ({org_id}, {workspace_id}, 'file_{random_string(10)}.pdf', 'application/pdf', {random.randint(1000, 10000000)}, {uploader_id});\n"

        sql += f"""CREATE TABLE file_versions (
  {self.make_id_col(dialect)},
  file_id BIGINT NOT NULL,
  version INT,
  size_bytes BIGINT,
  created_by_id BIGINT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (file_id) REFERENCES files(id),
  FOREIGN KEY (created_by_id) REFERENCES users(id)
);
"""

        sql += f"""CREATE TABLE file_permissions (
  {self.make_id_col(dialect)},
  file_id BIGINT NOT NULL,
  user_id BIGINT,
  permission VARCHAR(50),
  granted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (file_id) REFERENCES files(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
"""

        # Empty
        sql += f"""CREATE TABLE file_shares (
  {self.make_id_col(dialect)},
  file_id BIGINT NOT NULL,
  share_token VARCHAR(255),
  expires_at TIMESTAMP,
  created_at TIMESTAMP,
  FOREIGN KEY (file_id) REFERENCES files(id)
);
"""

        self.table_count += 4
        return sql

    def generate_usage_tables(self, dialect: str) -> str:
        sql = "-- Usage Tracking: Feature Usage, Quotas\n\n"

        sql += f"""CREATE TABLE feature_usage (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  user_id BIGINT,
  feature_name VARCHAR(255),
  usage_count INT DEFAULT 1,
  used_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
"""
        for i in range(200):
            org_id = random.choice(self.org_ids)
            user_id = random.choice(self.user_ids)
            feature = random.choice(['export', 'api_call', 'filter', 'report', 'visualization'])
            sql += f"INSERT INTO feature_usage (organization_id, user_id, feature_name, usage_count) VALUES ({org_id}, {user_id}, '{feature}', {random.randint(1, 100)});\n"

        sql += f"""CREATE TABLE usage_quotas (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  feature VARCHAR(100),
  quota_limit INT,
  period VARCHAR(50),
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        for i in range(20):
            org_id = random.choice(self.org_ids)
            sql += f"INSERT INTO usage_quotas (organization_id, feature, quota_limit, period) VALUES ({org_id}, 'api_calls', {random.randint(1000, 100000)}, 'month');\n"

        sql += f"""CREATE TABLE usage_billing (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  period_start DATE,
  period_end DATE,
  metered_usage {self.make_json_col(dialect)},
  calculated_cost DECIMAL(12, 2),
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""

        self.table_count += 3
        return sql

    def generate_support_tables(self, dialect: str) -> str:
        sql = "-- Support: Tickets, Feedback, Help\n\n"

        sql += f"""CREATE TABLE support_tickets (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  title VARCHAR(500),
  description TEXT,
  status VARCHAR(50) DEFAULT 'open',
  priority VARCHAR(50) DEFAULT 'normal',
  assigned_to_id BIGINT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  resolved_at TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (assigned_to_id) REFERENCES users(id)
);
"""
        for i in range(50):
            org_id = random.choice(self.org_ids)
            user_id = random.choice(self.user_ids)
            status = random.choice(['open', 'in_progress', 'resolved', 'closed'])
            sql += f"INSERT INTO support_tickets (organization_id, user_id, title, status, priority) VALUES ({org_id}, {user_id}, 'Ticket {random_string(10)}', '{status}', 'normal');\n"

        sql += f"""CREATE TABLE ticket_messages (
  {self.make_id_col(dialect)},
  ticket_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  message TEXT,
  is_internal BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (ticket_id) REFERENCES support_tickets(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
"""

        sql += f"""CREATE TABLE user_feedback (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  user_id BIGINT,
  feedback_type VARCHAR(50),
  rating INT,
  message TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
"""
        for i in range(80):
            org_id = random.choice(self.org_ids)
            user_id = random.choice(self.user_ids)
            rating = random.randint(1, 5)
            sql += f"INSERT INTO user_feedback (organization_id, user_id, feedback_type, rating) VALUES ({org_id}, {user_id}, 'general', {rating});\n"

        # Empty
        sql += f"""CREATE TABLE help_articles (
  {self.make_id_col(dialect)},
  organization_id BIGINT,
  title VARCHAR(500),
  slug VARCHAR(255),
  content TEXT,
  category VARCHAR(100),
  view_count INT DEFAULT 0,
  created_at TIMESTAMP
);
"""

        self.table_count += 4
        return sql

    def generate_performance_tables(self, dialect: str) -> str:
        sql = "-- Performance: Benchmarks, Profiling, Optimization\n\n"

        sql += f"""CREATE TABLE query_performance (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  query_hash VARCHAR(255),
  query_text TEXT,
  execution_time_ms DECIMAL(10, 2),
  rows_affected INT,
  executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        for i in range(150):
            org_id = random.choice(self.org_ids)
            sql += f"INSERT INTO query_performance (organization_id, query_hash, query_text, execution_time_ms, rows_affected) VALUES ({org_id}, 'HASH_{random_string(10)}', 'SELECT * FROM table', {random.uniform(1, 1000)}, {random.randint(0, 10000)});\n"

        sql += f"""CREATE TABLE endpoint_performance (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  endpoint VARCHAR(500),
  method VARCHAR(10),
  response_time_ms INT,
  status_code INT,
  recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""
        for i in range(150):
            org_id = random.choice(self.org_ids)
            method = random.choice(['GET', 'POST', 'PUT', 'DELETE'])
            sql += f"INSERT INTO endpoint_performance (organization_id, endpoint, method, response_time_ms, status_code) VALUES ({org_id}, '/api/endpoint', '{method}', {random.randint(10, 5000)}, 200);\n"

        sql += f"""CREATE TABLE resource_usage (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  resource_type VARCHAR(100),
  resource_id BIGINT,
  cpu_usage DECIMAL(5, 2),
  memory_usage DECIMAL(5, 2),
  disk_usage DECIMAL(10, 2),
  recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""

        # Empty
        sql += f"""CREATE TABLE optimization_suggestions (
  {self.make_id_col(dialect)},
  organization_id BIGINT NOT NULL,
  category VARCHAR(100),
  suggestion TEXT,
  impact_estimate VARCHAR(50),
  created_at TIMESTAMP,
  FOREIGN KEY (organization_id) REFERENCES organizations(id)
);
"""

        self.table_count += 4
        return sql


def main():
    import sys

    gen = DatabaseGenerator()

    if len(sys.argv) > 1 and sys.argv[1] == 'mysql':
        print(gen.generate_mysql())
    else:
        print(gen.generate_postgres())


if __name__ == '__main__':
    main()
