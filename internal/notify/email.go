package notify

import (
	"context"
	"fmt"
	"net/smtp"
	"strconv"
	"strings"
)

type emailNotifier struct {
	host     string
	port     int
	from     string
	to       []string
	username string
	password string
}

func NewEmail(host string, port int, from, to, username, password string) (Notifier, error) {
	host = strings.TrimSpace(host)
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if host == "" {
		return nil, fmt.Errorf("config.smtp_host is required")
	}
	if port <= 0 {
		return nil, fmt.Errorf("config.smtp_port must be > 0")
	}
	if from == "" {
		return nil, fmt.Errorf("config.from is required")
	}
	if to == "" {
		return nil, fmt.Errorf("config.to is required")
	}

	recipients := splitRecipients(to)
	if len(recipients) == 0 {
		return nil, fmt.Errorf("config.to must include at least one recipient")
	}

	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if (username == "") != (password == "") {
		return nil, fmt.Errorf("config.username and config.password must be set together")
	}

	return &emailNotifier{
		host:     host,
		port:     port,
		from:     from,
		to:       recipients,
		username: username,
		password: password,
	}, nil
}

func (e *emailNotifier) Notify(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	subject := fmt.Sprintf("[backupkit] %s: %s", event.Status, event.DB)
	body := buildEmailBody(event)
	msg := []byte(strings.Join([]string{
		"From: " + e.from,
		"To: " + strings.Join(e.to, ", "),
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n"))

	addr := e.host + ":" + strconv.Itoa(e.port)
	var auth smtp.Auth
	if e.username != "" {
		auth = smtp.PlainAuth("", e.username, e.password, e.host)
	}

	if err := smtp.SendMail(addr, auth, e.from, e.to, msg); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	return nil
}

func buildEmailBody(event Event) string {
	lines := []string{
		"Backup event",
		"",
		"db: " + event.DB,
		"status: " + event.Status,
		fmt.Sprintf("bytes: %d", event.Bytes),
		"dest: " + event.Dest,
		"duration: " + event.Duration,
	}
	if event.Error != "" {
		lines = append(lines, "error: "+event.Error)
	}
	return strings.Join(lines, "\n")
}

func splitRecipients(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}
