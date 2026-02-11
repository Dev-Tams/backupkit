package storage

import (
	"context"
	"fmt"

	"github.com/dev-tams/backupkit/internal/config"
	"github.com/dev-tams/backupkit/internal/storage/local"
	s3store "github.com/dev-tams/backupkit/internal/storage/s3"
)

func FromConfig(ctx context.Context, cfg *config.Config) (map[string]Storage, error) {
	return FromConfigByNames(ctx, cfg, nil)
}

// FromConfigByNames builds only storage backends whose names are present in include.
// If include is nil, all configured backends are built.
func FromConfigByNames(ctx context.Context, cfg *config.Config, include map[string]struct{}) (map[string]Storage, error) {
	out := make(map[string]Storage, len(cfg.Storage))

	for _, st := range cfg.Storage {
		if include != nil {
			if _, ok := include[st.Name]; !ok {
				continue
			}
		}

		switch st.Type {
		case "local":
			if st.Local == nil || st.Local.Path == "" {
				return nil, fmt.Errorf("storage %s: local.path is required", st.Name)
			}
			out[st.Name] = local.New(st.Name, st.Local.Path)

		case "s3":
			if st.S3 == nil {
				return nil, fmt.Errorf("storage %s: s3 config missing", st.Name)
			}
			if st.S3.AccessKey == "" || st.S3.SecretKey == "" {
				return nil, fmt.Errorf("storage %s: s3.access_key and s3.secret_key are required (or env expansion failed)", st.Name)
			}
			s, err := s3store.New(ctx, s3store.Options{
				Name:      st.Name,
				Bucket:    st.S3.Bucket,
				Region:    st.S3.Region,
				Prefix:    st.S3.Prefix,
				AccessKey: st.S3.AccessKey,
				SecretKey: st.S3.SecretKey,
			})
			if err != nil {
				return nil, fmt.Errorf("storage %s: %w", st.Name, err)
			}
			out[st.Name] = s
		default:
			return nil, fmt.Errorf("storage %s: unknown type %q", st.Name, st.Type)
		}
	}

	return out, nil
}
