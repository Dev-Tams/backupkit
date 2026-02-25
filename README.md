# BackupKit

BackupKit is a Go CLI for PostgreSQL backups with optional compression, optional encryption, configurable retention, and success/failure notifications.

## Documentation

- Operations runbook: `docs/OPERATIONS.md`
- Contributor guide: `CONTRIBUTING.md`

Current storage backends:
- Local filesystem
- AWS S3

Current runtime commands:
- `backup`
- `restore`
- `daemon`

Also present in CLI (currently placeholders):
- `init`
- `test`

## What It Does

- Streams PostgreSQL dumps from `pg_dump` directly into storage.
- Supports backup pipeline transforms:
  - gzip compression (`.gz`)
  - AES-GCM encryption (`.enc`)
- Applies per-database retention policies (`keep_daily`, `keep_weekly`, `keep_monthly`).
- Sends notifications via:
  - webhook
  - SMTP email
- Restores backups with automatic payload sniffing:
  - `pg_restore` for custom-format dump streams
  - optional `psql` fallback for SQL text streams

## Requirements

- Go toolchain matching `go.mod` (module currently targets `go 1.25.5`)
- PostgreSQL client tools on `PATH`:
  - `pg_dump` for backup
  - `pg_restore` for restore of pg_dump custom format
  - `psql` only when using `--allow-sql-fallback`
- Access to configured destination:
  - writable local directory, and/or
  - AWS credentials + S3 bucket access

## Install and Build

Build binary:

```bash
go build -o bin/backupkit ./cmd/backupkit
```

Run directly without build:

```bash
go run ./cmd/backupkit --help
```

## Quick Start

1. Create a config file (`config.yaml`) using the example below.
2. Export secrets as environment variables.
3. Run a backup:

```bash
./bin/backupkit backup -c config.yaml --verbose
```

4. Restore a backup:

```bash
./bin/backupkit restore -c config.yaml --from /path/to/file.dump.gz.enc --verbose
```

5. Run scheduled backups in daemon mode:

```bash
./bin/backupkit daemon -c config.yaml --verbose --run-timeout 30m
```

## Configuration

### Full Example

```yaml
version: 1

storage:
  - name: local
    type: local
    local:
      path: "/absolute/path/to/backups"

  - name: s3main
    type: s3
    s3:
      bucket: "my-backup-bucket"
      region: "us-east-1"
      prefix: "backupkit"
      access_key: "${AWS_ACCESS_KEY_ID}"
      secret_key: "${AWS_SECRET_ACCESS_KEY}"

databases:
  - name: app_db
    type: postgres
    connection:
      host: "127.0.0.1"
      port: 5432
      database: "app"
      user: "appuser"
      password: "${DB_PASSWORD}"
    backup:
      schedule: "0 2 * * *"
      storage: "local"
      compression: true
      encryption:
        enabled: true
        password: "${BACKUPKIT_ENC_PASSWORD}"
    retention:
      keep_daily: 7
      keep_weekly: 4
      keep_monthly: 3

notifications:
  - type: webhook
    on: ["failure"]
    config:
      url: "${BACKUP_WEBHOOK_URL}"
      headers:
        Authorization: "Bearer ${BACKUP_WEBHOOK_TOKEN}"

  - type: email
    on: ["success", "failure"]
    config:
      smtp_host: "smtp.example.com"
      smtp_port: 587
      from: "backups@example.com"
      to: "dev1@example.com,dev2@example.com"
      username: "${SMTP_USER}"
      password: "${SMTP_PASSWORD}"
```

### Validation Rules (Important)

- `version` must be > 0.
- `storage[].name` must be unique.
- `storage[].type` must be either `local` or `s3`.
- For local storage:
  - `local.path` is required.
- For S3 storage:
  - `s3.bucket` and `s3.region` are required.
  - `s3.access_key` and `s3.secret_key` are required when backend is instantiated.
- `databases[].type` currently supports `postgres`.
- `databases[].backup.storage` must reference an existing storage name.
- `databases[].backup.schedule` may be empty or a valid 5-field cron expression.
- `notifications[].type` must be `webhook` or `email`.
- `notifications[].on` must include `success`, `failure`, or `both`.
- Email notifier requires:
  - `smtp_host`, `smtp_port`, `from`, `to`
  - if one of `username` / `password` is set, both must be set.

### Environment Variable Expansion

BackupKit expands environment variables for these fields:
- `databases[].connection.password`
- `databases[].backup.encryption.password`
- `storage[].s3.access_key`
- `storage[].s3.secret_key`
- `notifications[].config.username`
- `notifications[].config.password`
- `notifications[].config.url`

Use `${VAR_NAME}` syntax in YAML.

## CLI Usage

Global command flags for `backup`, `restore`, `daemon`:
- `-c, --config` path to config file (required)
- `--verbose` enable verbose output

### `backup`

Run immediate backup for all configured databases.

```bash
backupkit backup -c config.yaml --verbose
```

Output includes success/failure per DB and destination path.

### `restore`

Restore one backup file into a configured database.

```bash
backupkit restore -c config.yaml --from /path/to/backup.dump.gz.enc --db app_db --verbose
```

Flags:
- `--db` database name in config (optional; defaults to first database)
- `--from` local backup file path (required)
- `--clean` pass `--clean --if-exists` to `pg_restore`
- `--strict-sniff` fail if payload header mismatches expected config pipeline
- `--allow-sql-fallback` allow restore via `psql` when decoded stream looks like SQL text

### `daemon`

Runs forever and triggers backups whenever current UTC minute matches each DB cron schedule.

```bash
backupkit daemon -c config.yaml --run-timeout 45m --verbose
```

Flags:
- `--run-timeout` timeout per scheduled run; `0` disables timeout

If a run times out or is canceled, backup failure notification is still attempted using an internal bounded notification context.

### `init` and `test` (Current Status)

`init` and `test` are present in CLI but currently print placeholder text and return.

## Backup File Naming and Pipeline

For each database, backup keys are generated like:

`<db-name>/<timestamp>.dump[.gz][.enc]`

Timestamp format:
- `YYYYMMDD_HHMMSS.NNNNNNNNNZ` (UTC)

Transform order on backup:
1. `pg_dump` stream
2. gzip (optional)
3. AES-GCM encryption (optional)
4. write to storage

Reverse order on restore is detected from payload bytes, not only config.

## Retention Behavior

Retention is applied after each successful backup.

Selection strategy:
- Keeps newest backup per day up to `keep_daily`
- Keeps newest backup per ISO week up to `keep_weekly`
- Keeps newest backup per month up to `keep_monthly`

Notes:
- Retention requires prunable storage support (local and S3 implement this in this repo).
- Files with unrecognized timestamp pattern are skipped by retention logic.

## Notifications

Event payload fields:
- `db`
- `status` (`success` or `failure`)
- `bytes`
- `dest`
- `duration`
- `error` (present on failure)

Dispatch behavior:
- Multiple routes can be configured.
- Each route can subscribe to `success`, `failure`, or both.
- Route errors are aggregated.

## Developer Guide

### Repository Layout

- `cmd/backupkit/main.go`: CLI entrypoint and command wiring
- `internal/config`: config loading, env expansion, validation
- `internal/backup`: database backup streaming (PostgreSQL)
- `internal/app`: orchestration (`RunBackup`, `RunRestore`, `RunDaemon`, retention)
- `internal/compression`: gzip/gunzip helpers
- `internal/encryption`: AES-GCM stream framing/encryption
- `internal/storage`: storage interfaces + local/S3 implementations
- `internal/notify`: webhook/email notifiers + dispatcher
- `internal/schedule`: cron parser and matcher

### Common Dev Commands

Run all tests:

```bash
go test ./...
```

Run specific package tests:

```bash
go test ./internal/app -v
go test ./internal/config -v
```

Format code:

```bash
gofmt -w ./cmd ./internal
```

### Restore Tool Selection (Implementation Detail)

Restore determines tool at runtime:
- decoded stream kind `pgdmp` -> `pg_restore`
- decoded stream kind `sql` + `--allow-sql-fallback` -> `psql`

This avoids failing SQL fallback restores just because `pg_restore` is missing.

### Testing Notes

Current tests cover config validation, schedule parsing/matching, retention selection, and some backup/restore helper logic. There is room for additional integration coverage around:
- end-to-end backup/restore against real Postgres
- notifier integration tests
- storage integration tests

## Troubleshooting

### `pg_dump not found in PATH`

Install PostgreSQL client tools and ensure shell `PATH` includes them.

### Restore complains about header mismatch

- Use `--strict-sniff` only if you want hard failure on mismatch.
- Otherwise, BackupKit prints warnings and still attempts decode from actual stream bytes.

### SQL stream restore fails

Retry with:

```bash
backupkit restore -c config.yaml --from /path/to/file --allow-sql-fallback
```

Ensure `psql` is installed and on `PATH`.

### S3 auth/config errors

Verify:
- bucket/region are correct
- access/secret key values are resolved from env vars
- IAM permissions include object write/list/delete as required

## License

MIT (see `LICENSE`).
