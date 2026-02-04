package backup

import (
	"context"
	"io"

	"github.com/dev-tams/backupkit/internal/config"
)

// Backup returns a reader for the backup stream. The caller is responsible for
// consuming it and handling process lifecycle as needed.

type Backupper interface {
	Backup(ctx context.Context, cfg config.DatabaseConfig) (io.ReadCloser, error)
}
