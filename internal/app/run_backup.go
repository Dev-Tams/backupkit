package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dev-tams/backupkit/internal/backup"
	"github.com/dev-tams/backupkit/internal/config"
)


// For now: the dump stream to a local file path like:
// ./backups/<dbName>/<timestamp>.dump

func RunBackup(ctx context.Context, cfg *config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	pg := backup.PostgresBackupper{}

	for _, db := range cfg.Databases {
		if db.Type != "postgress" {
			return fmt.Errorf("unsupported database type: %s {db: %s}", db.Name, db.Type)
		}
		r, err := pg.Backup(ctx, db)
		if err != nil {
			return fmt.Errorf("backup failed for %s: %w", db.Name, err)
		}
		defer r.Close()

		ts := time.Now().Format("20060102_150405")
		outDir := filepath.Join("backups", db.Name)
		if err := os.Mkdir(outDir, 0o755); err != nil {
			return fmt.Errorf("create outpuut dir : %w", err)
		}

		outPath := filepath.Join(outDir, ts + ".dump")
		f, err := os.Create(outPath)
		if err != nil{
			return fmt.Errorf(" create output file: %w", err)
		}
		n, copyErr := io.Copy(f, r)
		closeErr := f.Close()

		if copyErr != nil {
			return fmt.Errorf("write dump: %w", copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close file: %w", closeErr)
		}
		fmt.Printf("backup OK: db=%s bytes=%d file=%s\n", db.Name, n, outPath)
	}
	return nil
}
