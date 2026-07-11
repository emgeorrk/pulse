---
name: lint
description: Run golangci-lint on the project
disable-model-invocation: true
argument-hint: "[--fix]"
allowed-tools: Bash(golangci-lint *), Bash(make lint*)
---

Run the linter.

- If `$ARGUMENTS` contains `--fix`, run:
  ```bash
  make lint-fix
  ```
- Otherwise, run:
  ```bash
  make lint
  ```

If the linter reports violations, analyze the output and fix all issues found.
