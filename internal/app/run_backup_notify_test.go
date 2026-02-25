package app

import (
	"context"
	"testing"
	"time"
)

func TestNotificationContextIgnoresParentCancelAndPreservesValues(t *testing.T) {
	type key string
	const k key = "trace"

	parent, stop := context.WithCancel(context.WithValue(context.Background(), k, "abc"))
	stop()

	ctx, cancel := notificationContext(parent)
	defer cancel()

	select {
	case <-ctx.Done():
		t.Fatalf("notification context should not be canceled by parent cancel")
	default:
	}

	if got := ctx.Value(k); got != "abc" {
		t.Fatalf("expected context value to be preserved, got %v", got)
	}
}

func TestNotificationContextAppliesTimeout(t *testing.T) {
	ctx, cancel := notificationContext(context.Background())
	defer cancel()

	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline to be set")
	}

	remaining := time.Until(dl)
	if remaining <= 0 || remaining > notificationTimeout+time.Second {
		t.Fatalf("unexpected deadline window: %s", remaining)
	}
}
