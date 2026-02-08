package config

import "fmt"

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
			if st.S3.AccessKey == "" || st.S3.SecretKey == "" {
				return fmt.Errorf("storage %s: s3.access_key and s3.secret_key are required (or env expansion failed)", st.Name)
			}
			if st.S3.Bucket == "" || st.S3.Region == "" {
				return fmt.Errorf("storage %s: s3.bucket and s3.region are required", st.Name)
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
	}
	return nil
}
