package backup

import (
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
	cmd.Env = append(os.Environ(), "PGPASSWORD="+conn.Password)

	// StdoutPipe returns a reader for the backup stream; call Start before reading.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get pg_dump stdout: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start pg_dump: %w", err)
	}

	return stdout, nil
}
