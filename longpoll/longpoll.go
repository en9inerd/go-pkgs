package longpoll

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// ResponseHandler is a function that processes a long polling response.
// It receives the HTTP response and should return:
// - nextURL: the URL to use for the next request (empty string to reuse the same URL)
// - shouldContinue: true to continue polling, false to stop
// - error: an error to stop polling with an error
//
// This allows handlers to dynamically update request parameters (e.g., offset for Telegram Bot API).
type ResponseHandler func(*http.Response) (nextURL string, shouldContinue bool, err error)

// SimpleResponseHandler is a simplified handler that doesn't modify the URL.
type SimpleResponseHandler func(*http.Response) (bool, error)

// Config holds configuration for the long polling client.
type Config struct {
	// PollTimeout is the timeout for each individual poll request.
	// Default: 60 seconds
	PollTimeout time.Duration

	// RetryDelay is the delay between retries when a request fails.
	// Default: 1 second
	RetryDelay time.Duration

	// MaxRetries is the maximum number of consecutive retries before giving up.
	// Set to -1 for unlimited retries. Default: -1
	MaxRetries int

	// HTTPClient is the underlying HTTP client to use.
	// If nil, a default client will be created.
	HTTPClient *http.Client

	// Logger is an optional logger for debugging.
	Logger *slog.Logger

	// Headers are additional headers to include in each request.
	Headers map[string]string

	// Method is the HTTP method to use for requests. Default: GET
	Method string

	// BodyBuilder returns the request body for each poll.
	// If nil, no body is sent.
	BodyBuilder func() (io.Reader, error)
}

// Client is a long polling HTTP client.
type Client struct {
	config     Config
	httpClient *http.Client
	logger     *slog.Logger
	headers    map[string]string
	mu         sync.RWMutex
	active     map[*pollContext]struct{}
}

// pollContext tracks an active polling operation.
type pollContext struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new long polling client with default settings.
func New() *Client {
	return NewWithConfig(Config{})
}

// NewWithConfig creates a new long polling client with custom configuration.
func NewWithConfig(cfg Config) *Client {
	if cfg.PollTimeout == 0 {
		cfg.PollTimeout = 60 * time.Second
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 1 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = -1 // unlimited by default
	}
	if cfg.Method == "" {
		cfg.Method = http.MethodGet
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{
			Timeout: cfg.PollTimeout,
		}
	} else {
		if cfg.HTTPClient.Timeout == 0 {
			cfg.HTTPClient.Timeout = cfg.PollTimeout
		}
	}
	if cfg.Headers == nil {
		cfg.Headers = make(map[string]string)
	}

	return &Client{
		config:     cfg,
		httpClient: cfg.HTTPClient,
		logger:     cfg.Logger,
		headers:    cfg.Headers,
		active:     make(map[*pollContext]struct{}),
	}
}

// Poll starts a long polling loop that continuously polls the given URL.
// The handler function is called for each response. Polling continues until:
// - The context is cancelled
// - The handler returns shouldContinue=false
// - The handler returns an error
// - MaxRetries is exceeded (if set)
//
// The handler can return a new URL for the next request, or an empty string
// to reuse the same URL. This is useful for APIs like Telegram Bot API that
// require updating parameters (e.g., offset) between requests.
//
// This method blocks until polling stops. To poll in the background, call it
// in a goroutine.
func (c *Client) Poll(ctx context.Context, url string, handler ResponseHandler) error {
	pollCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	pc := &pollContext{
		ctx:    pollCtx,
		cancel: cancel,
	}

	c.mu.Lock()
	c.active[pc] = struct{}{}
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.active, pc)
		c.mu.Unlock()
	}()

	return c.pollLoop(pollCtx, url, handler)
}

// PollSimple is a convenience method that uses a SimpleResponseHandler.
// The URL remains constant across all requests.
func (c *Client) PollSimple(ctx context.Context, url string, handler SimpleResponseHandler) error {
	return c.Poll(ctx, url, func(resp *http.Response) (string, bool, error) {
		shouldContinue, err := handler(resp)
		return "", shouldContinue, err
	})
}

// pollLoop performs the actual polling loop.
func (c *Client) pollLoop(ctx context.Context, url string, handler ResponseHandler) error {
	retries := 0
	currentURL := url

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := c.makeRequest(ctx, currentURL)
		if err != nil {
			if c.logger != nil {
				c.logger.Warn("long poll request failed", "url", currentURL, "error", err)
			}

			if c.config.MaxRetries >= 0 && retries >= c.config.MaxRetries {
				return fmt.Errorf("max retries exceeded: %w", err)
			}

			retries++
			if c.logger != nil {
				c.logger.Debug("retrying long poll", "url", currentURL, "retry", retries)
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.config.RetryDelay):
				continue
			}
		}

		retries = 0

		nextURL, shouldContinue, err := handler(resp)
		if err != nil {
			resp.Body.Close()
			return fmt.Errorf("handler error: %w", err)
		}

		resp.Body.Close()

		if nextURL != "" {
			currentURL = nextURL
			if c.logger != nil {
				c.logger.Debug("handler updated URL", "new_url", currentURL)
			}
		}

		if !shouldContinue {
			if c.logger != nil {
				c.logger.Debug("handler requested stop", "url", currentURL)
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

// makeRequest creates and executes a single long polling HTTP request.
func (c *Client) makeRequest(ctx context.Context, url string) (*http.Response, error) {
	var bodyReader io.Reader
	if c.config.BodyBuilder != nil {
		var err error
		bodyReader, err = c.config.BodyBuilder()
		if err != nil {
			return nil, fmt.Errorf("build request body: %w", err)
		}
	}

	method := c.config.Method
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	c.mu.RLock()
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	c.mu.RUnlock()

	if bodyReader != nil && method == http.MethodPost {
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("http error %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// StopAll stops all active polling operations.
func (c *Client) StopAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for pc := range c.active {
		pc.cancel()
	}
}

// ActiveCount returns the number of active polling operations.
func (c *Client) ActiveCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.active)
}

// WithHeader adds a header that will be included in all polling requests.
func (c *Client) WithHeader(key, value string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.headers[key] = value
	return c
}

// WithHeaders sets multiple headers for all polling requests.
func (c *Client) WithHeaders(headers map[string]string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range headers {
		c.headers[k] = v
	}
	return c
}

// WithLogger sets the logger for the client.
func (c *Client) WithLogger(logger *slog.Logger) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logger = logger
	return c
}

// WithMethod sets the HTTP method for polling requests (GET, POST, etc.).
func (c *Client) WithMethod(method string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.Method = method
	return c
}

// WithBodyBuilder sets a function that builds the request body for each poll.
func (c *Client) WithBodyBuilder(builder func() (io.Reader, error)) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.BodyBuilder = builder
	return c
}
