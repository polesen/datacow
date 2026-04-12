#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 <milestone>" >&2
  echo "Example: $0 M2-db-core.md" >&2
  exit 1
fi

TASK_FILE="TASKS/$1"

if [ ! -f "$TASK_FILE" ] || [ ! -r "$TASK_FILE" ]; then
  echo "Error: '$TASK_FILE' is not a readable file" >&2
  exit 1
fi

echo "Starting dev container..."
npx @devcontainers/cli up --workspace-folder .

echo "Running Claude on $TASK_FILE..."
npx @devcontainers/cli exec --workspace-folder . \
  claude --dangerously-skip-permissions \
  "Read CLAUDE.md, then complete the task described in $TASK_FILE. Verify all acceptance criteria are met before finishing."
