package httpclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNew_Defaults(t *testing.T) {
	c := New()
	if c.httpClient.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", c.httpClient.Timeout)
	}
	if c.headers == nil {
		t.Error("headers map is nil")
	}
}

func TestNewWithConfig(t *testing.T) {
	c := NewWithConfig(Config{
		Timeout: 5 * time.Second,
		BaseURL: "https://example.com",
		Headers: map[string]string{"X-Custom": "val"},
	})
	if c.httpClient.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", c.httpClient.Timeout)
	}
	if c.baseURL != "https://example.com" {
		t.Errorf("baseURL = %q", c.baseURL)
	}
	if c.headers["X-Custom"] != "val" {
		t.Error("custom header not set")
	}
}

func TestNewWithConfig_ZeroTimeout(t *testing.T) {
	c := NewWithConfig(Config{})
	if c.httpClient.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s default", c.httpClient.Timeout)
	}
}

func TestBuildURL(t *testing.T) {
	tests := []struct {
		base, path, want string
	}{
		{"", "/foo", "/foo"},
		{"https://example.com", "", "https://example.com"},
		{"https://example.com", "/api", "https://example.com/api"},
		{"https://example.com/", "/api", "https://example.com/api"},
		{"https://example.com", "api", "https://example.com/api"},
		{"https://example.com/", "api", "https://example.com/api"},
	}
	for _, tt := range tests {
		c := &Client{baseURL: tt.base}
		got := c.buildURL(tt.path)
		if got != tt.want {
			t.Errorf("buildURL(%q, %q) = %q, want %q", tt.base, tt.path, got, tt.want)
		}
	}
}

func TestGetJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"hello": "world"})
	}))
	defer srv.Close()

	c := New().WithBaseURL(srv.URL)
	var result map[string]string
	if err := c.GetJSON(context.Background(), "/test", &result); err != nil {
		t.Fatal(err)
	}
	if result["hello"] != "world" {
		t.Errorf("result = %v", result)
	}
}

func TestPostJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		json.NewEncoder(w).Encode(map[string]string{"echo": body["msg"]})
	}))
	defer srv.Close()

	c := New().WithBaseURL(srv.URL)
	var result map[string]string
	err := c.PostJSON(context.Background(), "/submit", map[string]string{"msg": "hi"}, &result)
	if err != nil {
		t.Fatal(err)
	}
	if result["echo"] != "hi" {
		t.Errorf("result = %v", result)
	}
}

func TestGetJSON_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	c := New().WithBaseURL(srv.URL)
	var result map[string]string
	err := c.GetJSON(context.Background(), "/missing", &result)
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestDecodeJSONResponse_NilTarget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New().WithBaseURL(srv.URL)
	resp, err := c.Get(context.Background(), "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if err := DecodeJSONResponse(resp, nil); err != nil {
		t.Errorf("unexpected error for nil target: %v", err)
	}
}

func TestWithHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-One") != "1" || r.Header.Get("X-Two") != "2" {
			t.Errorf("headers: X-One=%q, X-Two=%q", r.Header.Get("X-One"), r.Header.Get("X-Two"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New().WithHeaders(map[string]string{"X-One": "1", "X-Two": "2"})
	resp, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("Method = %q, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New()
	resp, err := c.Delete(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
}

func TestPut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("Method = %q, want PUT", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		w.Write(body)
	}))
	defer srv.Close()

	c := New()
	resp, err := c.Put(context.Background(), srv.URL, map[string]string{"k": "v"})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestPatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("Method = %q, want PATCH", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New()
	resp, err := c.Patch(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestPost_NilBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "" {
			t.Error("Content-Type should be empty for nil body")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New()
	resp, err := c.Post(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}
