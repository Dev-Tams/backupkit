# BackupKit - Context Memory

## What This Is
You are building BackupKit, a Go-based CLI tool for automated database backups. This is Project 1 of 2 (Project 2 is RelayHook, a webhook relay service — separate context file). Both are being built linearly: BackupKit first, then RelayHook.

---

## Product Summary
- **Name:** BackupKit
- **Tagline:** "Dead-simple database backups that just work"
- **Type:** Open source CLI tool + optional managed cloud monitoring (BackupKit Cloud)
- **Language:** Go
- **Target:** Solo devs, small startups, freelance DevOps

---

## Target Personas
1. **Solo Developer** — 2-5 PostgreSQL DBs on Railway/Render/DigitalOcean, budget $0-20/mo, hasn't set up backups yet
2. **Startup CTO** — 5-15 databases, needs compliance-ready backups, budget $50-200/mo
3. **Freelance DevOps** — Manages 10+ clients, tired of maintaining custom scripts per client, budget $100-500/mo

---

## MVP Scope (Week 1-2 — what you are building now)

### Database Support (priority order)
1. PostgreSQL (via pg_dump)
2. MySQL/MariaDB (via mysqldump)
3. SQLite (native Go, no external binary)

### Storage Destinations (priority order)
1. AWS S3
2. Local filesystem
3. Backblaze B2 (week 2)
4. Google Cloud Storage (post-MVP)

### Core Features
- Cron-based scheduling (`"0 2 * * *"` or `@daily`, `@hourly`)
- Retention policies (keep_daily, keep_weekly, keep_monthly)
- Gzip compression (on by default)
- AES-256 encryption (optional, password from env var)
- Email notifications (SMTP) on success/failure
- Webhook notifications (POST JSON)
- Backup verification (checksum)
- Single binary distribution

### CLI Commands
```
backupkit init          → generates sample backup.yaml
backupkit test          → validates config, tests DB + storage connection
backupkit backup        → runs backup now
backupkit daemon        → runs in background on schedule
backupkit list          → lists stored backups
backupkit restore       → restores from a backup file/S3 path
backupkit verify        → verifies backup integrity
```

---

## Config Format (YAML)
```yaml
version: 1

databases:
  - name: production-db
    type: postgres          # postgres | mysql | sqlite
    connection:
      host: localhost
      port: 5432
      database: myapp
      user: postgres
      password: ${DB_PASSWORD}    # env var interpolation
    backup:
      schedule: "0 2 * * *"
      storage: s3-production      # references storage block below
      compression: true
      encryption:
        enabled: true
        password: ${BACKUP_ENCRYPTION_KEY}
    retention:
      keep_daily: 7
      keep_weekly: 4
      keep_monthly: 3

storage:
  - name: s3-production
    type: s3                # s3 | local | b2
    config:
      bucket: myapp-backups
      region: us-east-1
      prefix: production/
      access_key: ${AWS_ACCESS_KEY}
      secret_key: ${AWS_SECRET_KEY}

notifications:
  - type: email             # email | webhook
    on: [failure]           # success | failure | both
    config:
      smtp_host: smtp.gmail.com
      smtp_port: 587
      from: backups@myapp.com
      to: devops@myapp.com
      username: ${SMTP_USER}
      password: ${SMTP_PASSWORD}
```

---

## Project Structure
```
backupkit/
├── cmd/
│   └── backupkit/
│       └── main.go
├── internal/
│   ├── backup/
│   │   ├── backup.go          # Core orchestrator
│   │   ├── postgres.go        # pg_dump wrapper
│   │   ├── mysql.go           # mysqldump wrapper
│   │   └── sqlite.go          # Native Go SQLite backup
│   ├── storage/
│   │   ├── storage.go         # Storage interface
│   │   ├── s3.go              # AWS S3
│   │   ├── local.go           # Local filesystem
│   │   └── b2.go              # Backblaze B2
│   ├── compression/
│   │   └── gzip.go
│   ├── encryption/
│   │   └── aes.go
│   ├── scheduler/
│   │   └── cron.go
│   ├── retention/
│   │   └── policy.go
│   ├── notification/
│   │   ├── notification.go    # Notifier interface
│   │   ├── email.go
│   │   └── webhook.go
│   └── config/
│       └── config.go          # YAML parsing + env var interpolation
├── pkg/
│   └── backupkit/             # Public library API (optional)
├── examples/
│   └── backup.yaml
├── docs/
└── README.md
```

---

## Core Interfaces
```go
// internal/backup/backup.go
type Backupper interface {
    Backup(ctx context.Context, cfg DatabaseConfig) (io.Reader, error)
    Restore(ctx context.Context, cfg DatabaseConfig, backup io.Reader) error
}

// internal/storage/storage.go
type Storage interface {
    Upload(ctx context.Context, path string, data io.Reader) error
    Download(ctx context.Context, path string) (io.Reader, error)
    List(ctx context.Context, prefix string) ([]BackupMetadata, error)
    Delete(ctx context.Context, path string) error
}

// internal/notification/notification.go
type Notifier interface {
    Notify(ctx context.Context, event BackupEvent) error
}
```

---

## Go Dependencies
```
github.com/spf13/viper v1.18.2           → Config (YAML parsing)
github.com/robfig/cron/v3 v3.0.1         → Scheduling
github.com/aws/aws-sdk-go v1.49.0        → S3
github.com/lib/pq v1.10.9                → PostgreSQL driver
github.com/go-sql-driver/mysql v1.7.1    → MySQL driver
github.com/mattn/go-sqlite3 v1.14.19     → SQLite driver
github.com/urfave/cli/v2 v2.27.1         → CLI framework
go.uber.org/zap v1.26.0                  → Logging
gopkg.in/gomail.v2 v2.0.0-20160411212932 → Email (SMTP)
```

---

## Build Timeline
| Week | Focus | Deliverable |
|------|-------|-------------|
| 1 | Core engine | PostgreSQL backup → S3, basic CLI, YAML config, compression |
| 2 | Polish + launch | MySQL, SQLite, local storage, scheduling, retention, notifications, README, open source launch |
| 3 | Extras + Cloud start | B2 storage, encryption, verification, begin Cloud API |
| 4 | BackupKit Cloud | Monitoring API, web dashboard, Stripe billing, managed service launch |

---

## BackupKit Cloud (Managed Service — Week 3-4, not yet started)
- Users run BackupKit CLI locally, it reports metrics to BackupKit Cloud
- Cloud provides: monitoring dashboard, alerts, backup verification
- **Free:** 2 databases, 7-day retention, email alerts
- **Pro ($19/mo):** Unlimited DBs, 90-day retention, Slack/Discord, backup verification
- **Team ($49/mo):** 5 seats, SSO, audit logs
- **Enterprise:** Custom

---

## Pricing (Cloud)
| Tier | Price | Databases | Retention | Key Features |
|------|-------|-----------|-----------|--------------|
| Free | $0 | 2 | 7 days | Basic dashboard, email alerts |
| Pro | $19/mo | Unlimited | 90 days | Verification, Slack/Discord, trends |
| Team | $49/mo | Unlimited | 1 year | SSO, audit logs, 5 seats |
| Enterprise | Custom | Unlimited | Custom | On-prem, SLA, dedicated support |

---

## Go-to-Market
- Launch on: r/golang, r/selfhosted, r/PostgreSQL, Hacker News (Show HN)
- Blog post: "Why I built another backup tool"
- SEO targets: "PostgreSQL backup tutorial", "MySQL backup automation"
- Platform listings: Railway, Render, Fly.io
- Product Hunt launch with managed service

---

## Success Metrics
- Week 2: 50 GitHub stars
- Month 1: 200 stars, 10 active Discord users
- Month 3: 500 stars, 20 Pro cloud users ($380 MRR)
- Month 6: 1000 stars, 50 Pro + 5 Team ($1,195 MRR)
- Month 12: 2000 stars, 100 Pro + 15 Team ($2,635 MRR)

---

## What Comes After BackupKit
Project 2 is **RelayHook** — a webhook relay service. Separate context file. Starts Week 4 after BackupKit is launched and stabilized. Both products share Go patterns and are cross-promoted to each other's user bases.

---

## Current Status
- [ ] PRD written and approved
- [ ] No code written yet
- [ ] Ready to start Week 1: core PostgreSQL backup engine