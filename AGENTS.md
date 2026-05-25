# AGENTS

This file defines engineering standards for contributors and coding agents working on this repository.

## Language and Runtime

- Use modern Go features and idioms supported by the Go version in go.mod.
- Prefer standard library packages over new external dependencies unless there is a clear, documented need.
- Write clear, explicit code with strong error handling.
- Keep APIs small and composable; avoid unnecessary abstractions.

## Testing and Coverage

- Aim for high test coverage across the project.
- Repository target: at least 85% total line coverage.
- Core package target (manifest parsing/validation, lockfile logic, command behavior): at least 90% line coverage.
- Every bug fix must include a regression test.
- Prefer table-driven tests for validation logic and edge cases.
- Test output should be deterministic and stable for CI.

Recommended local coverage command:

```bash
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

## Linting and Static Analysis

- Linting is strict: warnings should be treated as failures for CI and PR review.
- Required checks:
  - go fmt ./...
  - goimports -w .
  - go vet ./...
  - staticcheck ./...
- Keep cyclomatic complexity under control; functions should generally stay below gocyclo score 25.

## Pull Requests and Commits

- Keep commits focused and atomic.
- Ensure all required checks pass before merging.
- Update TODOs/docs when behavior or standards change.
