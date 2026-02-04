package backup

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/dev-tams/backupkit/internal/config"
)

type PostgresBackupper struct{}

// Backup streams a pg_dump custom-format archive for the given database config.
func (backup PostgresBackupper) Backup(ctx context.Context, cfg config.DatabaseConfig) (io.Reader, error) {

	if _, err := exec.LookPath("pg_dump"); err != nil {
		return nil, fmt.Errorf(" pg_dump not found in PATH: %w", err)
	}
	conn := cfg.Connection

	cmd := exec.CommandContext(
		ctx,
		"pg_dump",
		"--host", conn.Host,
		"--port", strconv.Itoa(conn.Port),
		"--dbname", conn.Database,
		"--username", conn.User,
		"--format=custom",
	)
	// pg_dump reads the password from the environment variable if provided.
	if conn.Password != "" {
		cmd.Env = append(os.Environ(), "PGPASSWORD="+conn.Password)
	} else {
		cmd.Env = os.Environ()
	}

	// StdoutPipe returns a reader for the backup stream; call Start before reading.
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	pr, pw := io.Pipe()
	cmd.Stdout = pw

	go func() {
		//waits for command
		err := cmd.Run()
		if err != nil {
			_ = pw.CloseWithError(fmt.Errorf("pg_dump failed: %w : %s", err, stderr.String()))
			return
		}
		_ = pw.Close()
	}()
	return pr, nil
}
