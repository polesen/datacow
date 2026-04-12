#!/usr/bin/env bash
set -euo pipefail

echo "==> Installing Claude Code..."
npm install -g @anthropic-ai/claude-code

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
printf '{"skipDangerousModePermissionPrompt": true}' > ~/.claude/settings.json

echo "==> Versions:"
go version
gopls version
staticcheck --version
dlv version
gotestsum --version
claude --version

echo "==> Dev container ready"
