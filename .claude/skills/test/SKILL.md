---
name: test
description: Run unit tests for the project
disable-model-invocation: true
argument-hint: "[package path, e.g. ./internal/sensors/...]"
allowed-tools: Bash(go test *), Bash(make test*)
---

Run unit tests.

- If `$ARGUMENTS` is provided, run tests for that specific package:
  ```bash
  go test -v $ARGUMENTS
  ```
- If no arguments, run all tests:
  ```bash
  make test
  ```

If tests fail, analyze the output and report which tests failed and why.
