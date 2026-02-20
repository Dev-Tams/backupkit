package config

import (
	"fmt"
	"strings"

	"github.com/dev-tams/backupkit/internal/schedule"
)

//simple range over values to validate needed variables

func (c *Config) Validate() error {
	if c.Version == 0 {
		return fmt.Errorf(" config.Version must be > 0")
	}
	storageNames := map[string]struct{}{}
	for _, st := range c.Storage {
		if st.Name == "" {
			return fmt.Errorf(" storage.Name is required")
		}
		if _, ok := storageNames[st.Name]; ok {
			return fmt.Errorf(" duplicate storage.Name")
		}
		storageNames[st.Name] = struct{}{}

		switch st.Type {
		case "local":
			if st.Local == nil || st.Local.Path == "" {
				return fmt.Errorf("storage %s: local.path is required", st.Name)
			}
			if st.S3 != nil {
				return fmt.Errorf("storage %s: type local must not set s3 config", st.Name)
			}
		case "s3":
			if st.S3 == nil || st.S3.Bucket == "" || st.S3.Region == "" {
				return fmt.Errorf("storage %s: s3.bucket and s3.region are required", st.Name)
			}
			if st.Local != nil {
				return fmt.Errorf("storage %s: type s3 must not set local config", st.Name)
			}

		default:
			return fmt.Errorf("storage %s: unknown type %q", st.Name, st.Type)
		}

	}

	for i, db := range c.Databases {
		if db.Name == "" {
			return fmt.Errorf("databases[%d].name is required", i)
		}

		if db.Type == "" {
			return fmt.Errorf("databases[%d].type is required (e.g. postgres)", i)
		}
		if db.Connection.Host == "" || db.Connection.Port == 0 || db.Connection.Database == "" || db.Connection.User == "" {
			return fmt.Errorf("databases[%d] connection is incomplete (host/port/database/user required)", i)
		}
		if db.Backup.Storage == "" {
			return fmt.Errorf("databases[%d] backup.storage is required (must match a storage.name)", i)
		}
		if _, ok := storageNames[db.Backup.Storage]; !ok {
			return fmt.Errorf("databases[%d] backup.storage=%q not found in storage list", i, db.Backup.Storage)
		}

		if s := strings.TrimSpace(db.Backup.Schedule); s != "" {
			if _, err := schedule.ParseCronSpec(s); err != nil {
				return fmt.Errorf("databases[%d] backup.schedule=%q is invalid: %w", i, db.Backup.Schedule, err)
			}
		}
	}

	for i, n := range c.Notifications {
		t := strings.ToLower(strings.TrimSpace(n.Type))
		if t == "" {
			return fmt.Errorf("notifications[%d].type is required", i)
		}

		onSuccess := false
		onFailure := false
		if len(n.On) == 0 {
			return fmt.Errorf("notifications[%d].on must include success, failure, or both", i)
		}
		for _, on := range n.On {
			switch strings.ToLower(strings.TrimSpace(on)) {
			case "success":
				onSuccess = true
			case "failure":
				onFailure = true
			case "both":
				onSuccess = true
				onFailure = true
			default:
				return fmt.Errorf("notifications[%d].on has unsupported value %q", i, on)
			}
		}
		if !onSuccess && !onFailure {
			return fmt.Errorf("notifications[%d].on must include success, failure, or both", i)
		}

		switch t {
		case "webhook":
			if strings.TrimSpace(n.Config.URL) == "" {
				return fmt.Errorf("notifications[%d] webhook config.url is required", i)
			}
		case "email":
			if strings.TrimSpace(n.Config.SMTPHost) == "" {
				return fmt.Errorf("notifications[%d] email config.smtp_host is required", i)
			}
			if n.Config.SMTPPort <= 0 {
				return fmt.Errorf("notifications[%d] email config.smtp_port must be > 0", i)
			}
			if strings.TrimSpace(n.Config.From) == "" {
				return fmt.Errorf("notifications[%d] email config.from is required", i)
			}
			if strings.TrimSpace(n.Config.To) == "" {
				return fmt.Errorf("notifications[%d] email config.to is required", i)
			}
			userSet := strings.TrimSpace(n.Config.Username) != ""
			passSet := strings.TrimSpace(n.Config.Password) != ""
			if userSet != passSet {
				return fmt.Errorf("notifications[%d] email config.username and config.password must be set together", i)
			}
		default:
			return fmt.Errorf("notifications[%d].type=%q is unsupported", i, n.Type)
		}
	}

	return nil
}
