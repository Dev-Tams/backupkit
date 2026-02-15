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
	allowSQLFallback bool,
) error {
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

	// Build decode stages from file bytes, not config, so restore follows actual payload.
	switch rawKind {
	case "enc":
		if db.Backup.Encryption.Password == "" {
			return fmt.Errorf("restore/decrypt: encryption password is empty (db=%s)", db.Name)
		}
		stream = decryptReader(stream, db.Backup.Encryption.Password, &cs)
	case "gzip":
		stream = gunzipReader(stream, &cs)
	case "pgdmp":
		// no transform
	case "unknown":
		if !allowSQLFallback {
			return fmt.Errorf("restore/sniff: unrecognized backup header; rerun with --allow-sql-fallback if this may be a plain SQL dump")
		}
	default:
		return fmt.Errorf("restore/sniff: unsupported raw stream kind %q", rawKind)
	}

	br := bufio.NewReader(stream)
	innerKind := rawKind
	if rawKind == "enc" {
		innerKind, err = sniffLeadingKind(br)
		if err != nil {
			cs.closeAll()
			return fmt.Errorf("restore/sniff: %w", err)
		}
		if innerKind == "gzip" {
			stream = gunzipReader(br, &cs)
			br = bufio.NewReader(stream)
		}
	}

	decodedKind, err := sniffDecodedKind(br)
	if err != nil {
		cs.closeAll()
		return fmt.Errorf("restore/sniff: %w", err)
	}
	switch decodedKind {
	case "pgdmp":
	case "sql":
		if !allowSQLFallback {
			cs.closeAll()
			return fmt.Errorf("restore/sniff: decoded stream looks like SQL text; rerun with --allow-sql-fallback to restore with psql")
		}
	default:
		cs.closeAll()
		return fmt.Errorf("restore/sniff: decoded stream is neither pg_dump custom format nor recognizable SQL text")
	}
	stream = br

	conn := db.Connection

	if verbose {
		fmt.Printf(
			"restore pipeline: db=%s raw=%s inner=%s tool=pg_restore clean=%v\n",
			db.Name,
			rawKind,
			innerKind,
			clean,
		)
	}

	if decodedKind == "sql" {
		if _, err := exec.LookPath("psql"); err != nil {
			cs.closeAll()
			return fmt.Errorf("psql not found in PATH: %w", err)
		}
		if clean {
			fmt.Fprintln(os.Stderr, "warning: --clean is ignored when falling back to psql")
		}
		if verbose {
			fmt.Printf("restore tool fallback: db=%s tool=psql\n", db.Name)
		}
		args := []string{
			"--host", conn.Host,
			"--port", strconv.Itoa(conn.Port),
			"--dbname", conn.Database,
			"--username", conn.User,
			"-v", "ON_ERROR_STOP=1",
		}
		return runSQLRestore(ctx, args, conn.Password, stream, &cs, db.Name, fromPath)
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
		cs.closeAll()
		return fmt.Errorf("restore/pg_restore/stdin: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		cs.closeAll()
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

func runSQLRestore(
	ctx context.Context,
	args []string,
	password string,
	stream io.Reader,
	cs *closeStack,
	dbName string,
	fromPath string,
) error {
	cmd := exec.CommandContext(ctx, "psql", args...)
	if password != "" {
		cmd.Env = append(os.Environ(), "PGPASSWORD="+password)
	} else {
		cmd.Env = os.Environ()
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("restore/psql/stdin: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return fmt.Errorf("restore/psql/start: %w", err)
	}

	_, copyErr := io.Copy(stdin, stream)
	_ = stdin.Close()
	cs.closeAll()

	waitErr := cmd.Wait()

	if copyErr != nil {
		return fmt.Errorf("restore/stream: %w", copyErr)
	}
	if waitErr != nil {
		return fmt.Errorf("restore/psql/wait: %w: %s", waitErr, strings.TrimSpace(stderr.String()))
	}

	fmt.Printf("restore OK: db=%s from=%s tool=psql\n", dbName, fromPath)
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

func sniffDecodedKind(r *bufio.Reader) (string, error) {
	h, err := r.Peek(len(pgdmpMagic))
	if err == nil && bytes.Equal(h, pgdmpMagic) {
		return "pgdmp", nil
	}
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return "", fmt.Errorf("unable to read decoded stream header: %w", err)
	}

	probe, err := r.Peek(256)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return "", fmt.Errorf("unable to inspect decoded stream: %w", err)
	}
	if len(probe) == 0 {
		return "", fmt.Errorf("decoded stream is empty or truncated")
	}
	if looksLikeSQL(string(probe)) {
		return "sql", nil
	}
	return "unknown", nil
}

func sniffLeadingKind(r *bufio.Reader) (string, error) {
	h, err := r.Peek(len(encMagic))
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return "", fmt.Errorf("unable to inspect decoded stream prefix: %w", err)
	}
	if len(h) == 0 {
		return "", fmt.Errorf("decoded stream is empty or truncated")
	}
	switch {
	case len(h) >= len(gzipMagic) && bytes.Equal(h[:len(gzipMagic)], gzipMagic):
		return "gzip", nil
	case len(h) >= len(pgdmpMagic) && bytes.Equal(h[:len(pgdmpMagic)], pgdmpMagic):
		return "pgdmp", nil
	default:
		if looksLikeSQL(string(h)) {
			return "sql", nil
		}
		return "unknown", nil
	}
}

func looksLikeSQL(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	upper := strings.ToUpper(trimmed)
	prefixes := []string{
		"--",
		"/*",
		"SET ",
		"CREATE ",
		"INSERT ",
		"UPDATE ",
		"DELETE ",
		"BEGIN",
		"COPY ",
		"ALTER ",
		"DO ",
		"SELECT ",
		"\\CONNECT ",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(upper, p) {
			return true
		}
	}
	return false
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
