package app

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/dev-tams/backupkit/internal/backup"
	"github.com/dev-tams/backupkit/internal/config"
	"github.com/dev-tams/backupkit/internal/storage"
)

// For now: the dump stream to a local file path like:
// ./backups/<dbName>/<timestamp>.dump
func RunBackup(ctx context.Context, cfg *config.Config, verbose bool) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	usedStorage := make(map[string]struct{}, len(cfg.Databases))
	for _, db := range cfg.Databases {
		usedStorage[db.Backup.Storage] = struct{}{}
	}

	stores, err := storage.FromConfigByNames(ctx, cfg, usedStorage)
	if err != nil {
		return err
	}

	pg := backup.PostgresBackupper{}

	for _, db := range cfg.Databases {
		if db.Type != "postgres" {
			return fmt.Errorf("unsupported database type: %s {db: %s}", db.Name, db.Type)
		}

		st, ok := stores[db.Backup.Storage]
		if !ok {
			return fmt.Errorf("db %s: storage %q not found", db.Name, db.Backup.Storage)
		}

		if verbose {
			fmt.Printf(
				"pipeline: db=%s compression=%v encryption=%v storage=%s\n",
				db.Name,
				db.Backup.Compression,
				db.Backup.Encryption.Enabled,
				st.Name(),
			)
		}

		r, err := pg.Backup(ctx, db)
		if err != nil {
			return fmt.Errorf("backup failed for %s: %w", db.Name, err)
		}

		ts := time.Now().UTC().Format("20060102_150405.000000000Z")

		ext := ".dump"
		if db.Backup.Compression {
			ext += ".gz"
		}
		if db.Backup.Encryption.Enabled {
			ext += ".enc"
		}

		key := filepath.ToSlash(filepath.Join(db.Name, ts+ext))

		w, dest, err := st.OpenWriter(ctx, key)
		if err != nil {
			_ = r.Close()
			return fmt.Errorf("open storage writer: %w", err)
		}

		// Build the pipeline
		stream := io.Reader(r)
		var cs closeStack

		if db.Backup.Compression {
			stream = gzipReader(stream, &cs)
		}
		if db.Backup.Encryption.Enabled {
			stream = encryptReader(stream, db.Backup.Encryption.Password, &cs)
		}

		n, copyErr := io.Copy(w, stream)

		// close order matters
		cs.closeAll()
		closeDumpErr := r.Close()
		closeWriteErr := w.Close()

		if copyErr != nil {
			// local writer will leave .tmp if not closed successfully; we closed it above.
			return fmt.Errorf("write backup: %w", copyErr)
		}
		if closeDumpErr != nil {
			return fmt.Errorf("close dump stream: %w", closeDumpErr)
		}
		if closeWriteErr != nil {
			return fmt.Errorf("finalize storage write: %w", closeWriteErr)
		}

		fmt.Printf("backup OK: db=%s bytes=%d dest=%s\n", db.Name, n, dest)

		// after successful backup
		if err := ApplyRetention(ctx, db, st, verbose); err != nil {
			return fmt.Errorf("retention failed for %s: %w", db.Name, err)
		}

	}

	return nil
}
