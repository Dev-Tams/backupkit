package storage

import (
	"fmt"

	"github.com/dev-tams/backupkit/internal/config"
	"github.com/dev-tams/backupkit/internal/storage/local"
)

func FromConfig(cfg *config.Config) (map[string]Storage, error) {
	out := make(map[string]Storage, len(cfg.Storage))

	for _, st := range cfg.Storage {
		switch st.Type {
		case "local":
			if st.Local == nil || st.Local.Path == "" {
				return nil, fmt.Errorf("storage %s: local.path is required", st.Name)
			}
			out[st.Name] = local.New(st.Name, st.Local.Path)

		case "s3":
			return nil, fmt.Errorf("storage %s: s3 not implemented yet", st.Name)

		default:
			return nil, fmt.Errorf("storage %s: unknown type %q", st.Name, st.Type)
		}
	}

	return out, nil
}
