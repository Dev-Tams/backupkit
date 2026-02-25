package app

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateRestoreToolAvailabilityPrefersPsqlForSQL(t *testing.T) {
	orig := execLookPath
	defer func() { execLookPath = orig }()

	var lookedUp []string
	execLookPath = func(file string) (string, error) {
		lookedUp = append(lookedUp, file)
		return "/usr/bin/" + file, nil
	}

	if err := validateRestoreToolAvailability("sql"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lookedUp) != 1 || lookedUp[0] != "psql" {
		t.Fatalf("expected only psql lookup, got %v", lookedUp)
	}
}

func TestValidateRestoreToolAvailabilityUsesPgRestoreForCustomDump(t *testing.T) {
	orig := execLookPath
	defer func() { execLookPath = orig }()

	var lookedUp []string
	execLookPath = func(file string) (string, error) {
		lookedUp = append(lookedUp, file)
		return "/usr/bin/" + file, nil
	}

	if err := validateRestoreToolAvailability("pgdmp"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lookedUp) != 1 || lookedUp[0] != "pg_restore" {
		t.Fatalf("expected only pg_restore lookup, got %v", lookedUp)
	}
}

func TestValidateRestoreToolAvailabilityMissingPsql(t *testing.T) {
	orig := execLookPath
	defer func() { execLookPath = orig }()

	execLookPath = func(file string) (string, error) {
		if file == "psql" {
			return "", errors.New("not found")
		}
		return "/usr/bin/" + file, nil
	}

	err := validateRestoreToolAvailability("sql")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "psql not found in PATH") {
		t.Fatalf("unexpected error: %v", err)
	}
}
