
npx @devcontainers/cli up --workspace-folder .

npx @devcontainers/cli exec --workspace-folder . \
  claude --dangerouslySkipPermissions \
  "Read CLAUDE.md, then complete the task described in $@. Verify all acceptance criteria are met before finishing."

