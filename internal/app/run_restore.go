package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dev-tams/backupkit/internal/config"
)

func RunRestore(ctx context.Context, cfg *config.Config, dbName string, fromPath string, verbose bool) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	// pick db
	var db *config.DatabaseConfig
	if dbName == "" {
		if len(cfg.Databases) == 0 {
			return fmt.Errorf("no databases configured")
		}
		db = &cfg.Databases[0]
	} else {
		for i := range cfg.Databases {
			if cfg.Databases[i].Name == dbName {
				db = &cfg.Databases[i]
				break
			}
		}
		if db == nil {
			return fmt.Errorf("db %q not found in config", dbName)
		}
	}

	if db.Type != "postgres" {
		return fmt.Errorf("unsupported database type: %s {db: %s}", db.Name, db.Type)
	}

	if _, err := exec.LookPath("pg_restore"); err != nil {
		return fmt.Errorf("pg_restore not found in PATH: %w", err)
	}

	f, err := os.Open(fromPath)
	if err != nil {
		return fmt.Errorf("open backup file: %w", err)
	}
	defer f.Close()

	// Suffix mismatch is non-fatal; restore continues with a warning.
	expectedExt := expectedBackupExt(db.Backup.Compression, db.Backup.Encryption.Enabled)
	gotExt := backupSuffix(filepath.Base(fromPath))
	if gotExt != expectedExt {
		fmt.Fprintf(
			os.Stderr,
			"warning: backup suffix mismatch for db=%s: expected %q from config, got %q (%s)\n",
			db.Name,
			expectedExt,
			gotExt,
			fromPath,
		)
	}

	// reverse pipeline: decrypt -> gunzip
	stream := io.Reader(f)
	var cs closeStack

	if db.Backup.Encryption.Enabled {
		if db.Backup.Encryption.Password == "" {
			return fmt.Errorf("restore requires encryption password but it is empty (db %s)", db.Name)
		}
		stream = decryptReader(stream, db.Backup.Encryption.Password, &cs)
	}

	if db.Backup.Compression {
		stream = gunzipReader(stream, &cs)
	}

	conn := db.Connection

	if verbose {
		fmt.Printf(
			"restore pipeline: db=%s decrypt=%v gunzip=%v tool=pg_restore\n",
			db.Name,
			db.Backup.Encryption.Enabled,
			db.Backup.Compression,
		)
	}

	args := []string{
		"--host", conn.Host,
		"--port", strconv.Itoa(conn.Port),
		"--dbname", conn.Database,
		"--username", conn.User,
		"--format=custom",
		"--exit-on-error",
	}

	cmd := exec.CommandContext(ctx, "pg_restore", args...)

	// password env
	if conn.Password != "" {
		cmd.Env = append(os.Environ(), "PGPASSWORD="+conn.Password)
	} else {
		cmd.Env = os.Environ()
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("pg_restore stdin: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return fmt.Errorf("start pg_restore: %w", err)
	}

	_, copyErr := io.Copy(stdin, stream)
	_ = stdin.Close()

	// close pipeline readers (pipe readers) after streaming completes
	cs.closeAll()

	waitErr := cmd.Wait()

	if copyErr != nil {
		return fmt.Errorf("stream restore input: %w", copyErr)
	}
	if waitErr != nil {
		return fmt.Errorf("pg_restore failed: %w: %s", waitErr, stderr.String())
	}

	fmt.Printf("restore OK: db=%s from=%s\n", db.Name, fromPath)
	return nil
}

func expectedBackupExt(compression bool, encryption bool) string {
	ext := ".dump"
	if compression {
		ext += ".gz"
	}
	if encryption {
		ext += ".enc"
	}
	return ext
}

func backupSuffix(name string) string {
	switch {
	case strings.HasSuffix(name, ".dump.gz.enc"):
		return ".dump.gz.enc"
	case strings.HasSuffix(name, ".dump.enc"):
		return ".dump.enc"
	case strings.HasSuffix(name, ".dump.gz"):
		return ".dump.gz"
	case strings.HasSuffix(name, ".dump"):
		return ".dump"
	default:
		return "<unknown>"
	}
}
