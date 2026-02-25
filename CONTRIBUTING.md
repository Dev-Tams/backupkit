# Contributing to BackupKit

Thanks for contributing. This guide documents the expected development workflow for this repository.

## Ground Rules

- Keep changes focused and small.
- Prefer clear, testable behavior over speculative abstractions.
- Preserve current CLI behavior unless a change is intentional and documented.
- Avoid breaking config schema compatibility without explicit migration notes.

## Development Setup

Requirements:
- Go toolchain matching `go.mod`
- PostgreSQL client binaries on `PATH` for runtime/integration testing:
  - `pg_dump`
  - `pg_restore`
  - `psql` (for SQL fallback restore paths)

Clone and test:

```bash
go test ./...
```

Build:

```bash
go build -o bin/backupkit ./cmd/backupkit
```

Run locally:

```bash
go run ./cmd/backupkit --help
```

## Project Map

- `cmd/backupkit/main.go`: CLI wiring
- `internal/app`: backup/restore/daemon orchestration and retention
- `internal/backup`: DB dump integrations
- `internal/storage`: storage backends and interfaces
- `internal/notify`: notification dispatch and providers
- `internal/config`: config loading and validation
- `internal/schedule`: cron parsing/matching
- `internal/compression`, `internal/encryption`: stream transforms

## Making Changes

1. Create/update tests first when practical.
2. Implement minimal code change that satisfies behavior.
3. Run `go test ./...`.
4. Run `gofmt` on modified Go files.
5. Update docs when changing:
   - CLI behavior
   - config schema/validation
   - backup/restore pipeline semantics
   - operational expectations

## Testing Expectations

Minimum expectation for code changes:
- Unit tests for new helper logic and edge cases
- No regressions in existing tests

Strongly encouraged:
- Integration tests for storage/database interactions when behavior is non-trivial
- Restore-path tests for any pipeline or sniffing changes

## Style and Quality

- Use idiomatic Go.
- Return contextual errors with `%w`.
- Keep functions small and composable.
- Avoid global mutable state unless needed for test seams.
- Prefer explicit naming over comments for obvious logic.

## Config and Backward Compatibility

When editing config behavior:
- Preserve existing keys unless intentionally deprecating.
- Add validation changes carefully; stricter validation can be breaking.
- Document new required fields in `README.md`.

## Commit and PR Guidance

Recommended commit shape:
- One concern per commit where possible.
- Message format:
  - `app: fix restore tool selection for SQL fallback`
  - `notify: bound notification context on canceled backup`
  - `docs: add operations runbook`

PR checklist:
1. What changed
2. Why it changed
3. Risk assessment
4. Test evidence (`go test ./...` output summary)
5. Docs updated (if behavior/config changed)

## Areas That Need Contributions

- End-to-end integration coverage for backup/restore
- Additional storage/provider hardening
- Better operational commands (`init`, `test`) implementation
- Enhanced verification/integrity workflows

## Reporting Bugs

Include:
- BackupKit command used
- Redacted config relevant to issue
- Error output
- OS/runtime details
- Steps to reproduce

## Security Notes

- Do not commit real credentials, tokens, or database passwords.
- Use environment variables in config (`${VAR}` pattern).
- Redact sensitive values in issues and PRs.

