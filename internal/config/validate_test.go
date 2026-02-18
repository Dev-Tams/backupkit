package config

import (
	"strings"
	"testing"
)

func baseValidConfig() *Config {
	return &Config{
		Version: 1,
		Storage: []StorageConfig{
			{
				Name: "local-main",
				Type: "local",
				Local: &LocalConfig{
					Path: "/tmp/backups",
				},
			},
		},
		Databases: []DatabaseConfig{
			{
				Name: "db1",
				Type: "postgres",
				Connection: ConnectionConfig{
					Host:     "127.0.0.1",
					Port:     5432,
					Database: "app",
					User:     "app",
				},
				Backup: BackupConfig{
					Storage:  "local-main",
					Schedule: "*/5 * * * *",
				},
			},
		},
	}
}

func TestValidateAcceptsValidSchedule(t *testing.T) {
	cfg := baseValidConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func TestValidateRejectsInvalidSchedule(t *testing.T) {
	cfg := baseValidConfig()
	cfg.Databases[0].Backup.Schedule = "61 * * * *"

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "backup.schedule") {
		t.Fatalf("expected backup.schedule error, got: %v", err)
	}
}

func TestValidateAllowsEmptySchedule(t *testing.T) {
	cfg := baseValidConfig()
	cfg.Databases[0].Backup.Schedule = ""

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error for empty schedule: %v", err)
	}
}
