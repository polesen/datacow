#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 [--rebuild] [<task-file>]" >&2
  echo "Example: $0 tasks/ready/fuzzy-goto.md" >&2
  echo "         $0 ready/fuzzy-goto.md" >&2
  echo "         $0 --rebuild ready/fuzzy-goto.md" >&2
  echo "         $0                               # interactive: no task, no branch" >&2
}

REBUILD=""

while [[ "${1:-}" == --* ]]; do
  case "$1" in
    --rebuild) REBUILD="--remove-existing-container"; shift ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

if [ "$#" -gt 1 ]; then
  usage
  exit 1
fi

TASK_FILE=""
if [ "$#" -eq 1 ]; then
  # Accept either a full path or just a filename — normalise to a path under tasks/
  INPUT="$1"
  if [[ "$INPUT" == tasks/* || "$INPUT" == /* ]]; then
    TASK_FILE="$INPUT"
  else
    TASK_FILE="tasks/$INPUT"
  fi

  if [ ! -f "$TASK_FILE" ] || [ ! -r "$TASK_FILE" ]; then
    echo "Error: '$TASK_FILE' is not a readable file" >&2
    exit 1
  fi

  # Derive branch name from the bare filename, regardless of how the path was given
  BRANCH="task/$(basename "$TASK_FILE" .md)"

  # Create and switch to feature branch
  if git show-ref --verify --quiet "refs/heads/$BRANCH"; then
    echo "Branch '$BRANCH' already exists, switching to it..."
    git checkout "$BRANCH"
  else
    echo "Creating branch '$BRANCH'..."
    git checkout -b "$BRANCH"
  fi
fi

echo "Starting dev container..."
npx @devcontainers/cli up --workspace-folder . $REBUILD

echo "Running preflight checks..."
npx @devcontainers/cli exec --workspace-folder . bash .devcontainer/preflight.sh

if [ -n "$TASK_FILE" ]; then
  echo "Running Claude on $TASK_FILE (branch: $BRANCH)..."
  npx @devcontainers/cli exec --workspace-folder . \
    claude --dangerously-skip-permissions \
    "Read CLAUDE.md and tasks/definition-of-done.md, then complete the task described in $TASK_FILE. Verify all acceptance criteria in tasks/definition-of-done.md are met before finishing."

  echo ""
  echo "Pushing branch '$BRANCH'..."
  git push -u origin "$BRANCH"

  echo "Opening pull request..."
  TASK_NAME="$(basename "$TASK_FILE" .md)"
  gh pr create \
    --title "$TASK_NAME" \
    --body "Completes task: \`$TASK_FILE\`" \
    --head "$BRANCH"

  echo ""
  echo "Done. Review the PR above, run /simplify if needed, then merge to main."
else
  echo "Starting interactive Claude session..."
  npx @devcontainers/cli exec --workspace-folder . claude --dangerously-skip-permissions
fi
