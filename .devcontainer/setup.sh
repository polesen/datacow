#!/usr/bin/env bash
set -euo pipefail

echo "==> Installing system packages..."
sudo apt-get update -q && sudo apt-get install -y -q \
  postgresql-client \
  default-mysql-client \
  iputils-ping \
  curl \
  python3-pip

echo "==> Installing Claude Code..."
curl -fsSL https://claude.ai/install.sh | bash

echo "==> Installing uv (Python package runner for MCP servers)..."
curl -LsSf https://astral.sh/uv/install.sh | sh

# Persist ~/.local/bin in PATH for all future shell sessions in the container
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
export PATH="$HOME/.local/bin:$PATH"

echo "==> Installing Go tools..."
go install golang.org/x/tools/gopls@latest
go install honnef.co/go/tools/cmd/staticcheck@latest
go install github.com/go-delve/delve/cmd/dlv@latest
go install gotest.tools/gotestsum@latest
go install mvdan.cc/gofumpt@latest
go install github.com/fatih/gomodifytags@latest

echo "==> Configuring git identity..."
if [ -n "${GIT_AUTHOR_NAME:-}" ] && [ -n "${GIT_AUTHOR_EMAIL:-}" ]; then
  git config --global user.name "$GIT_AUTHOR_NAME"
  git config --global user.email "$GIT_AUTHOR_EMAIL"
else
  echo "  WARNING: GIT_AUTHOR_NAME or GIT_AUTHOR_EMAIL not set in .env.local — commits will have no author"
fi

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
uvx --version

echo "==> Dev container ready"
