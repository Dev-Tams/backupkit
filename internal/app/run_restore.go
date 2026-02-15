package app

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dev-tams/backupkit/internal/config"
)

var (
	encMagic   = []byte("BKENC001")
	gzipMagic  = []byte{0x1f, 0x8b}
	pgdmpMagic = []byte("PGDMP")
)

func RunRestore(
	ctx context.Context,
	cfg *config.Config,
	dbName string,
	fromPath string,
	verbose bool,
	clean bool,
	strictSniff bool,
) error {
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
		return fmt.Errorf("restore/open: %w", err)
	}
	defer f.Close()

	rawKind, err := sniffRawKind(f)
	if err != nil {
		return fmt.Errorf("restore/sniff: %w", err)
	}

	expectedRaw := expectedRawKind(db.Backup.Compression, db.Backup.Encryption.Enabled)
	if rawKind != expectedRaw {
		msg := fmt.Sprintf(
			"backup header mismatch for db=%s: expected %q from config, got %q (%s)",
			db.Name,
			expectedRaw,
			rawKind,
			fromPath,
		)
		if strictSniff {
			return fmt.Errorf("restore/sniff: %s", msg)
		}
		fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
	}

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
			return fmt.Errorf("restore/decrypt: encryption password is empty (db=%s)", db.Name)
		}
		stream = decryptReader(stream, db.Backup.Encryption.Password, &cs)
	}

	if db.Backup.Compression {
		stream = gunzipReader(stream, &cs)
	}
	br := bufio.NewReader(stream)
	if err := ensureCustomDumpHeader(br); err != nil {
		cs.closeAll()
		return fmt.Errorf("restore/sniff: %w", err)
	}
	stream = br

	conn := db.Connection

	if verbose {
		fmt.Printf(
			"restore pipeline: db=%s decrypt=%v gunzip=%v tool=pg_restore clean=%v\n",
			db.Name,
			db.Backup.Encryption.Enabled,
			db.Backup.Compression,
			clean,
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
	if clean {
		args = append(args, "--clean", "--if-exists")
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
		return fmt.Errorf("restore/pg_restore/stdin: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return fmt.Errorf("restore/pg_restore/start: %w", err)
	}

	_, copyErr := io.Copy(stdin, stream)
	_ = stdin.Close()

	// close pipeline readers (pipe readers) after streaming completes
	cs.closeAll()

	waitErr := cmd.Wait()

	if copyErr != nil {
		return fmt.Errorf("restore/stream: %w", copyErr)
	}
	if waitErr != nil {
		pgErr := strings.TrimSpace(stderr.String())
		if strings.Contains(pgErr, "already exists") {
			return fmt.Errorf(
				"restore/pg_restore/wait: %w: %s\nhint: target database is not empty. rerun with --clean or restore into a fresh database",
				waitErr,
				pgErr,
			)
		}
		return fmt.Errorf("restore/pg_restore/wait: %w: %s", waitErr, pgErr)
	}

	fmt.Printf("restore OK: db=%s from=%s\n", db.Name, fromPath)
	return nil
}

func sniffRawKind(f *os.File) (string, error) {
	var hdr [8]byte
	n, err := io.ReadFull(f, hdr[:])
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", err
	}
	if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
		return "", seekErr
	}
	b := hdr[:n]
	switch {
	case len(b) >= len(encMagic) && bytes.Equal(b[:len(encMagic)], encMagic):
		return "enc", nil
	case len(b) >= len(gzipMagic) && bytes.Equal(b[:len(gzipMagic)], gzipMagic):
		return "gzip", nil
	case len(b) >= len(pgdmpMagic) && bytes.Equal(b[:len(pgdmpMagic)], pgdmpMagic):
		return "pgdmp", nil
	default:
		return "unknown", nil
	}
}

func expectedRawKind(compression bool, encryption bool) string {
	if encryption {
		return "enc"
	}
	if compression {
		return "gzip"
	}
	return "pgdmp"
}

func ensureCustomDumpHeader(r *bufio.Reader) error {
	h, err := r.Peek(len(pgdmpMagic))
	if err != nil {
		return fmt.Errorf("unable to read decoded stream header: %w", err)
	}
	if !bytes.Equal(h, pgdmpMagic) {
		return fmt.Errorf("decoded stream is not pg_dump custom format (missing PGDMP header); if this is a plain SQL dump, restore with psql")
	}
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
