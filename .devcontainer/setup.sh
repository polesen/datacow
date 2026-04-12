#!/usr/bin/env bash
set -euo pipefail

echo "==> Installing Claude Code..."
npm install -g @anthropic-ai/claude-code

echo "==> Installing system packages..."
apt-get update -q && apt-get install -y -q \
  postgresql-client \
  default-mysql-client \
  iputils-ping \
  curl \
  python3-pip

echo "==> Installing uv (Python package runner for MCP servers)..."
curl -LsSf https://astral.sh/uv/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"

echo "==> Installing Go tools..."
go install golang.org/x/tools/gopls@latest
go install honnef.co/go/tools/cmd/staticcheck@latest
go install github.com/go-delve/delve/cmd/dlv@latest
go install gotest.tools/gotestsum@latest
go install mvdan.cc/gofumpt@latest
go install github.com/fatih/gomodifytags@latest

echo "==> Configuring Claude..."
printf '{
  "hasCompletedOnboarding": true,
  "theme": "dark",
  "projects": {
    "/workspace": {
      "hasTrustDialogAccepted": true,
      "hasClaudeMdExternalIncludesApproved": true
    }
  }
}' > ~/.claude.json

mkdir -p ~/.claude
cat > ~/.claude/settings.json << 'EOF'
{
  "skipDangerousModePermissionPrompt": true,
  "mcpServers": {
    "postgres": {
      "command": "uvx",
      "args": [
        "mcp-server-postgres",
        "--connection-string",
        "postgres://datacow:datacow@postgres:5432/datacow_test"
      ]
    },
    "context7": {
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp"]
    }
  }
}
EOF

echo "==> Versions:"
go version
gopls version
staticcheck --version
dlv version
gotestsum --version
claude --version

echo "==> Dev container ready"
