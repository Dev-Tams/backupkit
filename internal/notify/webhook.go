package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type webhookNotifier struct {
	url     string
	headers map[string]string
	client  *http.Client
}

func NewWebhook(url string, headers map[string]string) (Notifier, error) {
	trimmedURL := strings.TrimSpace(url)
	if trimmedURL == "" {
		return nil, fmt.Errorf("config.url is required")
	}

	copyHeaders := make(map[string]string, len(headers))
	for k, v := range headers {
		copyHeaders[k] = v
	}

	return &webhookNotifier{
		url:     trimmedURL,
		headers: copyHeaders,
		client:  &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (w *webhookNotifier) Notify(ctx context.Context, event Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("received non-success status: %s", resp.Status)
	}

	return nil
}
