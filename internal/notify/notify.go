package notify

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dev-tams/backupkit/internal/config"
)

const (
	StatusSuccess = "success"
	StatusFailure = "failure"
)

// Event is the notification payload shared by all notifier implementations.
type Event struct {
	DB       string `json:"db"`
	Status   string `json:"status"`
	Bytes    int64  `json:"bytes"`
	Dest     string `json:"dest"`
	Duration string `json:"duration"`
	Error    string `json:"error,omitempty"`
}

type Notifier interface {
	Notify(ctx context.Context, event Event) error
}

type route struct {
	onSuccess bool
	onFailure bool
	notifier  Notifier
}

type Dispatcher struct {
	routes []route
}

func NewDispatcher(cfgs []config.NotificationConfig) (*Dispatcher, error) {
	routes := make([]route, 0, len(cfgs))
	for i, n := range cfgs {
		onSuccess, onFailure, err := parseOn(n.On)
		if err != nil {
			return nil, fmt.Errorf("notifications[%d]: %w", i, err)
		}

		switch strings.ToLower(strings.TrimSpace(n.Type)) {
		case "webhook":
			nf, err := NewWebhook(n.Config.URL, n.Config.Headers)
			if err != nil {
				return nil, fmt.Errorf("notifications[%d] webhook: %w", i, err)
			}
			routes = append(routes, route{onSuccess: onSuccess, onFailure: onFailure, notifier: nf})
		case "email":
			nf, err := NewEmail(n.Config.SMTPHost, n.Config.SMTPPort, n.Config.From, n.Config.To, n.Config.Username, n.Config.Password)
			if err != nil {
				return nil, fmt.Errorf("notifications[%d] email: %w", i, err)
			}
			routes = append(routes, route{onSuccess: onSuccess, onFailure: onFailure, notifier: nf})
		default:
			return nil, fmt.Errorf("notifications[%d]: unsupported notification type %q", i, n.Type)
		}
	}
	return &Dispatcher{routes: routes}, nil
}

func (d *Dispatcher) Notify(ctx context.Context, event Event) error {
	if d == nil || len(d.routes) == 0 {
		return nil
	}

	var errs []error
	for i, r := range d.routes {
		if !r.wants(event.Status) {
			continue
		}
		if err := r.notifier.Notify(ctx, event); err != nil {
			errs = append(errs, fmt.Errorf("notification route %d: %w", i, err))
		}
	}
	return errors.Join(errs...)
}

func (r route) wants(status string) bool {
	switch status {
	case StatusSuccess:
		return r.onSuccess
	case StatusFailure:
		return r.onFailure
	default:
		return false
	}
}

func parseOn(raw []string) (bool, bool, error) {
	if len(raw) == 0 {
		return false, false, fmt.Errorf("on must include success, failure, or both")
	}

	var onSuccess bool
	var onFailure bool
	for _, v := range raw {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "success":
			onSuccess = true
		case "failure":
			onFailure = true
		case "both":
			onSuccess = true
			onFailure = true
		default:
			return false, false, fmt.Errorf("on contains unsupported value %q", v)
		}
	}

	if !onSuccess && !onFailure {
		return false, false, fmt.Errorf("on must include success, failure, or both")
	}

	return onSuccess, onFailure, nil
}
