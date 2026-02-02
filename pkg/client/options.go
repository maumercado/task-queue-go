package client

import (
	"context"
	"net/http"
	"time"
)

// Option configures the TaskQueue client.
type Option func(*options)

type options struct {
	apiKey     string
	httpClient *http.Client
	timeout    time.Duration
	headers    map[string]string
}

func defaultOptions() *options {
	return &options{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		timeout: 30 * time.Second,
		headers: make(map[string]string),
	}
}

// WithAPIKey sets the API key for authentication.
func WithAPIKey(key string) Option {
	return func(o *options) {
		o.apiKey = key
	}
}

// WithHTTPClientOpt allows providing a custom HTTP client.
func WithHTTPClientOpt(client *http.Client) Option {
	return func(o *options) {
		o.httpClient = client
	}
}

// WithTimeout sets the default timeout for HTTP requests.
func WithTimeout(d time.Duration) Option {
	return func(o *options) {
		o.timeout = d
		if o.httpClient != nil {
			o.httpClient.Timeout = d
		}
	}
}

// WithHeader adds a custom header to all requests.
func WithHeader(key, value string) Option {
	return func(o *options) {
		o.headers[key] = value
	}
}

// applyHeaders returns a RequestEditorFn that adds configured headers.
func (o *options) applyHeaders() RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		if o.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+o.apiKey)
		}
		for k, v := range o.headers {
			req.Header.Set(k, v)
		}
		return nil
	}
}
