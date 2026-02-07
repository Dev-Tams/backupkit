package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Version       int                  `yaml:"version"`
	Databases     []DatabaseConfig     `yaml:"databases"`
	Storage       []StorageConfig      `yaml:"storage"`
	Notifications []NotificationConfig `yaml:"notifications"`
	Verbose       bool                 `yaml:"-" mapstructure:"-"`
}

type DatabaseConfig struct {
	Name       string           `yaml:"name"`
	Type       string           `yaml:"type"`
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
	Name   string   `yaml:"name"`
	Type   string   `yaml:"type"`
	Local  *LocalConfig `yaml:"local,omitempty"`
	S3     *S3Config      `yaml:"s3,omitempty"`
}


type LocalConfig struct {
	Path string `yaml:"path"`
}

type S3Config struct {
	Bucket    string `yaml:"bucket"`
	Region    string `yaml:"region"`
	Prefix    string `yaml:"prefix"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
}

type NotificationConfig struct {
	Type   string              `yaml:"type"`
	On     []string            `yaml:"on"`
	Config NotificationDetails `yaml:"config"`
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



// match the config file with the yaml file
func LoadConfig(path string) (*Config, error) {

	var cfg Config

	v := viper.New()
	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf(" failed to read config: %w", err)
	}

	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf(" failed to unmarshall config :%w", err)
	}
	expandEnvVars(&cfg)
	
	return &cfg, nil
}

// Loop through the slices where needed (databases, storage, notifications are all slices)
// Called os.ExpandEnv on each of the fields identified
// and modify the needed places.
func expandEnvVars(cfg *Config) {
    if cfg == nil {
        return
    }

    for i := range cfg.Databases {
        db := &cfg.Databases[i]
        db.Connection.Password = os.ExpandEnv(db.Connection.Password)
        db.Backup.Encryption.Password = os.ExpandEnv(db.Backup.Encryption.Password)
    }

    for i := range cfg.Storage {
        st := &cfg.Storage[i]
        if st.S3 != nil {
            st.S3.AccessKey = os.ExpandEnv(st.S3.AccessKey)
            st.S3.SecretKey = os.ExpandEnv(st.S3.SecretKey)
        }
    }

    for i := range cfg.Notifications {
        nt := &cfg.Notifications[i]
        if nt.Config.Username != "" {
            nt.Config.Username = os.ExpandEnv(nt.Config.Username)
        }
        if nt.Config.Password != "" {
            nt.Config.Password = os.ExpandEnv(nt.Config.Password)
        }
        if nt.Config.URL != "" {
            nt.Config.URL = os.ExpandEnv(nt.Config.URL)
        }
    }
}
