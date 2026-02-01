package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)



type Config struct {
	Version       int                 `yaml:"version"`
	Databases     []DatabaseConfig    `yaml:"databases"`
	Storage       []StorageConfig     `yaml:"storage"`
	Notifications []NotificationConfig `yaml:"notifications"`
}

type DatabaseConfig struct {
	Name       string          `yaml:"name"`
	Type       string          `yaml:"type"`
	Connection ConnectionConfig `yaml:"connection"`
	Backup     BackupConfig     `yaml:"backup"`
	Retention  RetentionConfig  `yaml:"retention"`
}

type ConnectionConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type BackupConfig struct {
	Schedule    string           `yaml:"schedule"`
	Storage     string           `yaml:"storage"`
	Compression bool             `yaml:"compression"`
	Encryption  EncryptionConfig `yaml:"encryption"`
}

type EncryptionConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Password string `yaml:"password"`
}

type RetentionConfig struct {
	KeepDaily   int `yaml:"keep_daily"`
	KeepWeekly  int `yaml:"keep_weekly"`
	KeepMonthly int `yaml:"keep_monthly"`
}

type StorageConfig struct {
	Name   string    `yaml:"name"`
	Type   string    `yaml:"type"`
	Config S3Config  `yaml:"config"`
}

type S3Config struct {
	Bucket    string `yaml:"bucket"`
	Region    string `yaml:"region"`
	Prefix    string `yaml:"prefix"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
}

type NotificationConfig struct {
	Type   string                 `yaml:"type"`
	On     []string               `yaml:"on"`
	Config NotificationDetails    `yaml:"config"`
}

type NotificationDetails struct {
	SMTPHost string            `yaml:"smtp_host"`
	SMTPPort int               `yaml:"smtp_port"`
	From     string            `yaml:"from"`
	To       string            `yaml:"to"`
	Username string            `yaml:"username"`
	Password string            `yaml:"password"`
	URL      string            `yaml:"url"`
	Headers  map[string]string `yaml:"headers"`
}


// Create a new Viper instance (don't use the global one — using a dedicated instance is cleaner and more testable)
// Tell it the config file path
// Tell it to read the file — and handle the error if reading fails
// Unmarshal into a Config struct — and handle that error too
// Return the config and nil for the error if everything worked

// // 

func LoadConfig(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	ModifyConfig(&cfg)

	return &cfg, nil
}

func ModifyConfig(cfg *Config) {
	for i := range cfg.Databases {
		db := &cfg.Databases[i]
		db.Name = os.ExpandEnv(db.Name)
		db.Type = os.ExpandEnv(db.Type)
		db.Connection.Host = os.ExpandEnv(db.Connection.Host)
		db.Connection.Database = os.ExpandEnv(db.Connection.Database)
		db.Connection.User = os.ExpandEnv(db.Connection.User)
		db.Connection.Password = os.ExpandEnv(db.Connection.Password)
		db.Backup.Schedule = os.ExpandEnv(db.Backup.Schedule)
		db.Backup.Storage = os.ExpandEnv(db.Backup.Storage)
		db.Backup.Encryption.Password = os.ExpandEnv(db.Backup.Encryption.Password)
	}

	for i := range cfg.Storage {
		st := &cfg.Storage[i]
		st.Name = os.ExpandEnv(st.Name)
		st.Type = os.ExpandEnv(st.Type)
		st.Config.Bucket = os.ExpandEnv(st.Config.Bucket)
		st.Config.Region = os.ExpandEnv(st.Config.Region)
		st.Config.Prefix = os.ExpandEnv(st.Config.Prefix)
		st.Config.AccessKey = os.ExpandEnv(st.Config.AccessKey)
		st.Config.SecretKey = os.ExpandEnv(st.Config.SecretKey)
	}

	for i := range cfg.Notifications {
		nt := &cfg.Notifications[i]
		nt.Type = os.ExpandEnv(nt.Type)
		for j := range nt.On {
			nt.On[j] = os.ExpandEnv(nt.On[j])
		}
		nt.Config.SMTPHost = os.ExpandEnv(nt.Config.SMTPHost)
		nt.Config.From = os.ExpandEnv(nt.Config.From)
		nt.Config.To = os.ExpandEnv(nt.Config.To)
		nt.Config.Username = os.ExpandEnv(nt.Config.Username)
		nt.Config.Password = os.ExpandEnv(nt.Config.Password)
		nt.Config.URL = os.ExpandEnv(nt.Config.URL)
		for k, v := range nt.Config.Headers {
			nt.Config.Headers[k] = os.ExpandEnv(v)
		}
	}
}
