// Package httpclient provides HTTP client utilities for making API requests
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Client wraps http.Client with additional utilities
type Client struct {
	httpClient *http.Client
	logger     *slog.Logger
	baseURL    string
	headers    map[string]string
}

// Config holds client configuration
type Config struct {
	Timeout time.Duration
	BaseURL string
	Headers map[string]string
	Logger  *slog.Logger
}

// New creates a new HTTP client with default settings
func New() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		headers: make(map[string]string),
	}
}

// NewWithConfig creates a new HTTP client with custom configuration
func NewWithConfig(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.Headers == nil {
		cfg.Headers = make(map[string]string)
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		baseURL: cfg.BaseURL,
		headers: cfg.Headers,
		logger:  cfg.Logger,
	}
}

// WithHTTPClient sets a custom HTTP client
func (c *Client) WithHTTPClient(client *http.Client) *Client {
	c.httpClient = client
	return c
}

// WithTimeout sets the request timeout
func (c *Client) WithTimeout(timeout time.Duration) *Client {
	c.httpClient.Timeout = timeout
	return c
}

// WithBaseURL sets the base URL for all requests
func (c *Client) WithBaseURL(baseURL string) *Client {
	c.baseURL = baseURL
	return c
}

// WithHeader sets a header that will be included in all requests
func (c *Client) WithHeader(key, value string) *Client {
	if c.headers == nil {
		c.headers = make(map[string]string)
	}
	c.headers[key] = value
	return c
}

// WithHeaders sets multiple headers
func (c *Client) WithHeaders(headers map[string]string) *Client {
	if c.headers == nil {
		c.headers = make(map[string]string)
	}
	for k, v := range headers {
		c.headers[k] = v
	}
	return c
}

// WithLogger sets the logger
func (c *Client) WithLogger(logger *slog.Logger) *Client {
	c.logger = logger
	return c
}

// buildURL constructs the full URL from baseURL and path
func (c *Client) buildURL(path string) string {
	if c.baseURL == "" {
		return path
	}
	if path == "" {
		return c.baseURL
	}
	if c.baseURL[len(c.baseURL)-1] == '/' && path[0] == '/' {
		return c.baseURL + path[1:]
	}
	if c.baseURL[len(c.baseURL)-1] != '/' && path[0] != '/' {
		return c.baseURL + "/" + path
	}
	return c.baseURL + path
}

// setHeaders sets default headers on the request
func (c *Client) setHeaders(req *http.Request) {
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
}

// Do executes an HTTP request
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	c.setHeaders(req)

	if c.logger != nil {
		c.logger.Debug("making http request", "method", req.Method, "url", req.URL.String())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}

	return resp, nil
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	url := c.buildURL(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	return c.Do(ctx, req)
}

// Post performs a POST request with JSON body
func (c *Client) Post(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.postPutPatch(ctx, http.MethodPost, path, body)
}

// Put performs a PUT request with JSON body
func (c *Client) Put(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.postPutPatch(ctx, http.MethodPut, path, body)
}

// Patch performs a PATCH request with JSON body
func (c *Client) Patch(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.postPutPatch(ctx, http.MethodPatch, path, body)
}

// postPutPatch is a helper for POST, PUT, and PATCH requests
func (c *Client) postPutPatch(ctx context.Context, method, path string, body any) (*http.Response, error) {
	url := c.buildURL(path)

	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal json: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.Do(ctx, req)
}

// Delete performs a DELETE request
func (c *Client) Delete(ctx context.Context, path string) (*http.Response, error) {
	url := c.buildURL(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	return c.Do(ctx, req)
}

// GetJSON performs a GET request and decodes the JSON response
func (c *Client) GetJSON(ctx context.Context, path string, target any) error {
	resp, err := c.Get(ctx, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return DecodeJSONResponse(resp, target)
}

// PostJSON performs a POST request with JSON body and decodes the JSON response
func (c *Client) PostJSON(ctx context.Context, path string, body any, target any) error {
	resp, err := c.Post(ctx, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return DecodeJSONResponse(resp, target)
}

// PutJSON performs a PUT request with JSON body and decodes the JSON response
func (c *Client) PutJSON(ctx context.Context, path string, body any, target any) error {
	resp, err := c.Put(ctx, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return DecodeJSONResponse(resp, target)
}

// PatchJSON performs a PATCH request with JSON body and decodes the JSON response
func (c *Client) PatchJSON(ctx context.Context, path string, body any, target any) error {
	resp, err := c.Patch(ctx, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return DecodeJSONResponse(resp, target)
}

// DeleteJSON performs a DELETE request and decodes the JSON response
func (c *Client) DeleteJSON(ctx context.Context, path string, target any) error {
	resp, err := c.Delete(ctx, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return DecodeJSONResponse(resp, target)
}

// DecodeJSONResponse decodes a JSON response from an HTTP response
func DecodeJSONResponse(resp *http.Response, target any) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http error %d: %s", resp.StatusCode, string(body))
	}

	if target == nil {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}

	return nil
}
