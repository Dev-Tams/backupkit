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
//
func RunBackup(ctx context.Context, cfg *config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	pg := backup.PostgresBackupper{}

	for _, db := range cfg.Databases {
		if db.Type != "postgres" {
			return fmt.Errorf("unsupported database type: %s {db: %s}", db.Name, db.Type)
		}
		if cfg.Verbose {
			fmt.Printf(
				"pipeline: db=%s compression=%v encryption=%v\n",
				db.Name,
				db.Backup.Compression,
				db.Backup.Encryption.Enabled,
			)
		}
		r, err := pg.Backup(ctx, db)
		if err != nil {
			return fmt.Errorf("backup failed for %s: %w", db.Name, err)
		}

		ts := time.Now().Format("20060102_150405.000000000Z")
		outDir := filepath.Join("backups", db.Name)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			_ = r.Close()
			return fmt.Errorf("create output dir: %w", err)
		}

		outPath := filepath.Join(outDir, ts+".dump")

		// Build the pipeline
		stream := io.Reader(r)
		var cs closeStack
		// close any pipe readers we create (no defer)
		cs.closeAll()

		//compression
		if db.Backup.Compression {
			stream = gzipReader(stream, &cs)
			outPath += ".gz"
		}

		//encryption
		if db.Backup.Encryption.Enabled {
			stream = encryptReader(stream, db.Backup.Encryption.Password, &cs)
			outPath += ".enc"
		}

		f, err := os.Create(outPath)
		if err != nil {
			_ = r.Close()
			return fmt.Errorf("create output file: %w", err)
		}

		n, copyErr := io.Copy(f, stream)
		closeFileErr := f.Close()
		closeDumpErr := r.Close()

		if copyErr != nil {
			return fmt.Errorf("write backup: %w", copyErr)
		}
		if closeFileErr != nil {
			return fmt.Errorf("close output file: %w", closeFileErr)
		}
		if closeDumpErr != nil {
			return fmt.Errorf("close dump stream: %w", closeDumpErr)
		}

		fmt.Printf("backup OK: db=%s bytes=%d file=%s\n", db.Name, n, outPath)
	}

	return nil
}
