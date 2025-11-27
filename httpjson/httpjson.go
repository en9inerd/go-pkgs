// Package httpjson provides common helpers for JSON-based HTTP services
package httpjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// JSON is a convenience alias for a generic JSON object
type JSON map[string]any

// WriteJSON encodes and writes JSON to the response
func WriteJSON(w http.ResponseWriter, data any) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// WriteJSONBytes writes pre-encoded JSON bytes to the response
func WriteJSONBytes(w http.ResponseWriter, r *http.Request, data []byte) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("failed to send response to %s: %w", r.RemoteAddr, err)
	}
	return nil
}

// WriteJSONAllowHTML encodes and writes JSON with HTML characters unescaped
func WriteJSONAllowHTML(w http.ResponseWriter, r *http.Request, v any) error {
	encode := func(v any) ([]byte, error) {
		buf := &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(v); err != nil {
			return nil, fmt.Errorf("json encoding failed: %w", err)
		}
		return buf.Bytes(), nil
	}

	data, err := encode(v)
	if err != nil {
		return err
	}
	return WriteJSONBytes(w, r, data)
}

// writeJSONWithStatus encodes JSON and writes it with the given HTTP status
func writeJSONWithStatus(w http.ResponseWriter, data any, code int) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write(buf.Bytes())
}

// ParseDateRange extracts "from" and "to" query parameters and parses them as time.Time
func ParseDateRange(r *http.Request) (from, to time.Time, err error) {
	parseTimestamp := func(ts string) (time.Time, error) {
		formats := []string{
			"2006-01-02T15:04:05.000000000",
			"2006-01-02T15:04:05",
			"2006-01-02T15:04",
			"20060102",
			time.RFC3339,
			time.RFC3339Nano,
		}
		for _, f := range formats {
			if t, e := time.Parse(f, ts); e == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("can't parse date %q", ts)
	}

	if from, err = parseTimestamp(r.URL.Query().Get("from")); err != nil {
		return from, to, fmt.Errorf("invalid 'from' time: %w", err)
	}

	if to, err = parseTimestamp(r.URL.Query().Get("to")); err != nil {
		return from, to, fmt.Errorf("invalid 'to' time: %w", err)
	}
	return from, to, nil
}

// DecodeJSON decodes JSON from request body into the given struct
func DecodeJSON[T any](r *http.Request, target *T) error {
	if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}

// EncodeJSON encodes data as JSON and writes it with status code
func EncodeJSON[T any](w http.ResponseWriter, status int, v T) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}
