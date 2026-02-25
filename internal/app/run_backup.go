package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/dev-tams/backupkit/internal/backup"
	"github.com/dev-tams/backupkit/internal/config"
	"github.com/dev-tams/backupkit/internal/notify"
	"github.com/dev-tams/backupkit/internal/storage"
)

const notificationTimeout = 5 * time.Second

type BackupResult struct {
	DB       string
	Status   string
	Bytes    int64
	Dest     string
	Duration time.Duration
	Err      error
}

// For now: the dump stream to a local file path like:
// ./backups/<dbName>/<timestamp>.dump
func RunBackup(ctx context.Context, cfg *config.Config, verbose bool) error {
	_, err := RunBackupWithResults(ctx, cfg, verbose)
	return err
}

func RunBackupWithResults(ctx context.Context, cfg *config.Config, verbose bool) ([]BackupResult, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	usedStorage := make(map[string]struct{}, len(cfg.Databases))
	for _, db := range cfg.Databases {
		usedStorage[db.Backup.Storage] = struct{}{}
	}

	stores, err := storage.FromConfigByNames(ctx, cfg, usedStorage)
	if err != nil {
		return nil, err
	}

	dispatcher, err := notify.NewDispatcher(cfg.Notifications)
	if err != nil {
		return nil, err
	}

	pg := backup.PostgresBackupper{}
	results := make([]BackupResult, 0, len(cfg.Databases))

	for _, db := range cfg.Databases {
		started := time.Now().UTC()

		if db.Type != "postgres" {
			res := BackupResult{
				DB:       db.Name,
				Status:   notify.StatusFailure,
				Duration: time.Since(started),
				Err:      fmt.Errorf("unsupported database type: %s {db: %s}", db.Type, db.Name),
			}
			results = append(results, res)
			notifyResult(ctx, dispatcher, res, verbose)
			return results, res.Err
		}

		st, ok := stores[db.Backup.Storage]
		if !ok {
			res := BackupResult{
				DB:       db.Name,
				Status:   notify.StatusFailure,
				Duration: time.Since(started),
				Err:      fmt.Errorf("db %s: storage %q not found", db.Name, db.Backup.Storage),
			}
			results = append(results, res)
			notifyResult(ctx, dispatcher, res, verbose)
			return results, res.Err
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
			res := BackupResult{
				DB:       db.Name,
				Status:   notify.StatusFailure,
				Duration: time.Since(started),
				Err:      fmt.Errorf("backup failed for %s: %w", db.Name, err),
			}
			results = append(results, res)
			notifyResult(ctx, dispatcher, res, verbose)
			return results, res.Err
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
			res := BackupResult{
				DB:       db.Name,
				Status:   notify.StatusFailure,
				Duration: time.Since(started),
				Err:      fmt.Errorf("open storage writer: %w", err),
			}
			results = append(results, res)
			notifyResult(ctx, dispatcher, res, verbose)
			return results, res.Err
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

		copyDone := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				// Force-unblock copy/write path when context is canceled or times out.
				_ = r.Close()
				_ = w.Close()
			case <-copyDone:
			}
		}()

		n, copyErr := io.Copy(w, stream)
		close(copyDone)

		// close order matters
		cs.closeAll()
		closeDumpErr := r.Close()
		closeWriteErr := w.Close()

		if copyErr != nil {
			res := BackupResult{
				DB:       db.Name,
				Status:   notify.StatusFailure,
				Bytes:    n,
				Dest:     dest,
				Duration: time.Since(started),
			}
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				res.Err = fmt.Errorf("backup timed out for %s: %w", db.Name, ctx.Err())
				results = append(results, res)
				notifyResult(ctx, dispatcher, res, verbose)
				return results, res.Err
			}
			if errors.Is(ctx.Err(), context.Canceled) {
				res.Err = fmt.Errorf("backup canceled for %s: %w", db.Name, ctx.Err())
				results = append(results, res)
				notifyResult(ctx, dispatcher, res, verbose)
				return results, res.Err
			}
			// local writer will leave .tmp if not closed successfully; we closed it above.
			res.Err = fmt.Errorf("write backup: %w", copyErr)
			results = append(results, res)
			notifyResult(ctx, dispatcher, res, verbose)
			return results, res.Err
		}
		if closeDumpErr != nil {
			res := BackupResult{
				DB:       db.Name,
				Status:   notify.StatusFailure,
				Bytes:    n,
				Dest:     dest,
				Duration: time.Since(started),
				Err:      fmt.Errorf("close dump stream: %w", closeDumpErr),
			}
			results = append(results, res)
			notifyResult(ctx, dispatcher, res, verbose)
			return results, res.Err
		}
		if closeWriteErr != nil {
			res := BackupResult{
				DB:       db.Name,
				Status:   notify.StatusFailure,
				Bytes:    n,
				Dest:     dest,
				Duration: time.Since(started),
				Err:      fmt.Errorf("finalize storage write: %w", closeWriteErr),
			}
			results = append(results, res)
			notifyResult(ctx, dispatcher, res, verbose)
			return results, res.Err
		}

		res := BackupResult{
			DB:       db.Name,
			Status:   notify.StatusSuccess,
			Bytes:    n,
			Dest:     dest,
			Duration: time.Since(started),
		}

		// after successful backup
		if err := ApplyRetention(ctx, db, st, verbose); err != nil {
			res.Status = notify.StatusFailure
			res.Err = fmt.Errorf("retention failed for %s: %w", db.Name, err)
			results = append(results, res)
			notifyResult(ctx, dispatcher, res, verbose)
			return results, res.Err
		}
		results = append(results, res)

		fmt.Printf("backup OK: db=%s bytes=%d dest=%s duration=%s\n", db.Name, n, dest, res.Duration.Round(time.Millisecond))
		notifyResult(ctx, dispatcher, res, verbose)

	}

	return results, nil
}

func notifyResult(ctx context.Context, dispatcher *notify.Dispatcher, res BackupResult, verbose bool) {
	errMsg := ""
	if res.Err != nil {
		errMsg = res.Err.Error()
	}

	event := notify.Event{
		DB:       res.DB,
		Status:   res.Status,
		Bytes:    res.Bytes,
		Dest:     res.Dest,
		Duration: res.Duration.Round(time.Millisecond).String(),
		Error:    errMsg,
	}

	notifyCtx, cancel := notificationContext(ctx)
	defer cancel()

	if err := dispatcher.Notify(notifyCtx, event); err != nil && verbose {
		fmt.Printf("notification failed: db=%s status=%s err=%v\n", res.DB, res.Status, err)
	}
}

func notificationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), notificationTimeout)
	}
	return context.WithTimeout(context.WithoutCancel(ctx), notificationTimeout)
}
