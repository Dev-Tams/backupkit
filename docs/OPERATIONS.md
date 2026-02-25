# BackupKit Operations Runbook

This document is for operators running BackupKit in dev/staging/production environments.

## Scope

- Running one-off backups
- Running scheduled backups (`daemon`)
- Running restores safely
- Managing retention and notifications
- Basic incident response

## Preconditions

- BackupKit binary built and runnable
- PostgreSQL client tools installed on host:
  - `pg_dump`
  - `pg_restore`
  - `psql` (required only for SQL fallback restores)
- Valid `config.yaml`
- Required env vars exported before execution
- Target backup destination reachable:
  - Local path writable, and/or
  - S3 bucket credentials and network access

## Standard Operating Commands

Run immediate backup:

```bash
backupkit backup -c config.yaml --verbose
```

Run scheduler:

```bash
backupkit daemon -c config.yaml --verbose --run-timeout 45m
```

Run restore:

```bash
backupkit restore -c config.yaml --db app_db --from /path/to/backup.dump.gz.enc --verbose
```

Run restore into non-empty DB:

```bash
backupkit restore -c config.yaml --db app_db --from /path/to/backup.dump.gz.enc --clean --verbose
```

Allow SQL fallback restore:

```bash
backupkit restore -c config.yaml --db app_db --from /path/to/backup.sql --allow-sql-fallback --verbose
```

## Day-1 Deployment Checklist

1. Create per-environment config (`config.dev.yaml`, `config.staging.yaml`, `config.prod.yaml`).
2. Use environment variables for all secrets:
   - DB passwords
   - encryption key
   - S3 access keys
   - SMTP credentials
   - webhook URLs/tokens
3. Validate connectivity with a manual `backup` run.
4. Validate restore path using a non-production DB.
5. Enable notifications (`failure` at minimum).
6. Enable daemon with explicit `--run-timeout`.
7. Confirm backup files are written with expected extensions (`.dump`, `.gz`, `.enc`).
8. Verify retention settings with low counts first (example: `1/1/1` in staging).

## Backup Lifecycle

1. BackupKit invokes `pg_dump` for each configured database.
2. Optional transform pipeline runs:
   - gzip
   - AES-GCM encryption
3. Stream is written to storage key:
   - `<db-name>/<timestamp>.dump[.gz][.enc]`
4. Retention runs after each successful backup.
5. Notification routes are triggered on `success` or `failure`.

## Restore Safety Practices

1. Restore into isolated database first.
2. Only use `--clean` when replacing an existing schema intentionally.
3. Use `--strict-sniff` in controlled environments when pipeline mismatch must hard-fail.
4. Use `--allow-sql-fallback` only when source is expected to be plain SQL.
5. Record restore source file and timestamp in incident/change log.

## Daemon Operations

- Scheduler evaluates in UTC by minute.
- A run is triggered when cron spec matches the current UTC minute.
- Use `--run-timeout` to prevent long-running hung jobs.
- If timeout/cancel occurs, BackupKit still attempts to send failure notification.

Recommended service wrapper:
- systemd (preferred on Linux hosts)
- process manager of your platform (supervisor, container orchestrator, etc.)

## Monitoring and Alerts

Recommended minimum alerts:
- Failure notification route active (`email` and/or `webhook`)
- Missing expected backup files for each DB (external check)
- Daemon process liveness (external process monitor)

Recommended observability signals:
- Backup duration trend
- Backup size trend
- Restore drill success rate

## Incident Playbooks

### Playbook A: Backup Failure

1. Inspect command output and notification error field.
2. Confirm DB connectivity and credentials.
3. Confirm `pg_dump` exists in `PATH`.
4. Confirm destination writable/reachable.
5. Re-run manually with `--verbose`.
6. If destination partial file exists (local `.tmp`), confirm cleanup and rerun.

### Playbook B: Restore Failure

1. Verify backup file path and permissions.
2. Check decoded type assumptions:
   - custom dump needs `pg_restore`
   - SQL stream fallback needs `--allow-sql-fallback` and `psql`
3. If DB not empty errors occur, retry with `--clean` if appropriate.
4. If decrypt fails, verify encryption password.

### Playbook C: S3 Errors

1. Confirm bucket/region/prefix config.
2. Confirm `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` interpolation.
3. Validate IAM permissions for put/list/delete where required.
4. Re-run a single backup with `--verbose`.

## Operational Guardrails

- Never test restore only in production.
- Keep encryption password management external to repo.
- Rotate credentials regularly and update environment variables.
- Keep retention conservative until restore confidence is established.
- Treat backup success as incomplete until restore drill is verified.

## Periodic Tasks

Daily:
1. Check last successful backup per DB.
2. Review failure notifications.

Weekly:
1. Run at least one restore drill in non-prod.
2. Confirm retention is pruning as expected.

Monthly:
1. Rotate credentials where applicable.
2. Review storage growth and retention settings.
3. Verify notification endpoints and recipients.

## Version/Feature Caveats

- Current implementation supports PostgreSQL only.
- `init` and `test` CLI commands are currently placeholders.
- No built-in checksum verification command yet; perform integrity validation via restore drills.

