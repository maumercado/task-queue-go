package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/maumercado/task-queue-go/internal/logger"
	"github.com/maumercado/task-queue-go/internal/task"
)

// EchoHandler echoes the task payload back as the result
func EchoHandler(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
	logger.Info().
		Str("task_id", t.ID).
		Interface("payload", t.Payload).
		Msg("Echo handler processing task")

	return map[string]interface{}{
		"echoed":       t.Payload,
		"processed_at": time.Now().UTC(),
	}, nil
}

// SleepHandler sleeps for a specified duration
func SleepHandler(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
	duration := 1 * time.Second
	if d, ok := t.Payload["duration"].(float64); ok {
		duration = time.Duration(d) * time.Millisecond
	}
	if d, ok := t.Payload["duration_seconds"].(float64); ok {
		duration = time.Duration(d) * time.Second
	}

	logger.Info().
		Str("task_id", t.ID).
		Dur("duration", duration).
		Msg("Sleep handler processing task")

	select {
	case <-time.After(duration):
		return map[string]interface{}{
			"slept_for":    duration.String(),
			"completed_at": time.Now().UTC(),
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// HTTPHandler makes an HTTP request
func HTTPHandler(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
	url, ok := t.Payload["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url is required in payload")
	}

	method := "GET"
	if m, ok := t.Payload["method"].(string); ok {
		method = m
	}

	logger.Info().
		Str("task_id", t.ID).
		Str("url", url).
		Str("method", method).
		Msg("HTTP handler processing task")

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return map[string]interface{}{
		"status_code": resp.StatusCode,
		"status":      resp.Status,
		"headers":     resp.Header,
	}, nil
}

// ComputeHandler performs a computation
func ComputeHandler(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
	iterations := 1000000
	if i, ok := t.Payload["iterations"].(float64); ok {
		iterations = int(i)
	}

	logger.Info().
		Str("task_id", t.ID).
		Int("iterations", iterations).
		Msg("Compute handler processing task")

	sum := int64(0)
	for i := 0; i < iterations; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			sum += int64(i)
		}
	}

	return map[string]interface{}{
		"iterations": iterations,
		"result":     sum,
	}, nil
}

// EmailHandler simulates sending an email
func EmailHandler(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
	to, ok := t.Payload["to"].(string)
	if !ok {
		return nil, fmt.Errorf("to is required in payload")
	}

	subject, _ := t.Payload["subject"].(string)
	body, _ := t.Payload["body"].(string)

	logger.Info().
		Str("task_id", t.ID).
		Str("to", to).
		Str("subject", subject).
		Msg("Email handler processing task")

	// Simulate email sending delay
	select {
	case <-time.After(100 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return map[string]interface{}{
		"sent_to": to,
		"subject": subject,
		"body":    body,
		"sent_at": time.Now().UTC(),
	}, nil
}

// WebhookHandler sends a webhook notification
func WebhookHandler(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
	url, ok := t.Payload["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url is required in payload")
	}

	data, ok := t.Payload["data"].(map[string]interface{})
	if !ok {
		data = t.Payload
	}

	logger.Info().
		Str("task_id", t.ID).
		Str("url", url).
		Msg("Webhook handler processing task")

	body, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("webhook request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("webhook returned error status: %d", resp.StatusCode)
	}

	return map[string]interface{}{
		"url":         url,
		"status_code": resp.StatusCode,
		"body_size":   len(body),
		"sent_at":     time.Now().UTC(),
	}, nil
}

// FailHandler intentionally fails for testing
func FailHandler(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
	message := "intentional failure for testing"
	if m, ok := t.Payload["message"].(string); ok {
		message = m
	}

	logger.Info().
		Str("task_id", t.ID).
		Str("message", message).
		Msg("Fail handler processing task")

	return nil, fmt.Errorf("%s", message)
}
