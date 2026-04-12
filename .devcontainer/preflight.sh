#!/usr/bin/env bash
# Preflight check — run inside the devcontainer before starting any task.
# Exits non-zero immediately if anything required is missing or unreachable.
set -euo pipefail

PASS=0
FAIL=0

check() {
  local label="$1"
  local cmd="$2"
  if eval "$cmd" &>/dev/null; then
    echo "  [ok] $label"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $label"
    FAIL=$((FAIL + 1))
  fi
}

echo "==> Binaries"
check "go"             "command -v go"
check "gopls"          "command -v gopls"
check "staticcheck"    "command -v staticcheck"
check "gotestsum"      "command -v gotestsum"
check "gofumpt"        "command -v gofumpt"
check "gomodifytags"   "command -v gomodifytags"
check "dlv"            "command -v dlv"
check "golangci-lint"  "command -v golangci-lint"
check "claude"         "command -v claude"
check "psql"           "command -v psql"
check "pg_isready"     "command -v pg_isready"
check "mysql"          "command -v mysql"
check "mysqladmin"     "command -v mysqladmin"
check "uvx"            "command -v uvx"
check "npx"            "command -v npx"

echo ""
echo "==> Databases"
check "postgres reachable" "pg_isready -h postgres -U datacow -q"
check "mysql reachable"    "mysqladmin ping -h mysql -u datacow -pdatacow --silent"

echo ""
echo "==> Build"
check "project compiles"   "go build ./..."

echo ""
if [ "$FAIL" -gt 0 ]; then
  echo "PREFLIGHT FAILED: $FAIL check(s) failed. Rebuild the devcontainer with: ./run-task.sh --rebuild <task>"
  exit 1
else
  echo "PREFLIGHT OK: all $PASS checks passed."
fi
