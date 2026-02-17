package app

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/dev-tams/backupkit/internal/config"
	"github.com/dev-tams/backupkit/internal/storage"
	"github.com/dev-tams/backupkit/internal/storage/prunable"
)

type backupEntry struct {
	obj prunable.ObjectInfo
	t   time.Time
}

func ApplyRetention(ctx context.Context, db config.DatabaseConfig, st storage.Storage, verbose bool) error {
	r := db.Retention
	if r.KeepDaily <= 0 && r.KeepWeekly <= 0 && r.KeepMonthly <= 0 {
		return nil
	}

	pr, ok := st.(prunable.Prunable)
	if !ok {
		if verbose {
			fmt.Printf("retention: db=%s storage=%s skipped (not prunable)\n", db.Name, st.Name())
		}
		return nil
	}

	objects, err := pr.List(ctx, db.Name)
	if err != nil {
		return fmt.Errorf("retention list: %w", err)
	}
	if len(objects) == 0 {
		return nil
	}

	entries := make([]backupEntry, 0, len(objects))
	skipped := 0
	for _, o := range objects {
		t, ok := parseBackupTimeFromKey(o.Key)
		if !ok {
			skipped++
			continue
		}
		entries = append(entries, backupEntry{obj: o, t: t})
	}

	// newest first
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].t.After(entries[j].t)
	})

	keep := selectKeep(entries, r.KeepDaily, r.KeepWeekly, r.KeepMonthly)

	deleted := 0
	for _, e := range entries {
		if keep[e.obj.Key] {
			continue
		}
		if err := pr.Delete(ctx, e.obj.Key); err != nil {
			return fmt.Errorf("retention delete: %w", err)
		}
		deleted++
	}

	if verbose {
		fmt.Printf(
			"retention: db=%s storage=%s kept=%d deleted=%d skipped=%d\n",
			db.Name,
			st.Name(),
			len(keep),
			deleted,
			skipped,
		)
	}

	return nil
}

func selectKeep(entries []backupEntry, keepDaily, keepWeekly, keepMonthly int) map[string]bool {
	keep := make(map[string]bool, len(entries))

	daily := make(map[string]bool)
	weekly := make(map[string]bool)
	monthly := make(map[string]bool)

	dCount, wCount, mCount := 0, 0, 0

	for _, e := range entries {
		t := e.t.UTC()

		// daily bucket
		if keepDaily > 0 && dCount < keepDaily {
			b := t.Format("2006-01-02")
			if !daily[b] {
				daily[b] = true
				keep[e.obj.Key] = true
				dCount++
			}
		}

		// weekly bucket (ISO week)
		if keepWeekly > 0 && wCount < keepWeekly {
			y, w := t.ISOWeek()
			b := fmt.Sprintf("%04d-W%02d", y, w)
			if !weekly[b] {
				weekly[b] = true
				keep[e.obj.Key] = true
				wCount++
			}
		}

		// monthly bucket
		if keepMonthly > 0 && mCount < keepMonthly {
			b := t.Format("2006-01")
			if !monthly[b] {
				monthly[b] = true
				keep[e.obj.Key] = true
				mCount++
			}
		}

		if (keepDaily <= 0 || dCount >= keepDaily) &&
			(keepWeekly <= 0 || wCount >= keepWeekly) &&
			(keepMonthly <= 0 || mCount >= keepMonthly) {
			break
		}
	}

	return keep
}

func parseBackupTimeFromKey(key string) (time.Time, bool) {
	base := path.Base(key)
	// take everything up to ".dump"
	i := strings.Index(base, ".dump")
	if i <= 0 {
		return time.Time{}, false
	}
	ts := base[:i]

	// Example: 20260217_224501.123456789Z
	t, err := time.Parse("20060102_150405.000000000Z", ts)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
