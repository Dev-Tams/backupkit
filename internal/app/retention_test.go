package app

import (
	"testing"
	"time"

	"github.com/dev-tams/backupkit/internal/storage/prunable"
)

func TestSelectKeepDailyKeepsNewestPerDay(t *testing.T) {
	entries := []backupEntry{
		{
			obj: prunable.ObjectInfo{Key: "db/20260218_120000.000000000Z.dump.enc"},
			t:   time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC),
		},
		{
			obj: prunable.ObjectInfo{Key: "db/20260218_080000.000000000Z.dump.enc"},
			t:   time.Date(2026, 2, 18, 8, 0, 0, 0, time.UTC),
		},
		{
			obj: prunable.ObjectInfo{Key: "db/20260217_230000.000000000Z.dump.enc"},
			t:   time.Date(2026, 2, 17, 23, 0, 0, 0, time.UTC),
		},
	}

	keep := selectKeep(entries, 2, 0, 0)
	if len(keep) != 2 {
		t.Fatalf("expected 2 kept entries, got %d", len(keep))
	}
	if !keep["db/20260218_120000.000000000Z.dump.enc"] {
		t.Fatalf("expected newest backup for 2026-02-18 to be kept")
	}
	if keep["db/20260218_080000.000000000Z.dump.enc"] {
		t.Fatalf("expected older backup on same day to be pruned")
	}
	if !keep["db/20260217_230000.000000000Z.dump.enc"] {
		t.Fatalf("expected newest backup for 2026-02-17 to be kept")
	}
}

func TestSelectKeepWeeklyKeepsSingleISOWeek(t *testing.T) {
	entries := []backupEntry{
		{
			obj: prunable.ObjectInfo{Key: "db/20260218_120000.000000000Z.dump.enc"},
			t:   time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC),
		},
		{
			obj: prunable.ObjectInfo{Key: "db/20260217_120000.000000000Z.dump.enc"},
			t:   time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC),
		},
	}

	keep := selectKeep(entries, 0, 1, 0)
	if len(keep) != 1 {
		t.Fatalf("expected 1 kept entry, got %d", len(keep))
	}
	if !keep["db/20260218_120000.000000000Z.dump.enc"] {
		t.Fatalf("expected newest backup in same ISO week to be kept")
	}
}

func TestParseBackupTimeFromKey(t *testing.T) {
	if _, ok := parseBackupTimeFromKey("db/not-a-timestamp.dump.enc"); ok {
		t.Fatalf("expected parse to fail for invalid timestamp")
	}

	got, ok := parseBackupTimeFromKey("db/20260218_120000.000000000Z.dump.enc")
	if !ok {
		t.Fatalf("expected parse to succeed")
	}

	want := time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("unexpected parsed time: got %s want %s", got, want)
	}
}
