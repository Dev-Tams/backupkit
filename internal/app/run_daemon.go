package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dev-tams/backupkit/internal/config"
	"github.com/dev-tams/backupkit/internal/schedule"
)

type daemonJob struct {
	db       config.DatabaseConfig
	schedule schedule.CronSpec
}

func RunDaemon(ctx context.Context, cfg *config.Config, verbose bool, runTimeout time.Duration) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	jobs := make([]daemonJob, 0, len(cfg.Databases))
	for _, db := range cfg.Databases {
		s := strings.TrimSpace(db.Backup.Schedule)
		if s == "" {
			if verbose {
				fmt.Printf("daemon: db=%s skipped (empty schedule)\n", db.Name)
			}
			continue
		}

		spec, err := schedule.ParseCronSpec(s)
		if err != nil {
			return fmt.Errorf("db %s: invalid schedule %q: %w", db.Name, s, err)
		}
		jobs = append(jobs, daemonJob{db: db, schedule: spec})
	}

	if len(jobs) == 0 {
		return fmt.Errorf("daemon: no databases with a valid non-empty backup.schedule")
	}

	if verbose {
		fmt.Printf("daemon: started with %d scheduled database(s)\n", len(jobs))
	}

	lastMinute := time.Time{}
	lastRunByDB := make(map[string]time.Time, len(jobs))

	for {
		select {
		case <-ctx.Done():
			if verbose {
				fmt.Println("daemon: shutdown requested")
			}
			return nil
		default:
		}

		now := time.Now().UTC()
		currentMinute := now.Truncate(time.Minute)
		if currentMinute.Equal(lastMinute) {
			sleepUntilNextPoll(ctx, 500*time.Millisecond)
			continue
		}
		lastMinute = currentMinute

		due := make([]config.DatabaseConfig, 0, len(jobs))
		for _, job := range jobs {
			if !job.schedule.Matches(currentMinute) {
				continue
			}
			if lm, ok := lastRunByDB[job.db.Name]; ok && lm.Equal(currentMinute) {
				continue
			}
			due = append(due, job.db)
		}

		if len(due) == 0 {
			continue
		}

		runCfg := *cfg
		runCfg.Databases = due

		if verbose {
			fmt.Printf("daemon: triggering %d backup job(s) at %s UTC\n", len(due), currentMinute.Format(time.RFC3339))
		}

		runCtx := ctx
		cancel := func() {}
		if runTimeout > 0 {
			runCtx, cancel = context.WithTimeout(ctx, runTimeout)
		}

		err := RunBackup(runCtx, &runCfg, verbose)
		cancel()
		if err != nil {
			if runTimeout > 0 && errors.Is(runCtx.Err(), context.DeadlineExceeded) {
				if verbose {
					fmt.Printf(
						"daemon: run timeout after %s at %s UTC for %d job(s)\n",
						runTimeout,
						currentMinute.Format(time.RFC3339),
						len(due),
					)
				}
				return fmt.Errorf("daemon run timed out after %s", runTimeout)
			}
			return fmt.Errorf("daemon run: %w", err)
		}

		for _, db := range due {
			lastRunByDB[db.Name] = currentMinute
		}
	}
}

func sleepUntilNextPoll(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
