package schedule

import (
	"testing"
	"time"
)

func TestParseCronSpecAcceptsCommonForms(t *testing.T) {
	cases := []string{
		"* * * * *",
		"*/5 * * * *",
		"0 2 * * *",
		"0,15,30,45 9-17 * * 1-5",
	}

	for _, expr := range cases {
		if _, err := ParseCronSpec(expr); err != nil {
			t.Fatalf("ParseCronSpec(%q) unexpected error: %v", expr, err)
		}
	}
}

func TestParseCronSpecRejectsInvalid(t *testing.T) {
	cases := []string{
		"61 * * * *",
		"* 24 * * *",
		"* * 0 * *",
		"* * * 13 *",
		"* * * * 7",
		"* * * *",
		"bad * * * *",
	}

	for _, expr := range cases {
		if _, err := ParseCronSpec(expr); err == nil {
			t.Fatalf("ParseCronSpec(%q) expected error, got nil", expr)
		}
	}
}

func TestCronSpecMatches(t *testing.T) {
	spec, err := ParseCronSpec("15 2 * * 1-5")
	if err != nil {
		t.Fatalf("ParseCronSpec: %v", err)
	}

	match := time.Date(2026, 2, 20, 2, 15, 0, 0, time.UTC) // Friday
	noMatchMinute := time.Date(2026, 2, 20, 2, 16, 0, 0, time.UTC)
	noMatchDow := time.Date(2026, 2, 21, 2, 15, 0, 0, time.UTC) // Saturday

	if !spec.Matches(match) {
		t.Fatalf("expected match at %s", match)
	}
	if spec.Matches(noMatchMinute) {
		t.Fatalf("expected no match at %s", noMatchMinute)
	}
	if spec.Matches(noMatchDow) {
		t.Fatalf("expected no match at %s", noMatchDow)
	}
}
